package services

import (
	"context"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type BackupService struct {
	hostRepo    *repositories.HostRepository
	instRepo    *repositories.InstanceRepository
	policyRepo  *repositories.BackupRepository
	agentClient *AgentClient
}

func NewBackupService(hostRepo *repositories.HostRepository, instRepo *repositories.InstanceRepository, policyRepo *repositories.BackupRepository, agentClient *AgentClient) *BackupService {
	return &BackupService{
		hostRepo:    hostRepo,
		instRepo:    instRepo,
		policyRepo:  policyRepo,
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
	taskID := fmt.Sprintf("backup-%d", time.Now().Unix())
	now := time.Now()
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/backup", map[string]interface{}{
		"task_id":     taskID,
		"instance_id": req.InstanceID,
		"config":      config,
	})
	out := &BackupTaskResult{
		TaskID:    taskID,
		StartedAt: now,
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
		InstanceID: req.InstanceID,
		BackupType: req.BackupType,
		StartedAt:  now,
		CompletedAt: out.CompletedAt,
		Status:     out.Status,
		FilePath:   out.FilePath,
		CreatedAt:  now,
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
			TaskID:      r.ID,
			Status:      r.Status,
			StartedAt:   r.StartedAt,
			CompletedAt: r.CompletedAt,
			FilePath:    r.FilePath,
			FileSize:    r.FileSize,
			Checksum:    r.Checksum,
		})
	}
	return out, nil
}
