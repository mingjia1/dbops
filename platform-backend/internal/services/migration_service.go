package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type MigrationService struct {
	repo        *repositories.MigrationRepository
	instRepo    *repositories.InstanceRepository
	hostRepo    *repositories.HostRepository
	agentClient *AgentClient
	auditSvc    *AuditService
}

func NewMigrationService(repo *repositories.MigrationRepository, instRepo *repositories.InstanceRepository, hostRepo *repositories.HostRepository, agentClient *AgentClient, auditSvc ...*AuditService) *MigrationService {
	var audit *AuditService
	if len(auditSvc) > 0 {
		audit = auditSvc[0]
	}
	return &MigrationService{
		repo:        repo,
		instRepo:    instRepo,
		hostRepo:    hostRepo,
		agentClient: agentClient,
		auditSvc:    audit,
	}
}

// resolveAgentHost returns the concrete agent endpoint for an instance.
// It must never silently fall back to localhost: that can dispatch migration
// or upgrade work to the backend machine instead of the managed MySQL host.
func resolveAgentHost(ctx context.Context, inst *models.Instance, instRepo *repositories.InstanceRepository, hostRepo *repositories.HostRepository, defaultPort int) (string, int, error) {
	if inst == nil {
		return "", 0, fmt.Errorf("cannot resolve agent endpoint: instance is nil")
	}
	if inst.HostID != nil && *inst.HostID != "" && hostRepo != nil {
		host, err := hostRepo.GetByID(ctx, *inst.HostID)
		if err != nil || host == nil {
			return "", 0, fmt.Errorf("cannot resolve agent endpoint: host %s not found", *inst.HostID)
		}
		if host.Address == "" {
			return "", 0, fmt.Errorf("cannot resolve agent endpoint: host %s has no address", *inst.HostID)
		}
		port := defaultPort
		if host.AgentPort > 0 {
			port = host.AgentPort
		}
		return host.Address, port, nil
	}
	if inst.Connection.Host != "" {
		return inst.Connection.Host, defaultPort, nil
	}
	if instRepo != nil {
		conn, err := instRepo.GetConnection(ctx, inst.ID)
		if err == nil && conn != nil && conn.Host != "" {
			return conn.Host, defaultPort, nil
		}
	}
	return "", 0, fmt.Errorf("cannot resolve agent endpoint for instance %s: no host association or connection host", inst.ID)
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
	Message   string                   `json:"message,omitempty"`
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
	s.auditMigration(ctx, "create_migration_task", "create", task.ID, "success", "",
		fmt.Sprintf("name=%s source_instance_id=%s target_instance_id=%s strategy=%s", task.Name, task.SourceInstanceID, task.TargetInstanceID, task.Strategy))
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

	// Resolve the real managed host agent. Do not dispatch migration to localhost.
	sourceInstFull, _ := s.instRepo.GetByID(ctx, task.SourceInstanceID)
	agentHost, agentPort, err := resolveAgentHost(ctx, sourceInstFull, s.instRepo, s.hostRepo, 9090)
	if err != nil {
		s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, 0, err.Error())
		out := &MigrationTaskResult{
			TaskID:    taskID,
			Status:    models.MigrationStatusFailed,
			Strategy:  strategy,
			StartedAt: now,
			Progress:  0,
			Message:   err.Error(),
		}
		s.auditMigrationExecution(ctx, task, out, err.Error())
		return out, nil
	}
	if s.agentClient == nil {
		message := "agent client not configured"
		_ = s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, 0, message)
		out := &MigrationTaskResult{
			TaskID:    taskID,
			Status:    models.MigrationStatusFailed,
			Strategy:  strategy,
			StartedAt: now,
			Progress:  0,
			Message:   message,
		}
		s.auditMigrationExecution(ctx, task, out, message)
		return out, nil
	}
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/migration", map[string]interface{}{
		"task_id":     taskID,
		"instance_id": task.SourceInstanceID,
		"config":      config,
	})
	if err != nil {
		s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, 0, err.Error())
		out := &MigrationTaskResult{
			TaskID:    taskID,
			Status:    models.MigrationStatusFailed,
			Strategy:  strategy,
			StartedAt: now,
			Progress:  0,
			Message:   err.Error(),
		}
		s.auditMigrationExecution(ctx, task, out, err.Error())
		return out, nil
	}

	if result == nil {
		message := "agent returned no migration result"
		_ = s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, 0, message)
		out := &MigrationTaskResult{
			TaskID:    taskID,
			Status:    models.MigrationStatusFailed,
			Strategy:  strategy,
			StartedAt: now,
			Progress:  0,
			Message:   message,
		}
		s.auditMigrationExecution(ctx, task, out, message)
		return out, nil
	}
	status := normalizeMigrationStatus(result.Status)
	if status == models.MigrationStatusFailed {
		s.repo.UpdateStatusWithError(ctx, taskID, status, result.Progress, result.Message)
	} else {
		s.repo.UpdateStatus(ctx, taskID, status, result.Progress)
	}

	out := &MigrationTaskResult{
		TaskID:    taskID,
		Status:    status,
		Strategy:  strategy,
		StartedAt: now,
		Progress:  result.Progress,
		Message:   result.Message,
	}
	s.auditMigrationExecution(ctx, task, out, result.Message)
	return out, nil
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
	// Missing tasks must return an explicit error instead of fake progress.
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
	agentHost, agentPort, err := resolveAgentHost(ctx, inst, s.instRepo, s.hostRepo, 9090)
	if err != nil {
		return nil, err
	}
	if s.agentClient == nil {
		return nil, fmt.Errorf("agent client not configured")
	}
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
	status := normalizeMigrationStatus(result.Status)
	if status == models.MigrationStatusFailed {
		message := strings.TrimSpace(result.Message)
		if message == "" {
			message = "agent migration monitor failed"
		}
		_ = s.repo.UpdateStatusWithError(ctx, taskID, status, result.Progress, message)
	} else {
		_ = s.repo.UpdateStatus(ctx, taskID, status, result.Progress)
	}

	return &models.MigrationProgress{
		TaskID:    taskID,
		Status:    status,
		Progress:  result.Progress,
		UpdatedAt: time.Now(),
	}, nil
}

// VerifyMigration calls Agent for real verification results.
func (s *MigrationService) VerifyMigration(ctx context.Context, taskID string) (*models.MigrationVerification, error) {
	task, err := s.repo.GetByID(ctx, taskID)
	if err != nil || task == nil {
		return nil, fmt.Errorf("migration task %s not found", taskID)
	}
	inst, err := s.instRepo.GetByID(ctx, task.TargetInstanceID)
	if err != nil || inst == nil {
		return nil, fmt.Errorf("target instance %s not found", task.TargetInstanceID)
	}
	agentHost, agentPort, err := resolveAgentHost(ctx, inst, s.instRepo, s.hostRepo, 9090)
	if err != nil {
		_ = s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, task.Progress, err.Error())
		out := &models.MigrationVerification{
			TaskID:     taskID,
			VerifiedAt: time.Now(),
			Errors:     []string{err.Error()},
		}
		return out, nil
	}
	out := &models.MigrationVerification{
		TaskID:     taskID,
		VerifiedAt: time.Now(),
	}
	if s.agentClient == nil {
		message := "agent client not configured"
		out.Errors = []string{message}
		_ = s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, task.Progress, message)
		return out, nil
	}
	result, callErr := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/migration-verify", map[string]interface{}{
		"task_id":            taskID,
		"source_instance_id": task.SourceInstanceID,
		"target_instance_id": task.TargetInstanceID,
	})
	if callErr != nil {
		out.Errors = []string{"agent verify call failed: " + callErr.Error()}
		_ = s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, task.Progress, out.Errors[0])
		return out, nil
	}
	if result == nil {
		out.Errors = []string{"agent verify returned no result"}
		_ = s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, task.Progress, out.Errors[0])
		return out, nil
	}
	if normalizeMigrationStatus(result.Status) == models.MigrationStatusFailed {
		msg := strings.TrimSpace(result.Message)
		if msg == "" {
			msg = "agent verification failed"
		}
		out.Errors = []string{msg}
		_ = s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, task.Progress, msg)
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
		_ = s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, task.Progress, strings.Join(out.Errors, "; "))
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
	agentHost, agentPort, err := resolveAgentHost(ctx, inst, s.instRepo, s.hostRepo, 9090)
	if err != nil {
		_ = s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, task.Progress, err.Error())
		out := &models.MigrationSwitchResult{
			TaskID: taskID,
			Status: "failed",
		}
		s.auditMigrationSwitch(ctx, task, out, err.Error())
		return out, nil
	}
	_ = s.repo.UpdateStatus(ctx, taskID, models.MigrationStatusSwitching, task.Progress)
	if s.agentClient == nil {
		message := "agent client not configured"
		_ = s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, task.Progress, message)
		out := &models.MigrationSwitchResult{
			TaskID: taskID,
			Status: "failed",
		}
		s.auditMigrationSwitch(ctx, task, out, message)
		return out, nil
	}
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/migration-switch", map[string]interface{}{
		"task_id":     taskID,
		"instance_id": task.TargetInstanceID,
		"config":      map[string]interface{}{},
	})
	if err != nil {
		_ = s.repo.UpdateStatusWithError(ctx, taskID, models.MigrationStatusFailed, task.Progress, err.Error())
		out := &models.MigrationSwitchResult{
			TaskID: taskID,
			Status: "failed",
		}
		s.auditMigrationSwitch(ctx, task, out, err.Error())
		return out, nil
	}
	status := models.MigrationStatusFailed
	progress := task.Progress
	completed := isCompletedMigrationAgentStatus(result.Status)
	if completed {
		status = models.MigrationStatusCompleted
		progress = 100
	}
	if status == models.MigrationStatusFailed {
		_ = s.repo.UpdateStatusWithError(ctx, taskID, status, progress, result.Message)
	} else {
		_ = s.repo.UpdateStatus(ctx, taskID, status, progress)
	}

	out := &models.MigrationSwitchResult{
		TaskID:             taskID,
		Status:             string(status),
		SwitchedAt:         time.Now(),
		ApplicationUpdated: completed,
	}
	s.auditMigrationSwitch(ctx, task, out, result.Message)
	return out, nil
}

func normalizeMigrationStatus(status string) models.MigrationStatus {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "completed", "success", "succeeded", "ok":
		return models.MigrationStatusCompleted
	case "failed", "error", "timeout":
		return models.MigrationStatusFailed
	case "cancelled", "canceled":
		return models.MigrationStatusCancelled
	case "preparing", "accepted", "submitted", "queued", "pending":
		return models.MigrationStatusPreparing
	case "verifying":
		return models.MigrationStatusVerifying
	case "switching":
		return models.MigrationStatusSwitching
	case "running", "migrating":
		return models.MigrationStatusMigrating
	default:
		return models.MigrationStatusMigrating
	}
}

func isCompletedMigrationAgentStatus(status string) bool {
	return normalizeMigrationStatus(status) == models.MigrationStatusCompleted
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
	err := s.repo.UpdateStatus(ctx, taskID, models.MigrationStatusCancelled, 0)
	if err == nil {
		s.auditMigration(ctx, "cancel_migration_task", "cancel", taskID, "success", "",
			fmt.Sprintf("task_id=%s status=%s", taskID, models.MigrationStatusCancelled))
	}
	return err
}

func (s *MigrationService) auditMigrationExecution(ctx context.Context, task *models.MigrationTask, result *MigrationTaskResult, message string) {
	if task == nil || result == nil {
		return
	}
	details := fmt.Sprintf("task_id=%s source_instance_id=%s target_instance_id=%s strategy=%s status=%s progress=%d message=%s",
		task.ID, task.SourceInstanceID, task.TargetInstanceID, result.Strategy, result.Status, result.Progress, message)
	s.auditMigration(ctx, "execute_migration", "execute", task.ID, migrationAuditResult(string(result.Status)), migrationAuditError(string(result.Status), message), details)
}

func (s *MigrationService) auditMigrationSwitch(ctx context.Context, task *models.MigrationTask, result *models.MigrationSwitchResult, message string) {
	if task == nil || result == nil {
		return
	}
	details := fmt.Sprintf("task_id=%s source_instance_id=%s target_instance_id=%s status=%s application_updated=%t message=%s",
		task.ID, task.SourceInstanceID, task.TargetInstanceID, result.Status, result.ApplicationUpdated, message)
	s.auditMigration(ctx, "switch_migration", "switch", task.ID, migrationAuditResult(result.Status), migrationAuditError(result.Status, message), details)
}

func (s *MigrationService) auditMigration(ctx context.Context, operation, action, resourceID, result, errorMsg, details string) {
	if s.auditSvc == nil {
		return
	}
	_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID:       userIDFromCtx(ctx),
		Operation:    operation,
		ResourceType: "migration_task",
		ResourceID:   resourceID,
		Action:       action,
		Details:      details,
		Result:       result,
		ErrorMsg:     errorMsg,
	})
}

func migrationAuditResult(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "timeout", "cancelled", "canceled":
		return "failed"
	default:
		return "success"
	}
}

func migrationAuditError(status, message string) string {
	if migrationAuditResult(status) != "failed" {
		return ""
	}
	return message
}
