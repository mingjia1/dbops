package services

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type HostService struct {
	repo        *repositories.HostRepository
	encKey      string

	taskMu      sync.RWMutex
	testResults map[string]*HostTestResult
}

func NewHostService(repo *repositories.HostRepository, encKey string) *HostService {
	return &HostService{
		repo:        repo,
		encKey:      encKey,
		testResults: make(map[string]*HostTestResult),
	}
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
