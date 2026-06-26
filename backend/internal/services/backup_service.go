package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type BackupService struct {
	hostRepo    *repositories.HostRepository
	instRepo    *repositories.InstanceRepository
	policyRepo  *repositories.BackupRepository
	taskRepo    *repositories.TaskRepository
	agentClient *AgentClient
	auditSvc    *AuditService
	encKey      string
}

func NewBackupService(hostRepo *repositories.HostRepository, instRepo *repositories.InstanceRepository, policyRepo *repositories.BackupRepository, agentClient *AgentClient, encKey string, opts ...interface{}) *BackupService {
	var audit *AuditService
	var taskRepo *repositories.TaskRepository
	for _, opt := range opts {
		switch v := opt.(type) {
		case *AuditService:
			audit = v
		case *repositories.TaskRepository:
			taskRepo = v
		}
	}
	return &BackupService{
		hostRepo:    hostRepo,
		instRepo:    instRepo,
		policyRepo:  policyRepo,
		taskRepo:    taskRepo,
		agentClient: agentClient,
		auditSvc:    audit,
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

type UpdateBackupPolicyRequest struct {
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
	PolicyID   string `json:"policy_id"`
}

type RestoreBackupRequest struct {
	BackupID         string `json:"backup_id" binding:"required"`
	TargetInstanceID string `json:"target_instance_id" binding:"required"`
	TargetType       string `json:"target_type"`
	ConfirmOverwrite bool   `json:"confirm_overwrite"`
}

type BackupTaskResult struct {
	ID          string    `json:"id"`
	InstanceID  string    `json:"instance_id"`
	BackupType  string    `json:"backup_type"`
	TaskID      string    `json:"task_id"`
	Status      string    `json:"status"`
	Message     string    `json:"message,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	FilePath    string    `json:"file_path"`
	FileSize    int64     `json:"file_size"`
	Size        string    `json:"size"`
	Checksum    string    `json:"checksum"`
	CreatedAt   time.Time `json:"created_at"`
}

type RestoreBackupResult struct {
	ID               string    `json:"id"`
	BackupID         string    `json:"backup_id"`
	TargetInstanceID string    `json:"target_instance_id"`
	TaskID           string    `json:"task_id"`
	Status           string    `json:"status"`
	Message          string    `json:"message,omitempty"`
	StartedAt        time.Time `json:"started_at"`
	CompletedAt      time.Time `json:"completed_at"`
}

type DiscoveredBackup struct {
	FileName        string    `json:"file_name"`
	FilePath        string    `json:"file_path"`
	SizeBytes       int64     `json:"size_bytes"`
	BackupType      string    `json:"backup_type"`
	IsDir           bool      `json:"is_dir"`
	Complete        bool      `json:"complete"`
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
	s.auditBackup(ctx, "create_backup_policy", "create", "backup_policy", policy.ID, "success", "",
		fmt.Sprintf("instance_id=%s backup_type=%s schedule=%s storage=%s:%s", policy.InstanceID, policy.BackupType, policy.Schedule, policy.StorageType, policy.StoragePath))
	return policy.ID, nil
}

func (s *BackupService) ListPolicies(ctx context.Context, instanceID string) ([]models.BackupPolicy, error) {
	return s.policyRepo.ListPolicies(ctx, instanceID, 100, 0)
}

func (s *BackupService) UpdatePolicy(ctx context.Context, policyID string, req UpdateBackupPolicyRequest) (*models.BackupPolicy, error) {
	if strings.TrimSpace(policyID) == "" {
		return nil, fmt.Errorf("backup policy id is required")
	}
	policy, err := s.policyRepo.GetPolicyByID(ctx, policyID)
	if err != nil {
		return nil, err
	}
	policy.InstanceID = req.InstanceID
	policy.BackupType = req.BackupType
	policy.Schedule = req.Schedule
	policy.RetentionDays = req.RetentionDays
	policy.StorageType = req.StorageType
	policy.StoragePath = req.StoragePath
	policy.Enabled = req.Enabled
	policy.UpdatedAt = time.Now()
	if policy.StorageType == "" {
		policy.StorageType = "local"
	}
	if policy.RetentionDays == 0 {
		policy.RetentionDays = 7
	}
	if err := s.policyRepo.UpdatePolicy(ctx, policy); err != nil {
		return nil, err
	}
	s.auditBackup(ctx, "update_backup_policy", "update", "backup_policy", policy.ID, "success", "",
		fmt.Sprintf("instance_id=%s backup_type=%s schedule=%s storage=%s:%s enabled=%v", policy.InstanceID, policy.BackupType, policy.Schedule, policy.StorageType, policy.StoragePath, policy.Enabled))
	return policy, nil
}

func (s *BackupService) DeletePolicy(ctx context.Context, policyID string) error {
	if strings.TrimSpace(policyID) == "" {
		return fmt.Errorf("backup policy id is required")
	}
	if err := s.policyRepo.DeletePolicy(ctx, policyID); err != nil {
		return err
	}
	s.auditBackup(ctx, "delete_backup_policy", "delete", "backup_policy", policyID, "success", "", "")
	return nil
}

func (s *BackupService) ExecuteBackup(ctx context.Context, req ExecuteBackupRequest) (*BackupTaskResult, error) {
	targetDir := "/backup/mysql"
	if v := os.Getenv("DBOPS_BACKUP_DIR"); v != "" {
		targetDir = v
	}
	if req.PolicyID != "" {
		policy, err := s.policyRepo.GetPolicyByID(ctx, req.PolicyID)
		if err != nil {
			return nil, err
		}
		req.InstanceID = policy.InstanceID
		req.BackupType = policy.BackupType
		if strings.TrimSpace(policy.StoragePath) != "" {
			targetDir = strings.TrimSpace(policy.StoragePath)
		}
	}
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
			if host.AgentPort != 0 {
				agentPort = host.AgentPort
			}
		}
	}
	if agentHost == "" {
		return nil, fmt.Errorf("cannot determine agent host")
	}
	conn, err := s.instRepo.GetConnection(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("instance connection not found: %w", err)
	}
	if conn.PasswordEncrypted == "" {
		return nil, fmt.Errorf("instance has no password configured, please update instance credentials first")
	}
	password, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt instance password: %w", err)
	}

	taskID := fmt.Sprintf("backup-%d-%s", time.Now().UnixNano(), uuid.NewString()[:8])
	now := time.Now()
	if s.taskRepo != nil {
		_ = s.taskRepo.Create(ctx, &models.Task{
			ID:         taskID,
			TaskType:   "backup",
			InstanceID: req.InstanceID,
			Status:     "pending",
			Progress:   0,
			CreatedAt:  now,
		})
	}
	if s.agentClient == nil {
		errMsg := "agent client not configured"
		out := &BackupTaskResult{
			InstanceID: req.InstanceID,
			BackupType: req.BackupType,
			TaskID:     taskID,
			Status:     "failed",
			Message:    errMsg,
			StartedAt:  now,
		}
		_ = s.createRecord(&models.BackupRecord{
			InstanceID: req.InstanceID,
			PolicyID:   req.PolicyID,
			BackupType: req.BackupType,
			TaskID:     taskID,
			StartedAt:  now,
			Status:     "failed",
			Message:    errMsg,
			CreatedAt:  now,
		})
		if s.taskRepo != nil {
			_ = s.taskRepo.UpdateStatusWithMessage(ctx, taskID, "failed", 0, errMsg)
		}
		s.auditBackupExecution(ctx, out, "failed")
		return out, nil
	}
	config := map[string]interface{}{
		"backup_type":   req.BackupType,
		"backup_method": "auto",
		"target_dir":    targetDir,
		"mysql_host":    localMySQLHostForAgent(conn.Host, agentHost),
		"mysql_port":    conn.Port,
		"mysql_user":    conn.Username,
		"mysql_pass":    password,
		"datadir":       conn.Datadir,
	}
	if req.BackupType == "incremental" {
		base, err := s.policyRepo.LatestCompletedRecord(ctx, req.InstanceID, "full")
		if err != nil || base.FilePath == "" {
			out := &BackupTaskResult{
				InstanceID: req.InstanceID,
				BackupType: req.BackupType,
				TaskID:     taskID,
				Status:     "failed",
				Message:    "Incremental backup requires at least one completed full backup",
				StartedAt:  now,
				CreatedAt:  now,
			}
			_ = s.createRecord(&models.BackupRecord{
				InstanceID: req.InstanceID,
				PolicyID:   req.PolicyID,
				BackupType: req.BackupType,
				TaskID:     taskID,
				StartedAt:  now,
				Status:     "failed",
				Message:    out.Message,
				CreatedAt:  now,
			})
			if s.taskRepo != nil {
				_ = s.taskRepo.UpdateStatusWithMessage(ctx, taskID, "failed", 0, out.Message)
			}
			s.auditBackupExecution(ctx, out, "failed")
			return out, nil
		}
		config["base_backup_path"] = base.FilePath
		config["base_backup_label"] = base.ID
		if isLogicalDumpBackupPath(base.FilePath) {
			config["backup_method"] = "mysqlbinlog"
		}
	}
	out := &BackupTaskResult{
		InstanceID: req.InstanceID,
		BackupType: req.BackupType,
		TaskID:     taskID,
		StartedAt:  now,
	}
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/backup", map[string]interface{}{
		"task_id":     taskID,
		"instance_id": req.InstanceID,
		"config":      config,
	})
	if err != nil {
		out.Status = "failed"
		out.Message = err.Error()
		// 真实落库失败记录, 而不是悄无声息.
		_ = s.createRecord(&models.BackupRecord{
			InstanceID: req.InstanceID,
			PolicyID:   req.PolicyID,
			BackupType: req.BackupType,
			TaskID:     taskID,
			StartedAt:  now,
			Status:     "failed",
			Message:    out.Message,
			CreatedAt:  now,
		})
		if s.taskRepo != nil {
			_ = s.taskRepo.UpdateStatusWithMessage(ctx, taskID, "failed", 0, out.Message)
		}
		s.auditBackupExecution(ctx, out, "failed")
		return out, nil
	}
	out.Status = normalizeBackupAgentStatus(result.Status)
	out.Message = result.Message
	if isTerminalBackupStatus(out.Status) {
		out.CompletedAt = time.Now()
	}
	out.FilePath = stringValue(result.Data["backup_path"])
	out.FileSize = int64Value(result.Data["file_size"])
	out.Checksum = stringValue(result.Data["checksum"])
	if out.FilePath == "" {
		out.FilePath = stringValueFromMessage(result.Message)
	}
	out.Size = formatBytes(out.FileSize)

	_ = s.createRecord(&models.BackupRecord{
		InstanceID:  req.InstanceID,
		PolicyID:    req.PolicyID,
		BackupType:  req.BackupType,
		TaskID:      taskID,
		StartedAt:   now,
		CompletedAt: out.CompletedAt,
		Status:      out.Status,
		Message:     out.Message,
		FilePath:    out.FilePath,
		FileSize:    out.FileSize,
		Checksum:    out.Checksum,
		CreatedAt:   now,
	})
	if s.taskRepo != nil {
		progress := result.Progress
		if progress == 0 && isTerminalBackupStatus(out.Status) {
			progress = 100
		}
		_ = s.taskRepo.UpdateStatusWithMessage(ctx, taskID, out.Status, progress, out.Message)
		_ = s.taskRepo.AddLog(ctx, &models.TaskLog{
			TaskID:    taskID,
			LogID:     uuid.NewString(),
			Timestamp: time.Now(),
			Level:     backupTaskLogLevel(out.Status),
			Message:   out.Message,
			Context:   fmt.Sprintf("backup_type=%s backup_method=%s file_path=%s checksum=%s", out.BackupType, stringValue(result.Data["backup_method"]), out.FilePath, out.Checksum),
		})
	}
	s.auditBackupExecution(ctx, out, backupAuditResult(out.Status))
	return out, nil
}

func (s *BackupService) auditBackupExecution(ctx context.Context, result *BackupTaskResult, auditResult string) {
	if result == nil {
		return
	}
	details := fmt.Sprintf("instance_id=%s backup_type=%s task_id=%s status=%s file_path=%s message=%s",
		result.InstanceID, result.BackupType, result.TaskID, result.Status, result.FilePath, result.Message)
	s.auditBackup(ctx, "execute_backup", "execute", "backup_record", result.TaskID, auditResult, result.Message, details)
}

func (s *BackupService) auditBackup(ctx context.Context, operation, action, resourceType, resourceID, result, errorMsg, details string) {
	if s.auditSvc == nil {
		return
	}
	_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID:       userIDFromCtx(ctx),
		Operation:    operation,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		Details:      details,
		Result:       result,
		ErrorMsg:     errorMsg,
	})
}

func backupAuditResult(status string) string {
	switch normalizeBackupAgentStatus(status) {
	case "failed", "error", "timeout", "cancelled", "canceled":
		return "failed"
	default:
		return "success"
	}
}

func isTerminalBackupStatus(status string) bool {
	switch normalizeBackupAgentStatus(status) {
	case "completed", "success", "succeeded", "ok", "failed", "error", "timeout", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func isActiveBackupStatus(status string) bool {
	switch normalizeBackupAgentStatus(status) {
	case "pending", "running", "submitted", "accepted", "queued":
		return true
	default:
		return false
	}
}

func backupTaskLogLevel(status string) string {
	switch normalizeBackupAgentStatus(status) {
	case "failed":
		return "error"
	case "completed":
		return "info"
	default:
		return "info"
	}
}

func normalizeBackupAgentStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "succeeded", "ok", "completed":
		return "completed"
	case "failed", "error", "timeout", "cancelled", "canceled", "unhealthy":
		return "failed"
	case "pending", "running", "submitted", "accepted", "queued":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func isLogicalDumpBackupPath(path string) bool {
	normalized := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(normalized, ".sql") || strings.HasSuffix(normalized, ".sql.gz")
}

func (s *BackupService) createRecord(record *models.BackupRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.policyRepo.CreateRecord(ctx, record)
}

// ListBackups P0-4: 真实从 backup_records 读, 不再返回空.
func (s *BackupService) ListBackups(ctx context.Context, instanceID string) ([]BackupTaskResult, error) {
	records, err := s.policyRepo.ListRecords(ctx, instanceID, 100, 0)
	if err != nil {
		return nil, err
	}
	s.refreshActiveBackupRecords(ctx, records)
	out := make([]BackupTaskResult, 0, len(records))
	for _, r := range records {
		taskID := r.TaskID
		if taskID == "" {
			taskID = r.ID
		}
		out = append(out, BackupTaskResult{
			ID:          r.ID,
			InstanceID:  r.InstanceID,
			BackupType:  r.BackupType,
			TaskID:      taskID,
			Status:      r.Status,
			Message:     r.Message,
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

func (s *BackupService) refreshActiveBackupRecords(ctx context.Context, records []models.BackupRecord) {
	if s.agentClient == nil {
		return
	}
	for i := range records {
		if !isActiveBackupStatus(records[i].Status) || records[i].TaskID == "" {
			continue
		}
		inst, err := s.instRepo.GetByID(ctx, records[i].InstanceID)
		if err != nil {
			continue
		}
		host, port, err := s.resolveAgentEndpoint(ctx, inst)
		if err != nil {
			continue
		}
		progressCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		result, err := s.agentClient.GetTaskProgress(progressCtx, host, port, records[i].TaskID)
		cancel()
		if err != nil || result == nil {
			continue
		}
		records[i].Status = normalizeBackupAgentStatus(result.Status)
		records[i].Message = result.Message
		if path := stringValue(result.Data["backup_path"]); path != "" {
			records[i].FilePath = path
		}
		if size := int64Value(result.Data["file_size"]); size > 0 {
			records[i].FileSize = size
		}
		if checksum := stringValue(result.Data["checksum"]); checksum != "" {
			records[i].Checksum = checksum
		}
		if isTerminalBackupStatus(records[i].Status) && records[i].CompletedAt.IsZero() {
			records[i].CompletedAt = time.Now()
		}
		_ = s.policyRepo.UpdateRecord(context.Background(), &records[i])
	}
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
	if s.agentClient == nil {
		return nil, fmt.Errorf("agent client not configured")
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
	if result == nil {
		return nil, fmt.Errorf("agent backup scan failed: empty result")
	}
	if normalizeBackupAgentStatus(result.Status) == "failed" {
		if result.Message != "" {
			return nil, fmt.Errorf("agent backup scan failed: %s", result.Message)
		}
		return nil, fmt.Errorf("agent backup scan failed")
	}

	discovered := dedupeDiscoveredBackups(decodeDiscoveredBackups(result.Data))
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
	for i := range discovered {
		if discovered[i].AlreadyManaged || discovered[i].FilePath == "" || discovered[i].BackupType == "" {
			continue
		}
		now := time.Now()
		record := &models.BackupRecord{
			InstanceID:  instanceID,
			BackupType:  discovered[i].BackupType,
			TaskID:      "backup-scan-" + uuid.NewString(),
			StartedAt:   discovered[i].MTime,
			CompletedAt: discovered[i].MTime,
			Status:      "completed",
			Message:     "discovered by backup scan",
			FilePath:    discovered[i].FilePath,
			FileSize:    discovered[i].SizeBytes,
			CreatedAt:   now,
		}
		if record.StartedAt.IsZero() {
			record.StartedAt = now
		}
		if record.CompletedAt.IsZero() {
			record.CompletedAt = record.StartedAt
		}
		if err := s.policyRepo.CreateRecord(ctx, record); err != nil {
			return nil, fmt.Errorf("failed to register scanned backup %s: %w", discovered[i].FilePath, err)
		}
		discovered[i].AlreadyManaged = true
		discovered[i].ManagedBackupID = record.ID
		managed[record.FilePath] = record.ID
		managed[filepath.Base(record.FilePath)] = record.ID
	}
	return &BackupScanResult{Backups: discovered, ScannedAt: time.Now()}, nil
}

func (s *BackupService) RestoreBackup(ctx context.Context, req RestoreBackupRequest) (*RestoreBackupResult, error) {
	if req.BackupID == "" || req.TargetInstanceID == "" {
		return nil, fmt.Errorf("backup_id and target_instance_id are required")
	}
	if !req.ConfirmOverwrite {
		return nil, fmt.Errorf("restore requires confirm_overwrite=true")
	}
	backup, err := s.policyRepo.GetRecordByID(ctx, req.BackupID)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(strings.TrimSpace(backup.Status)) != "completed" {
		return nil, fmt.Errorf("backup record is not completed")
	}
	if backup.FilePath == "" {
		return nil, fmt.Errorf("backup record has no file_path")
	}
	target, err := s.instRepo.GetByID(ctx, req.TargetInstanceID)
	if err != nil {
		return nil, fmt.Errorf("target instance not found: %w", err)
	}
	conn, err := s.instRepo.GetConnection(ctx, req.TargetInstanceID)
	if err != nil {
		return nil, fmt.Errorf("target instance connection not found: %w", err)
	}
	password, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt target instance password: %w", err)
	}
	agentHost, agentPort, err := s.resolveAgentEndpoint(ctx, target)
	if err != nil {
		return nil, err
	}
	taskID := fmt.Sprintf("restore-%d-%s", time.Now().UnixNano(), uuid.NewString()[:8])
	startedAt := time.Now()
	if s.taskRepo != nil {
		_ = s.taskRepo.Create(ctx, &models.Task{
			ID:         taskID,
			TaskType:   "restore",
			InstanceID: req.TargetInstanceID,
			Status:     "pending",
			Progress:   0,
			CreatedAt:  startedAt,
		})
	}
	out := &RestoreBackupResult{
		BackupID:         req.BackupID,
		TargetInstanceID: req.TargetInstanceID,
		TaskID:           taskID,
		Status:           "failed",
		StartedAt:        startedAt,
	}
	if s.agentClient == nil {
		out.Message = "agent client not configured"
	} else {
		result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/restore", map[string]interface{}{
			"task_id":     taskID,
			"instance_id": req.TargetInstanceID,
			"config": map[string]interface{}{
				"backup_path":       backup.FilePath,
				"mysql_host":        localMySQLHostForAgent(conn.Host, agentHost),
				"mysql_port":        conn.Port,
				"mysql_user":        conn.Username,
				"mysql_pass":        password,
				"target_type":       req.TargetType,
				"confirm_overwrite": req.ConfirmOverwrite,
			},
		})
		if err != nil {
			out.Message = err.Error()
		} else {
			out.Status = normalizeBackupAgentStatus(result.Status)
			out.Message = result.Message
			if isTerminalBackupStatus(out.Status) {
				out.CompletedAt = time.Now()
			}
		}
	}
	if s.taskRepo != nil {
		progress := 0
		if isTerminalBackupStatus(out.Status) {
			progress = 100
		}
		_ = s.taskRepo.UpdateStatusWithMessage(ctx, taskID, out.Status, progress, out.Message)
		_ = s.taskRepo.AddLog(ctx, &models.TaskLog{
			TaskID:    taskID,
			LogID:     uuid.NewString(),
			Timestamp: time.Now(),
			Level:     backupTaskLogLevel(out.Status),
			Message:   out.Message,
			Context:   fmt.Sprintf("backup_id=%s target_instance_id=%s target_type=%s", req.BackupID, req.TargetInstanceID, req.TargetType),
		})
	}
	restoreRecord := &models.RestoreRecord{
		BackupID:         req.BackupID,
		TargetInstanceID: req.TargetInstanceID,
		StartedAt:        out.StartedAt,
		CompletedAt:      out.CompletedAt,
		Status:           out.Status,
		CreatedAt:        startedAt,
	}
	if err := s.policyRepo.CreateRestoreRecord(ctx, restoreRecord); err != nil {
		return nil, err
	}
	out.ID = restoreRecord.ID
	s.auditBackup(ctx, "restore_backup", "restore", "backup_record", req.BackupID, backupAuditResult(out.Status), out.Message,
		fmt.Sprintf("backup_id=%s target_instance_id=%s task_id=%s status=%s backup_path=%s", req.BackupID, req.TargetInstanceID, taskID, out.Status, backup.FilePath))
	return out, nil
}

func (s *BackupService) DeleteBackupRecord(ctx context.Context, backupID string) error {
	if backupID == "" {
		return fmt.Errorf("backup_id is required")
	}
	backup, err := s.policyRepo.GetRecordByID(ctx, backupID)
	if err != nil {
		return err
	}
	if err := s.policyRepo.DeleteRecord(ctx, backupID); err != nil {
		return err
	}
	s.auditBackup(ctx, "delete_backup_record", "delete", "backup_record", backupID, "success", "",
		fmt.Sprintf("backup_id=%s instance_id=%s file_path=%s", backup.ID, backup.InstanceID, backup.FilePath))
	return nil
}

func localMySQLHostForAgent(mysqlHost, agentHost string) string {
	mysqlHost = strings.TrimSpace(mysqlHost)
	agentHost = strings.TrimSpace(agentHost)
	if mysqlHost == "" || mysqlHost == agentHost {
		return "127.0.0.1"
	}
	return mysqlHost
}

func dedupeDiscoveredBackups(items []DiscoveredBackup) []DiscoveredBackup {
	seen := map[string]bool{}
	out := make([]DiscoveredBackup, 0, len(items))
	for _, item := range items {
		key := filepath.Clean(item.FilePath)
		if key == "." || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
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
		backup := DiscoveredBackup{
			FileName:   stringValue(m["file_name"]),
			FilePath:   stringValue(m["file_path"]),
			SizeBytes:  int64Value(m["size_bytes"]),
			BackupType: normalizeDiscoveredBackupType(stringValue(m["backup_type"])),
			IsDir:      boolValue(m["is_dir"]),
			Complete:   boolValue(m["complete"]),
			DetectedAt: timeValue(m["detected_at"], time.Now()),
			MTime:      timeValue(m["mtime"], time.Now()),
		}
		if backup.FileName == "" && backup.FilePath != "" {
			backup.FileName = filepath.Base(filepath.Clean(backup.FilePath))
		}
		if backup.BackupType == "" {
			backup.BackupType = inferDiscoveredBackupType(backup.FileName, backup.FilePath)
		}
		if !isCompleteDiscoveredBackup(backup) {
			continue
		}
		out = append(out, backup)
	}
	return out
}

func isCompleteDiscoveredBackup(item DiscoveredBackup) bool {
	if strings.TrimSpace(item.FilePath) == "" || normalizeDiscoveredBackupType(item.BackupType) == "" {
		return false
	}
	if item.Complete {
		return true
	}
	if item.IsDir {
		return false
	}
	name := strings.ToLower(filepath.Base(filepath.Clean(item.FilePath)))
	if item.SizeBytes <= 0 || isIncompleteBackupPath(name) {
		return false
	}
	return isCompleteBackupFileName(name)
}

func normalizeDiscoveredBackupType(backupType string) string {
	switch strings.ToLower(strings.TrimSpace(backupType)) {
	case "full", "incremental", "logical":
		return strings.ToLower(strings.TrimSpace(backupType))
	default:
		return ""
	}
}

func inferDiscoveredBackupType(parts ...string) string {
	text := strings.ToLower(strings.Join(parts, " "))
	if isIncompleteBackupPath(text) {
		return ""
	}
	switch {
	case strings.Contains(text, "incremental"), strings.Contains(text, "inc-"), strings.Contains(text, "_inc"):
		return "incremental"
	case strings.Contains(text, "logical"), strings.Contains(text, "dump"), strings.HasSuffix(text, ".sql"), strings.HasSuffix(text, ".sql.gz"):
		return "logical"
	case strings.Contains(text, "full"), strings.Contains(text, ".xbstream"), strings.Contains(text, ".xtrabackup"):
		return "full"
	default:
		return ""
	}
}

func isIncompleteBackupPath(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(lower, ".tmp") || strings.HasSuffix(lower, ".partial") || strings.HasSuffix(lower, ".incomplete")
}

func isCompleteBackupFileName(name string) bool {
	return strings.HasSuffix(name, ".xbstream") ||
		strings.HasSuffix(name, ".xtrabackup") ||
		strings.HasSuffix(name, ".sql") ||
		strings.HasSuffix(name, ".sql.gz") ||
		strings.HasSuffix(name, ".dump")
}

func stringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func stringValueFromMessage(message string) string {
	const marker = "Path: "
	idx := strings.Index(message, marker)
	if idx < 0 {
		return ""
	}
	value := strings.TrimSpace(message[idx+len(marker):])
	if comma := strings.Index(value, ","); comma >= 0 {
		value = strings.TrimSpace(value[:comma])
	}
	return value
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

func boolValue(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		return strings.EqualFold(strings.TrimSpace(s), "true")
	}
	return false
}

func timeValue(v interface{}, fallback time.Time) time.Time {
	if t, ok := v.(time.Time); ok {
		return t
	}
	if s, ok := v.(string); ok && s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return fallback
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
