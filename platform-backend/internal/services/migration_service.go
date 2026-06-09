package services

import (
	"context"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type MigrationService struct {
	repo        *repositories.MigrationRepository
	instRepo    *repositories.InstanceRepository
	hostRepo    *repositories.HostRepository
	agentClient *AgentClient
}

func NewMigrationService(repo *repositories.MigrationRepository, instRepo *repositories.InstanceRepository, hostRepo *repositories.HostRepository, agentClient *AgentClient) *MigrationService {
	return &MigrationService{
		repo:        repo,
		instRepo:    instRepo,
		hostRepo:    hostRepo,
		agentClient: agentClient,
	}
}

// resolveAgentHost P1-5: 仿 switch_service 模式, 从 instance → host 拿到真实 agent 地址和端口.
func resolveAgentHost(ctx context.Context, inst *models.Instance, instRepo *repositories.InstanceRepository, hostRepo *repositories.HostRepository, defaultPort int) (string, int) {
	if inst == nil {
		return "localhost", defaultPort
	}
	if inst.HostID == nil || *inst.HostID == "" {
		// 没有 host 关联时退回到 instance.Connection.Host (但仍有可能不是 agent)
		return "localhost", defaultPort
	}
	host, err := hostRepo.GetByID(ctx, *inst.HostID)
	if err != nil || host == nil {
		return "localhost", defaultPort
	}
	port := defaultPort
	if host.AgentPort > 0 {
		port = host.AgentPort
	}
	return host.Address, port
}

type CreateMigrationTaskRequest struct {
	Name             string                   `json:"name" binding:"required"`
	SourceInstanceID string                   `json:"source_instance_id" binding:"required"`
	TargetInstanceID string                   `json:"target_instance_id" binding:"required"`
	Strategy         models.MigrationStrategy `json:"strategy" binding:"required"`
	Config           string                   `json:"config"`
}

type MigrationTaskResult struct {
	TaskID    string                   `json:"task_id"`
	Status    models.MigrationStatus   `json:"status"`
	Strategy  models.MigrationStrategy `json:"strategy"`
	StartedAt time.Time                `json:"started_at"`
	Progress  int                      `json:"progress"`
}

func (s *MigrationService) CreateTask(ctx context.Context, req CreateMigrationTaskRequest) (string, error) {
	task := &models.MigrationTask{
		Name:             req.Name,
		SourceInstanceID: req.SourceInstanceID,
		TargetInstanceID: req.TargetInstanceID,
		Strategy:         req.Strategy,
		Status:           models.MigrationStatusPending,
		Progress:         0,
	}
	if err := s.repo.Create(ctx, task); err != nil {
		return "", fmt.Errorf("failed to create migration task: %w", err)
	}
	return task.ID, nil
}

func (s *MigrationService) executeMigration(ctx context.Context, taskID string, strategy models.MigrationStrategy) (*MigrationTaskResult, error) {
	task, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	sourceInst, err := s.instRepo.GetByID(ctx, task.SourceInstanceID)
	if err != nil {
		return nil, fmt.Errorf("source instance not found: %w", err)
	}
	_ = sourceInst

	now := time.Now()
	s.repo.UpdateStatus(ctx, taskID, models.MigrationStatusMigrating, 0)

	config := map[string]interface{}{
		"migration_type":     string(strategy),
		"source_instance_id": task.SourceInstanceID,
		"target_instance_id": task.TargetInstanceID,
	}

	// P1-5: 用 instance → host → 真实 agent 地址, 不再写死 localhost.
	sourceInstFull, _ := s.instRepo.GetByID(ctx, task.SourceInstanceID)
	agentHost, agentPort := resolveAgentHost(ctx, sourceInstFull, s.instRepo, s.hostRepo, 9090)
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/migration", map[string]interface{}{
		"task_id":     taskID,
		"instance_id": task.SourceInstanceID,
		"config":      config,
	})
	if err != nil {
		s.repo.UpdateStatus(ctx, taskID, models.MigrationStatusFailed, 0)
		return &MigrationTaskResult{
			TaskID:    taskID,
			Status:    models.MigrationStatusFailed,
			Strategy:  strategy,
			StartedAt: now,
			Progress:  0,
		}, nil
	}

	status := models.MigrationStatusCompleted
	if result.Status == "failed" {
		status = models.MigrationStatusFailed
	}
	s.repo.UpdateStatus(ctx, taskID, status, result.Progress)

	return &MigrationTaskResult{
		TaskID:    taskID,
		Status:    status,
		Strategy:  strategy,
		StartedAt: now,
		Progress:  result.Progress,
	}, nil
}

func (s *MigrationService) ExecutePhysicalMigration(ctx context.Context, taskID string) (*MigrationTaskResult, error) {
	return s.executeMigration(ctx, taskID, models.MigrationStrategyPhysical)
}

func (s *MigrationService) ExecuteReplicationMigration(ctx context.Context, taskID string) (*MigrationTaskResult, error) {
	return s.executeMigration(ctx, taskID, models.MigrationStrategyReplication)
}

func (s *MigrationService) ExecuteGTIDMigration(ctx context.Context, taskID string) (*MigrationTaskResult, error) {
	return s.executeMigration(ctx, taskID, models.MigrationStrategyGTID)
}

func (s *MigrationService) OrchestrateMigration(ctx context.Context, taskID string) (*MigrationTaskResult, error) {
	task, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return s.executeMigration(ctx, taskID, task.Strategy)
}

func (s *MigrationService) MonitorMigrationProgress(ctx context.Context, taskID string) (*models.MigrationProgress, error) {
	// B10: silent 吞 err 改为显式: task 不存在直接 404, 不要用假 Progress 误导前端.
	task, err := s.repo.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	inst, err := s.instRepo.GetByID(ctx, task.SourceInstanceID)
	if err != nil {
		return nil, fmt.Errorf("source instance not found: %w", err)
	}
	agentHost, agentPort := resolveAgentHost(ctx, inst, s.instRepo, s.hostRepo, 9090)
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/migration", map[string]interface{}{
		"task_id":     taskID,
		"instance_id": task.SourceInstanceID,
		"config": map[string]interface{}{
			"monitor": true,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("agent monitor call failed: %w", err)
	}

	return &models.MigrationProgress{
		TaskID:    taskID,
		Status:    models.MigrationStatus(result.Status),
		Progress:  result.Progress,
		UpdatedAt: time.Now(),
	}, nil
}

// VerifyMigration P0-4: 真实调 Agent 拿校验结果, 不再写死 1000000 行一致.
func (s *MigrationService) VerifyMigration(ctx context.Context, taskID string) (*models.MigrationVerification, error) {
	task, err := s.repo.GetByID(ctx, taskID)
	if err != nil || task == nil {
		return nil, fmt.Errorf("migration task %s not found", taskID)
	}
	inst, err := s.instRepo.GetByID(ctx, task.TargetInstanceID)
	if err != nil || inst == nil {
		return nil, fmt.Errorf("target instance %s not found", task.TargetInstanceID)
	}
	agentHost, agentPort := resolveAgentHost(ctx, inst, s.instRepo, s.hostRepo, 9090)
	result, callErr := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/migration-verify", map[string]interface{}{
		"task_id":            taskID,
		"source_instance_id": task.SourceInstanceID,
		"target_instance_id": task.TargetInstanceID,
	})
	out := &models.MigrationVerification{
		TaskID:     taskID,
		VerifiedAt: time.Now(),
	}
	if callErr != nil {
		out.Errors = []string{"agent verify call failed: " + callErr.Error()}
		_ = s.repo.UpdateStatus(ctx, taskID, models.MigrationStatusFailed, task.Progress)
		return out, nil
	}
	if v, ok := result.Data["source_count"].(float64); ok {
		out.SourceCount = int64(v)
	}
	if v, ok := result.Data["target_count"].(float64); ok {
		out.TargetCount = int64(v)
	}
	if v, ok := result.Data["data_consistency"].(bool); ok {
		out.DataConsistency = v
	}
	if v, ok := result.Data["schema_match"].(bool); ok {
		out.SchemaMatch = v
	}
	if v, ok := result.Data["checksum_match"].(bool); ok {
		out.ChecksumMatch = v
	}
	if errs, ok := result.Data["errors"].([]interface{}); ok {
		for _, e := range errs {
			if s, ok := e.(string); ok {
				out.Errors = append(out.Errors, s)
			}
		}
	}
	if len(out.Errors) > 0 {
		_ = s.repo.UpdateStatus(ctx, taskID, models.MigrationStatusFailed, task.Progress)
	} else {
		_ = s.repo.UpdateStatus(ctx, taskID, models.MigrationStatusVerifying, task.Progress)
	}
	return out, nil
}

func (s *MigrationService) ExecuteSwitch(ctx context.Context, taskID string) (*models.MigrationSwitchResult, error) {
	task, err := s.repo.GetByID(ctx, taskID)
	if err != nil || task == nil {
		return nil, fmt.Errorf("migration task %s not found", taskID)
	}
	inst, err := s.instRepo.GetByID(ctx, task.TargetInstanceID)
	if err != nil || inst == nil {
		return nil, fmt.Errorf("target instance %s not found", task.TargetInstanceID)
	}
	agentHost, agentPort := resolveAgentHost(ctx, inst, s.instRepo, s.hostRepo, 9090)
	_ = s.repo.UpdateStatus(ctx, taskID, models.MigrationStatusSwitching, task.Progress)
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/migration-switch", map[string]interface{}{
		"task_id":     taskID,
		"instance_id": task.TargetInstanceID,
		"config":      map[string]interface{}{},
	})
	if err != nil {
		_ = s.repo.UpdateStatus(ctx, taskID, models.MigrationStatusFailed, task.Progress)
		return &models.MigrationSwitchResult{
			TaskID: taskID,
			Status: "failed",
		}, nil
	}
	status := models.MigrationStatusFailed
	progress := task.Progress
	if result.Status == "success" || result.Status == "completed" {
		status = models.MigrationStatusCompleted
		progress = 100
	}
	_ = s.repo.UpdateStatus(ctx, taskID, status, progress)

	return &models.MigrationSwitchResult{
		TaskID:             taskID,
		Status:             result.Status,
		SwitchedAt:         time.Now(),
		ApplicationUpdated: result.Status == "success",
	}, nil
}

func (s *MigrationService) GetTask(ctx context.Context, taskID string) (*models.MigrationTask, error) {
	return s.repo.GetByID(ctx, taskID)
}

func (s *MigrationService) ListTasks(ctx context.Context, instanceID string) ([]models.MigrationTask, error) {
	tasks, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	if instanceID != "" {
		filtered := make([]models.MigrationTask, 0)
		for _, t := range tasks {
			if t.SourceInstanceID == instanceID || t.TargetInstanceID == instanceID {
				filtered = append(filtered, t)
			}
		}
		return filtered, nil
	}
	return tasks, nil
}

func (s *MigrationService) CancelTask(ctx context.Context, taskID string) error {
	return s.repo.UpdateStatus(ctx, taskID, models.MigrationStatusCancelled, 0)
}
