package services

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type BackupService struct {
	hostRepo    *repositories.HostRepository
	instRepo    *repositories.InstanceRepository
	policyRepo  *repositories.BackupRepository
	agentClient *AgentClient
	encKey      string
}

func NewBackupService(hostRepo *repositories.HostRepository, instRepo *repositories.InstanceRepository, policyRepo *repositories.BackupRepository, agentClient *AgentClient, encKey string) *BackupService {
	return &BackupService{
		hostRepo:    hostRepo,
		instRepo:    instRepo,
		policyRepo:  policyRepo,
		agentClient: agentClient,
		encKey:      encKey,
	}
}

type CreateBackupPolicyRequest struct {
	InstanceID    string `json:"instance_id" binding:"required"`
	BackupType    string `json:"backup_type" binding:"required"`
	Schedule      string `json:"schedule" binding:"required"`
	RetentionDays int    `json:"retention_days"`
	StorageType   string `json:"storage_type"`
	StoragePath   string `json:"storage_path"`
	Enabled       bool   `json:"enabled"`
}

type ExecuteBackupRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	BackupType string `json:"backup_type" binding:"required"`
}

type BackupTaskResult struct {
	ID          string    `json:"id"`
	InstanceID  string    `json:"instance_id"`
	BackupType  string    `json:"backup_type"`
	TaskID      string    `json:"task_id"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	FilePath    string    `json:"file_path"`
	FileSize    int64     `json:"file_size"`
	Size        string    `json:"size"`
	Checksum    string    `json:"checksum"`
	CreatedAt   time.Time `json:"created_at"`
}

type DiscoveredBackup struct {
	FileName        string    `json:"file_name"`
	FilePath        string    `json:"file_path"`
	SizeBytes       int64     `json:"size_bytes"`
	BackupType      string    `json:"backup_type"`
	DetectedAt      time.Time `json:"detected_at"`
	MTime           time.Time `json:"mtime"`
	AlreadyManaged  bool      `json:"already_managed"`
	ManagedBackupID string    `json:"managed_backup_id,omitempty"`
}

type BackupScanResult struct {
	Backups   []DiscoveredBackup `json:"backups"`
	ScannedAt time.Time          `json:"scanned_at"`
}

// CreatePolicy P0-4: 真实写入 backup_policies 表, 不再是占位字符串.
func (s *BackupService) CreatePolicy(ctx context.Context, req CreateBackupPolicyRequest) (string, error) {
	policy := &models.BackupPolicy{
		InstanceID:    req.InstanceID,
		BackupType:    req.BackupType,
		Schedule:      req.Schedule,
		RetentionDays: req.RetentionDays,
		StorageType:   req.StorageType,
		StoragePath:   req.StoragePath,
		Enabled:       req.Enabled,
		CreatedAt:     time.Now(),
	}
	if policy.StorageType == "" {
		policy.StorageType = "local"
	}
	if policy.RetentionDays == 0 {
		policy.RetentionDays = 7
	}
	if err := s.policyRepo.CreatePolicy(ctx, policy); err != nil {
		return "", err
	}
	return policy.ID, nil
}

func (s *BackupService) ListPolicies(ctx context.Context, instanceID string) ([]models.BackupPolicy, error) {
	return s.policyRepo.ListPolicies(ctx, instanceID, 100, 0)
}

func (s *BackupService) ExecuteBackup(ctx context.Context, req ExecuteBackupRequest) (*BackupTaskResult, error) {
	inst, err := s.instRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("instance not found: %w", err)
	}

	var agentHost string
	agentPort := 9090
	if inst.HostID != nil && *inst.HostID != "" {
		host, err := s.hostRepo.GetByID(ctx, *inst.HostID)
		if err == nil {
			agentHost = host.Address
		}
	}
	if agentHost == "" {
		return nil, fmt.Errorf("cannot determine agent host")
	}
	conn, err := s.instRepo.GetConnection(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("instance connection not found: %w", err)
	}
	password, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt instance password: %w", err)
	}

	config := map[string]interface{}{
		"backup_type": req.BackupType,
		"target_dir":  "/backup/mysql",
		"mysql_host":  conn.Host,
		"mysql_port":  conn.Port,
		"mysql_user":  conn.Username,
		"mysql_pass":  password,
	}
	taskID := fmt.Sprintf("backup-%d", time.Now().Unix())
	now := time.Now()
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/backup", map[string]interface{}{
		"task_id":     taskID,
		"instance_id": req.InstanceID,
		"config":      config,
	})
	out := &BackupTaskResult{
		InstanceID: req.InstanceID,
		BackupType: req.BackupType,
		TaskID:     taskID,
		StartedAt:  now,
	}
	if err != nil {
		out.Status = "failed"
		// 真实落库失败记录, 而不是悄无声息.
		_ = s.policyRepo.CreateRecord(ctx, &models.BackupRecord{
			InstanceID: req.InstanceID,
			BackupType: req.BackupType,
			StartedAt:  now,
			Status:     "failed",
			CreatedAt:  now,
		})
		return out, nil
	}
	out.Status = result.Status
	out.CompletedAt = time.Now()
	out.FilePath = result.Message

	_ = s.policyRepo.CreateRecord(ctx, &models.BackupRecord{
		InstanceID:  req.InstanceID,
		BackupType:  req.BackupType,
		StartedAt:   now,
		CompletedAt: out.CompletedAt,
		Status:      out.Status,
		FilePath:    out.FilePath,
		CreatedAt:   now,
	})
	return out, nil
}

// ListBackups P0-4: 真实从 backup_records 读, 不再返回空.
func (s *BackupService) ListBackups(ctx context.Context, instanceID string) ([]BackupTaskResult, error) {
	records, err := s.policyRepo.ListRecords(ctx, instanceID, 100, 0)
	if err != nil {
		return nil, err
	}
	out := make([]BackupTaskResult, 0, len(records))
	for _, r := range records {
		out = append(out, BackupTaskResult{
			ID:          r.ID,
			InstanceID:  r.InstanceID,
			BackupType:  r.BackupType,
			TaskID:      r.ID,
			Status:      r.Status,
			StartedAt:   r.StartedAt,
			CompletedAt: r.CompletedAt,
			FilePath:    r.FilePath,
			FileSize:    r.FileSize,
			Size:        formatBytes(r.FileSize),
			Checksum:    r.Checksum,
			CreatedAt:   r.CreatedAt,
		})
	}
	return out, nil
}

func (s *BackupService) ScanBackups(ctx context.Context, instanceID string) (*BackupScanResult, error) {
	if instanceID == "" {
		return nil, fmt.Errorf("instance_id is required")
	}
	inst, err := s.instRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("instance not found: %w", err)
	}
	agentHost, agentPort, err := s.resolveAgentEndpoint(ctx, inst)
	if err != nil {
		return nil, err
	}

	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/backup-scan", map[string]interface{}{
		"task_id":     fmt.Sprintf("backup-scan-%d", time.Now().UnixNano()),
		"instance_id": instanceID,
		"config": map[string]interface{}{
			"paths": []string{"/backup/mysql", "/backup", "/data/backup", "/var/lib/mysql-backup"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("agent backup scan failed: %w", err)
	}

	discovered := decodeDiscoveredBackups(result.Data)
	records, _ := s.policyRepo.ListRecords(ctx, instanceID, 1000, 0)
	managed := make(map[string]string, len(records))
	for _, record := range records {
		if record.FilePath != "" {
			managed[record.FilePath] = record.ID
			managed[filepath.Base(record.FilePath)] = record.ID
		}
	}
	for i := range discovered {
		if id := managed[discovered[i].FilePath]; id != "" {
			discovered[i].AlreadyManaged = true
			discovered[i].ManagedBackupID = id
			continue
		}
		if id := managed[discovered[i].FileName]; id != "" {
			discovered[i].AlreadyManaged = true
			discovered[i].ManagedBackupID = id
		}
	}
	return &BackupScanResult{Backups: discovered, ScannedAt: time.Now()}, nil
}

func (s *BackupService) resolveAgentEndpoint(ctx context.Context, inst *models.Instance) (string, int, error) {
	if inst.HostID != nil && *inst.HostID != "" {
		host, err := s.hostRepo.GetByID(ctx, *inst.HostID)
		if err == nil && host.Address != "" {
			port := host.AgentPort
			if port == 0 {
				port = 9090
			}
			return host.Address, port, nil
		}
	}
	conn, err := s.instRepo.GetConnection(ctx, inst.ID)
	if err != nil {
		return "", 0, fmt.Errorf("instance connection not found: %w", err)
	}
	if conn.Host == "" {
		return "", 0, fmt.Errorf("cannot determine agent host")
	}
	return conn.Host, 9090, nil
}

func decodeDiscoveredBackups(data map[string]interface{}) []DiscoveredBackup {
	raw, _ := data["backups"].([]interface{})
	out := make([]DiscoveredBackup, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, DiscoveredBackup{
			FileName:   stringValue(m["file_name"]),
			FilePath:   stringValue(m["file_path"]),
			SizeBytes:  int64Value(m["size_bytes"]),
			BackupType: stringValue(m["backup_type"]),
			DetectedAt: time.Now(),
			MTime:      time.Now(),
		})
	}
	return out
}

func stringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func int64Value(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "-"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	text := fmt.Sprintf("%.1f %s", size, units[unit])
	return strings.Replace(text, ".0 ", " ", 1)
}
