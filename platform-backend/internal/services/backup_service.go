package services

import (
	"context"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type BackupService struct {
	hostRepo    *repositories.HostRepository
	instRepo    *repositories.InstanceRepository
	agentClient *AgentClient
}

func NewBackupService(hostRepo *repositories.HostRepository, instRepo *repositories.InstanceRepository, agentClient *AgentClient) *BackupService {
	return &BackupService{
		hostRepo:    hostRepo,
		instRepo:    instRepo,
		agentClient: agentClient,
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
	TaskID      string    `json:"task_id"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	FilePath    string    `json:"file_path"`
	FileSize    int64     `json:"file_size"`
	Checksum    string    `json:"checksum"`
}

func (s *BackupService) CreatePolicy(ctx context.Context, req CreateBackupPolicyRequest) (string, error) {
	return fmt.Sprintf("policy-%d", time.Now().Unix()), nil
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

	config := map[string]interface{}{
		"backup_type": req.BackupType,
		"target_dir":  "/backup/mysql",
	}

	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/backup", map[string]interface{}{
		"task_id":     fmt.Sprintf("backup-%d", time.Now().Unix()),
		"instance_id": req.InstanceID,
		"config":      config,
	})
	if err != nil {
		return &BackupTaskResult{
			TaskID:    fmt.Sprintf("backup-%d", time.Now().Unix()),
			Status:    "failed",
			StartedAt: time.Now(),
		}, nil
	}

	return &BackupTaskResult{
		TaskID:      result.TaskID,
		Status:      result.Status,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		FilePath:    result.Message,
	}, nil
}

func (s *BackupService) ListBackups(ctx context.Context, instanceID string) ([]BackupTaskResult, error) {
	return []BackupTaskResult{}, nil
}
