package services

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type HostService struct {
	repo         *repositories.HostRepository
	instanceRepo *repositories.InstanceRepository
	encKey       string

	taskMu      sync.RWMutex
	testResults map[string]*HostTestResult

	scanMu      sync.RWMutex
	scanResults map[string]*HostScanResult
}

func NewHostService(repo *repositories.HostRepository, encKey string) *HostService {
	return &HostService{
		repo:         repo,
		encKey:       encKey,
		testResults:  make(map[string]*HostTestResult),
		scanResults:  make(map[string]*HostScanResult),
	}
}

func (s *HostService) SetInstanceRepo(repo *repositories.InstanceRepository) {
	s.instanceRepo = repo
}

type CreateHostRequest struct {
	Name          string `json:"name" binding:"required"`
	Address       string `json:"address" binding:"required"`
	SSHPort       int    `json:"ssh_port"`
	SSHUser       string `json:"ssh_user" binding:"required"`
	SSHAuthMethod string `json:"ssh_auth_method"`
	SSHCredential string `json:"ssh_credential"`
	AgentPort     int    `json:"agent_port"`
	OSType        string `json:"os_type"`
	Description   string `json:"description"`
	Tags          string `json:"tags"`
}

type UpdateHostRequest struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	SSHPort       int    `json:"ssh_port"`
	SSHUser       string `json:"ssh_user"`
	SSHAuthMethod string `json:"ssh_auth_method"`
	SSHCredential string `json:"ssh_credential"`
	AgentPort     int    `json:"agent_port"`
	OSType        string `json:"os_type"`
	Description   string `json:"description"`
	Tags          string `json:"tags"`
}

type HostTestResult struct {
	TaskID    string    `json:"task_id"`
	HostID    string    `json:"host_id"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	LatencyMs int64     `json:"latency_ms"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

func (s *HostService) Create(ctx context.Context, req CreateHostRequest) (*models.Host, error) {
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.SSHAuthMethod == "" {
		req.SSHAuthMethod = "password"
	}
	if req.OSType == "" {
		req.OSType = "linux"
	}

	encrypted, err := utils.Encrypt(req.SSHCredential, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt credential: %w", err)
	}

	agentPort := req.AgentPort
	if agentPort == 0 {
		agentPort = 9090
	}

	host := &models.Host{
		Name:          req.Name,
		Address:       req.Address,
		SSHPort:       req.SSHPort,
		SSHUser:       req.SSHUser,
		SSHAuthMethod: req.SSHAuthMethod,
		SSHCredential: encrypted,
		AgentPort:     agentPort,
		OSType:        req.OSType,
		Description:   req.Description,
		Tags:          req.Tags,
		Status:        "unknown",
	}

	if err := s.repo.Create(ctx, host); err != nil {
		return nil, err
	}
	return host, nil
}

func (s *HostService) GetByID(ctx context.Context, id string) (*models.Host, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *HostService) List(ctx context.Context, limit, offset int) ([]models.Host, error) {
	return s.repo.List(ctx, limit, offset)
}

func (s *HostService) Update(ctx context.Context, id string, req UpdateHostRequest) (*models.Host, error) {
	host, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		host.Name = req.Name
	}
	if req.Address != "" {
		host.Address = req.Address
	}
	if req.SSHPort != 0 {
		host.SSHPort = req.SSHPort
	}
	if req.SSHUser != "" {
		host.SSHUser = req.SSHUser
	}
	if req.SSHAuthMethod != "" {
		host.SSHAuthMethod = req.SSHAuthMethod
	}
	if req.SSHCredential != "" {
		encrypted, err := utils.Encrypt(req.SSHCredential, s.encKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt credential: %w", err)
		}
		host.SSHCredential = encrypted
	}
	if req.AgentPort != 0 {
		host.AgentPort = req.AgentPort
	}
	if req.OSType != "" {
		host.OSType = req.OSType
	}
	if req.Description != "" {
		host.Description = req.Description
	}
	if req.Tags != "" {
		host.Tags = req.Tags
	}

	if err := s.repo.Update(ctx, host); err != nil {
		return nil, err
	}
	return host, nil
}

func (s *HostService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *HostService) StartTestConnection(ctx context.Context, hostID string) (*HostTestResult, error) {
	host, err := s.repo.GetByID(ctx, hostID)
	if err != nil {
		return nil, err
	}

	taskID := uuid.New().String()
	now := time.Now()
	initial := &HostTestResult{
		TaskID:    taskID,
		HostID:    hostID,
		Status:    "pending",
		StartedAt: now,
	}
	s.taskMu.Lock()
	s.testResults[taskID] = initial
	s.taskMu.Unlock()

	go s.runConnectionTest(taskID, host)

	return initial, nil
}

func (s *HostService) runConnectionTest(taskID string, host *models.Host) {
	startTime := time.Now()
	address := net.JoinHostPort(host.Address, fmt.Sprintf("%d", host.SSHPort))
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	latency := time.Since(startTime).Milliseconds()

	result := &HostTestResult{
		TaskID:    taskID,
		HostID:    host.ID,
		StartedAt: startTime,
		EndedAt:   time.Now(),
		LatencyMs: latency,
	}

	if err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("TCP连接失败: %v", err)
	} else {
		conn.Close()
		result.Status = "success"
		result.Message = fmt.Sprintf("TCP端口可达 (延迟 %dms)", latency)
	}

	s.taskMu.Lock()
	s.testResults[taskID] = result
	s.taskMu.Unlock()

	_ = s.repo.UpdateStatus(context.Background(), host.ID, result.Status)
}

func (s *HostService) GetTestResult(taskID string) (*HostTestResult, error) {
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()

	result, ok := s.testResults[taskID]
	if !ok {
		return nil, fmt.Errorf("test task not found")
	}
	copy := *result
	return &copy, nil
}

// ============== 实例扫描 ==============

type ScannedInstance struct {
	Port            int    `json:"port"`
	Version         string `json:"version,omitempty"`
	VersionFull     string `json:"version_full,omitempty"`
	Flavor          string `json:"flavor,omitempty"`
	Role            string `json:"role,omitempty"`
	Datadir         string `json:"datadir,omitempty"`
	Socket          string `json:"socket,omitempty"`
	ConfigPath      string `json:"config_path,omitempty"`
	Running         bool   `json:"running"`
	PID             int    `json:"pid,omitempty"`
	MemoryMB        int    `json:"memory_mb,omitempty"`
	DataSizeMB      int    `json:"data_size_mb,omitempty"`
	RecommendedName string `json:"recommended_name,omitempty"`
	AlreadyManaged  bool   `json:"already_managed"`
	ManagedID       string `json:"managed_instance_id,omitempty"`
}

type HostScanResult struct {
	TaskID    string           `json:"task_id"`
	HostID    string           `json:"host_id"`
	Status    string           `json:"status"`
	Message   string           `json:"message"`
	Instances []ScannedInstance `json:"instances"`
	ScannedAt *time.Time       `json:"scanned_at,omitempty"`
	Error     string           `json:"error,omitempty"`
}

type ScanInstancesRequest struct {
	Ports      []int  `json:"ports"`
	PortRange  string `json:"port_range"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	ProbeMySQL bool   `json:"probe_mysql"`
}

var defaultScanPorts = []int{3306, 33060, 33061, 33306, 3307, 3308, 3309, 3310, 13306, 23306}

func (s *HostService) StartScanInstances(ctx context.Context, hostID string, req ScanInstancesRequest) (*HostScanResult, error) {
	host, err := s.repo.GetByID(ctx, hostID)
	if err != nil {
		return nil, err
	}

	ports := normalizeScanPorts(req.Ports, req.PortRange)
	if len(ports) == 0 {
		ports = defaultScanPorts
	}
	sort.Ints(ports)
	ports = dedupInts(ports)
	if len(ports) > 1024 {
		ports = ports[:1024]
	}

	probeMySQL := req.ProbeMySQL
	username := req.Username
	password := req.Password
	if username == "" {
		username = "root"
	}

	taskID := uuid.New().String()
	initial := &HostScanResult{
		TaskID:  taskID,
		HostID:  hostID,
		Status:  "pending",
		Message: fmt.Sprintf("已加入扫描队列, 探测 %d 个端口", len(ports)),
	}
	s.scanMu.Lock()
	s.scanResults[taskID] = initial
	s.scanMu.Unlock()

	go s.runScan(taskID, host, ports, probeMySQL, username, password)

	return initial, nil
}

func (s *HostService) GetScanResult(taskID string) (*HostScanResult, error) {
	s.scanMu.RLock()
	defer s.scanMu.RUnlock()

	result, ok := s.scanResults[taskID]
	if !ok {
		return nil, fmt.Errorf("scan task not found")
	}
	copy := *result
	return &copy, nil
}

func (s *HostService) runScan(taskID string, host *models.Host, ports []int, probeMySQL bool, username, password string) {
	result := &HostScanResult{
		TaskID:  taskID,
		HostID:  host.ID,
		Status:  "running",
		Message: fmt.Sprintf("正在扫描 %d 个端口", len(ports)),
	}
	s.scanMu.Lock()
	s.scanResults[taskID] = result
	s.scanMu.Unlock()

	managedPorts := s.listManagedPorts(host.ID)

	scanned := make([]ScannedInstance, 0, len(ports))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)

	for _, p := range ports {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			si := probePort(host.Address, port, probeMySQL, username, password)
			if si == nil {
				return
			}
			si.RecommendedName = fmt.Sprintf("%s-%d", sanitizeName(host.Name), port)
			if mid, ok := managedPorts[port]; ok {
				si.AlreadyManaged = true
				si.ManagedID = mid
			}
			mu.Lock()
			scanned = append(scanned, *si)
			mu.Unlock()
		}(p)
	}
	wg.Wait()

	sort.Slice(scanned, func(i, j int) bool { return scanned[i].Port < scanned[j].Port })

	now := time.Now()
	result.Instances = scanned
	result.ScannedAt = &now
	result.Status = "success"
	if len(scanned) == 0 {
		result.Message = fmt.Sprintf("扫描完成, %d 个端口中未发现 MySQL 实例", len(ports))
	} else {
		newCount := 0
		for i := range scanned {
			if !scanned[i].AlreadyManaged {
				newCount++
			}
		}
		result.Message = fmt.Sprintf("扫描完成, 发现 %d 个实例 (%d 个新发现, %d 个已纳管)", len(scanned), newCount, len(scanned)-newCount)
	}

	s.scanMu.Lock()
	s.scanResults[taskID] = result
	s.scanMu.Unlock()
}

func (s *HostService) listManagedPorts(hostID string) map[int]string {
	out := make(map[int]string)
	if s.instanceRepo == nil {
		return out
	}
	insts, err := s.instanceRepo.ListByHostID(context.Background(), hostID, 1000, 0)
	if err != nil {
		return out
	}
	for _, inst := range insts {
		conn, err := s.instanceRepo.GetConnection(context.Background(), inst.ID)
		if err != nil || conn == nil {
			continue
		}
		if conn.Port > 0 {
			out[conn.Port] = inst.ID
		}
	}
	return out
}

func normalizeScanPorts(ports []int, portRange string) []int {
	out := make([]int, 0)
	if len(ports) > 0 {
		for _, p := range ports {
			if p > 0 && p <= 65535 {
				out = append(out, p)
			}
		}
	}
	if portRange != "" {
		out = append(out, parsePortRange(portRange)...)
	}
	return out
}

func parsePortRange(s string) []int {
	out := make([]int, 0)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			start, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil || start <= 0 || end <= 0 || end < start {
				continue
			}
			if end > 65535 {
				end = 65535
			}
			if end-start > 1024 {
				end = start + 1024
			}
			for p := start; p <= end; p++ {
				out = append(out, p)
			}
		} else {
			p, err := strconv.Atoi(part)
			if err == nil && p > 0 && p <= 65535 {
				out = append(out, p)
			}
		}
	}
	return out
}

func dedupInts(a []int) []int {
	seen := make(map[int]struct{}, len(a))
	out := make([]int, 0, len(a))
	for _, v := range a {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func sanitizeName(n string) string {
	n = strings.TrimSpace(n)
	if n == "" {
		return "host"
	}
	n = strings.ReplaceAll(n, " ", "-")
	return n
}

func probePort(host string, port int, probeMySQL bool, username, password string) *ScannedInstance {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 1500*time.Millisecond)
	if err != nil {
		return nil
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	si := &ScannedInstance{
		Port:    port,
		Running: true,
	}

	if !probeMySQL {
		si.Flavor = "tcp-only"
		return si
	}

	// Send a simple MySQL handshake probe: read greeting then close.
	// MySQL server sends greeting packet on connect; first byte is length, second is sequence (0), then protocol version.
	reader := bufio.NewReader(conn)
	hdr, err := reader.Peek(5)
	if err != nil || len(hdr) < 5 {
		si.Flavor = "unknown"
		return si
	}
	plen := int(uint32(hdr[0]) | uint32(hdr[1])<<8 | uint32(hdr[2])<<16)
	if hdr[4] != 0 {
		si.Flavor = "non-mysql"
		return si
	}
	if plen <= 0 || plen > 4096 {
		si.Flavor = "non-mysql"
		return si
	}
	greeting := make([]byte, 4+plen)
	if _, err := reader.Read(greeting); err != nil {
		si.Flavor = "non-mysql"
		return si
	}
	if len(greeting) < 6 || greeting[4] != 0x0a {
		si.Flavor = "non-mysql"
		return si
	}

	// greeting: 4-byte header + 0x0a (protocol 10) + server_version (null-terminated) + ...
	rest := greeting[5:]
	if idx := indexByte(rest, 0); idx > 0 {
		si.VersionFull = string(rest[:idx])
		si.Version = normalizeVersionString(si.VersionFull)
	}
	if i := strings.Index(strings.ToLower(si.VersionFull), "mariadb"); i >= 0 {
		si.Flavor = "mariadb"
	} else if i := strings.Index(strings.ToLower(si.VersionFull), "tidb"); i >= 0 {
		si.Flavor = "tidb"
	} else {
		si.Flavor = "mysql"
	}

	return si
}

func indexByte(b []byte, target byte) int {
	for i, c := range b {
		if c == target {
			return i
		}
	}
	return -1
}

func normalizeVersionString(s string) string {
	if s == "" {
		return s
	}
	if idx := strings.Index(s, "-"); idx > 0 {
		return s[:idx]
	}
	return s
}

type RegisterScannedInstanceRequest struct {
	Port      int    `json:"port" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Username  string `json:"username"`
	Password  string `json:"password" binding:"required"`
	ClusterID string `json:"cluster_id"`
}

func (s *HostService) RegisterScannedInstance(ctx context.Context, hostID string, req RegisterScannedInstanceRequest) (string, error) {
	if s.instanceRepo == nil {
		return "", fmt.Errorf("instance repository not initialized")
	}
	host, err := s.repo.GetByID(ctx, hostID)
	if err != nil {
		return "", err
	}
	hid := host.ID
	now := time.Now()
	inst := &models.Instance{
		Name:      req.Name,
		ClusterID: req.ClusterID,
		HostID:    &hid,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.instanceRepo.Create(ctx, inst); err != nil {
		return "", fmt.Errorf("failed to create instance: %w", err)
	}
	conn := &models.InstanceConnection{
		InstanceID: inst.ID,
		Host:       host.Address,
		Port:       req.Port,
		Username:   req.Username,
	}
	// P1-4: 之前 PasswordEncrypted: req.Password 直接存明文, 与 InstanceService.Create
	// 不一致 — 任何后续 health_check / failover 拿 conn 当密码, MySQL 收到 AES-GCM 密文必败.
	// 修: 落库前 utils.Encrypt.
	if req.Password != "" {
		encPwd, err := utils.Encrypt(req.Password, s.encKey)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt password: %w", err)
		}
		conn.PasswordEncrypted = encPwd
	}
	if err := s.instanceRepo.CreateConnection(ctx, conn); err != nil {
		return "", fmt.Errorf("failed to create connection: %w", err)
	}
	return inst.ID, nil
}
