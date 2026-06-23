package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type InstanceService struct {
	repo        *repositories.InstanceRepository
	hostRepo    *repositories.HostRepository
	taskRepo    *repositories.TaskRepository
	agentClient *AgentClient
	backupSvc   *BackupService
	auditSvc    *AuditService // P0: 删/更新/部署 写 audit, SOC2 合规
	encKey      string
}

func NewInstanceService(repo *repositories.InstanceRepository, hostRepo *repositories.HostRepository, taskRepo *repositories.TaskRepository, agentClient *AgentClient, auditSvc *AuditService, encKey string) *InstanceService {
	return &InstanceService{
		repo:        repo,
		hostRepo:    hostRepo,
		taskRepo:    taskRepo,
		agentClient: agentClient,
		auditSvc:    auditSvc,
		encKey:      encKey,
	}
}

func (s *InstanceService) SetBackupService(backupSvc *BackupService) {
	s.backupSvc = backupSvc
}

func (s *InstanceService) Create(ctx context.Context, req CreateInstanceRequest) (*models.Instance, error) {
	if req.VersionID != "" && req.PackageURL == "" {
		entry, err := NewVersionCatalog().Get(req.VersionID)
		if err != nil {
			return nil, err
		}
		req.PackageURL = entry.PackageURL
	}

	instance := &models.Instance{
		Name:      req.Name,
		ClusterID: req.ClusterID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if req.HostID != "" {
		hid := req.HostID
		instance.HostID = &hid
	}

	if err := s.repo.Create(ctx, instance); err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	conn := &models.InstanceConnection{
		InstanceID: instance.ID,
		Host:       req.Host,
		Port:       req.Port,
		Username:   req.Username,
		SSLEnabled: req.SSLEnabled,
		Basedir:    req.Basedir,
		Datadir:    req.Datadir,
		OSUser:     req.OSUser,
		PackageURL: req.PackageURL,
		VersionID:  req.VersionID,
	}
	// P1-3: 落库前用 AES-GCM 加密, 与 host.SSHCredential 一致.
	enc, err := utils.Encrypt(req.Password, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt password: %w", err)
	}
	conn.PasswordEncrypted = enc

	if err := s.repo.CreateConnection(ctx, conn); err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	return instance, nil
}

func (s *InstanceService) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *InstanceService) List(ctx context.Context, limit, offset int) ([]models.Instance, error) {
	instances, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	return s.enrichListedInstances(ctx, instances), nil
}

func (s *InstanceService) ListByHostID(ctx context.Context, hostID string, limit, offset int) ([]models.Instance, error) {
	instances, err := s.repo.ListByHostID(ctx, hostID, limit, offset)
	if err != nil {
		return nil, err
	}
	return s.enrichListedInstances(ctx, instances), nil
}

func (s *InstanceService) enrichListedInstances(ctx context.Context, instances []models.Instance) []models.Instance {
	for i := range instances {
		full, err := s.repo.GetByID(ctx, instances[i].ID)
		if err == nil && full != nil {
			instances[i] = *full
		}
	}
	return instances
}

func (s *InstanceService) Update(ctx context.Context, id string, req UpdateInstanceRequest) (*models.Instance, error) {
	instance, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		instance.Name = req.Name
	}
	if req.ClusterID != "" {
		instance.ClusterID = req.ClusterID
	}
	if req.HostID != "" {
		hid := req.HostID
		instance.HostID = &hid
	}
	instance.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, instance); err != nil {
		return nil, fmt.Errorf("failed to update instance: %w", err)
	}
	if req.HasConnectionUpdate() {
		conn, err := s.repo.GetConnection(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("instance connection not found: %w", err)
		}
		if req.Host != "" {
			conn.Host = req.Host
		}
		if req.Port != 0 {
			conn.Port = req.Port
		}
		if req.Username != "" {
			conn.Username = req.Username
		}
		if req.Password != "" {
			enc, err := utils.Encrypt(req.Password, s.encKey)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt password: %w", err)
			}
			conn.PasswordEncrypted = enc
		}
		if req.SSLEnabled != nil {
			conn.SSLEnabled = *req.SSLEnabled
		}
		if req.Basedir != "" {
			conn.Basedir = req.Basedir
		}
		if req.Datadir != "" {
			conn.Datadir = req.Datadir
		}
		if req.OSUser != "" {
			conn.OSUser = req.OSUser
		}
		if req.PackageURL != "" {
			conn.PackageURL = req.PackageURL
		}
		if req.VersionID != "" {
			conn.VersionID = req.VersionID
		}
		if strings.TrimSpace(conn.Host) == "" {
			return nil, fmt.Errorf("connection host is required")
		}
		if conn.Port <= 0 || conn.Port > 65535 {
			return nil, fmt.Errorf("connection port must be between 1 and 65535")
		}
		if strings.TrimSpace(conn.Username) == "" {
			return nil, fmt.Errorf("connection username is required")
		}
		if err := s.repo.UpdateConnection(ctx, conn); err != nil {
			return nil, err
		}
	}
	if s.auditSvc != nil {
		_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
			UserID:       userIDFromCtx(ctx),
			Action:       "update",
			ResourceType: "instance",
			ResourceID:   id,
			Result:       "success",
		})
	}

	updated, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *InstanceService) Delete(ctx context.Context, id string) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return err
	}

	// 尝试备份和退役
	backup, err := s.backupAndDecommission(ctx, id)
	if err != nil {
		// 如果备份/退役失败（实例离线、无密码或agent不可达），跳过备份直接删除
		log.Printf("WARN: backup/decommission failed for instance %s, proceeding with direct delete: %v", id, err)
		log.Printf("INFO: Deleting instance %s without backup (instance may be offline or incomplete)", id)

		if delErr := s.repo.Delete(ctx, id); delErr != nil {
			return delErr
		}

		s.auditInstanceDelete(ctx, id, "success", "deleted without backup: "+err.Error(), "")
		return nil
	}

	// 正常流程：备份成功后删除
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	// P0: 写 audit, 之前删 instance 0 记录, SOC2 不通过.
	s.auditInstanceDelete(ctx, id, "success", "", fmt.Sprintf("backup_id=%s backup_path=%s", backup.ID, backup.FilePath))
	return nil
}

func (s *InstanceService) backupAndDecommission(ctx context.Context, id string) (*BackupTaskResult, error) {
	backup, err := s.executeRemovalBackup(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := validateRemovalBackup(backup); err != nil {
		return backup, err
	}
	if err := s.decommissionRemoteInstance(ctx, id); err != nil {
		return backup, err
	}
	return backup, nil
}

func (s *InstanceService) executeRemovalBackup(ctx context.Context, id string) (*BackupTaskResult, error) {
	if s.backupSvc == nil {
		return nil, fmt.Errorf("backup service is required before removing instance %s", id)
	}
	backup, err := s.backupSvc.ExecuteBackup(ctx, ExecuteBackupRequest{
		InstanceID: id,
		BackupType: "full",
	})
	if err != nil {
		return nil, fmt.Errorf("full backup before removal failed: %w", err)
	}
	return backup, nil
}

func validateRemovalBackup(backup *BackupTaskResult) error {
	if backup == nil {
		return fmt.Errorf("full backup before removal returned empty result")
	}
	if !isSuccessfulTaskStatus(backup.Status) {
		return fmt.Errorf("full backup before removal did not complete: status=%s message=%s", backup.Status, backup.Message)
	}
	if strings.TrimSpace(backup.FilePath) == "" {
		return fmt.Errorf("full backup before removal completed without backup path")
	}
	if backup.FileSize <= 0 {
		return fmt.Errorf("full backup before removal completed with invalid file size: %d", backup.FileSize)
	}
	if strings.TrimSpace(backup.Checksum) == "" {
		return fmt.Errorf("full backup before removal completed without checksum")
	}
	return nil
}

func (s *InstanceService) decommissionRemoteInstance(ctx context.Context, id string) error {
	result, err := s.AdminAction(ctx, id, InstanceAdminRequest{Action: "decommission"})
	if err != nil {
		return err
	}
	if result == nil {
		return fmt.Errorf("decommission returned empty result")
	}
	if !isSuccessfulTaskStatus(result.Status) {
		return fmt.Errorf("decommission failed: %s", result.Message)
	}
	return nil
}

func (s *InstanceService) auditInstanceDelete(ctx context.Context, id, result, errorMsg, details string) {
	if s.auditSvc == nil {
		return
	}
	_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID:       userIDFromCtx(ctx),
		Operation:    "delete_instance",
		Action:       "delete",
		ResourceType: "instance",
		ResourceID:   id,
		Result:       result,
		ErrorMsg:     errorMsg,
		Details:      details,
	})
}

func (s *InstanceService) DetectVersion(ctx context.Context, id string) (*models.InstanceVersion, error) {
	instance, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.repo.GetConnection(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("instance connection not found: %w", err)
	}

	// P0: 之前返 error "agent not yet wired", 阻断所有升级路径.
	// 修: 调 agent POST /agent/tasks/version-detect, 真跑 SELECT @@version, @@version_comment.
	if s.agentClient == nil {
		return nil, fmt.Errorf("agent client not configured; cannot detect version for instance %s", id)
	}
	host, port := conn.Host, conn.Port
	if host == "" {
		return nil, fmt.Errorf("instance %s has no host/port to probe", id)
	}
	password, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt instance password: %w", err)
	}
	agentHost, agentPort, err := s.resolveAgentEndpoint(ctx, instance, conn)
	if err != nil {
		return nil, err
	}
	// PasswordEncrypted 是 AES-GCM 密文, agent 端自己解密再连 MySQL.
	targetHost := s.resolveAgentMySQLTarget(ctx, instance, conn, agentHost)
	cfg := map[string]interface{}{
		"target_host": targetHost,
		"target_port": port,
		"target_user": conn.Username,
		"target_pass": password,
	}
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/version-detect", map[string]interface{}{
		"task_id":     uuid.New().String(),
		"instance_id": id,
		"config":      cfg,
	})
	if err != nil {
		return nil, fmt.Errorf("version detect call failed: %w", err)
	}
	if result == nil {
		return nil, fmt.Errorf("version detect returned no result")
	}
	if isFailedTaskStatus(result.Status) {
		log.Printf("WARN: version detect failed for instance %s: %s", id, result.Message)
		versionRow := &models.InstanceVersion{
			InstanceID:  id,
			Flavor:      "unknown",
			Version:     "unknown",
			FullVersion: "unknown",
			IsLTS:       false,
			ReleaseDate: time.Now(),
			EOLDate:     time.Now().AddDate(3, 0, 0),
			Features:    result.Message,
			Engines:     "",
		}
		if err := s.repo.CreateVersion(ctx, versionRow); err != nil {
			return nil, fmt.Errorf("failed to create version: %w", err)
		}
		return versionRow, nil
	}
	// agent 返 message 形如 "8.0.36\tuuid\tMySQL Community Server"
	// 解析成 3 段, 失败则把整段当 full_version.
	parts := strings.SplitN(strings.TrimSpace(result.Message), "\t", 3)
	var fullVersion, version, versionComment string
	if len(parts) >= 1 {
		fullVersion = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		versionComment = strings.TrimSpace(parts[1])
	}
	// "8.0.36" → "8.0", "5.7.44" → "5.7"
	if idx := strings.LastIndex(fullVersion, "."); idx > 0 {
		version = fullVersion[:idx]
	} else {
		version = fullVersion
	}
	// @@version_comment 包含 "MySQL Community" / "MariaDB" 等用于判别 flavor.
	flavor := "mysql"
	if strings.Contains(strings.ToLower(versionComment), "mariadb") {
		flavor = "mariadb"
	} else if strings.Contains(strings.ToLower(versionComment), "percona") {
		flavor = "percona"
	}
	versionRow := &models.InstanceVersion{
		InstanceID:  id,
		Flavor:      flavor,
		Version:     version,
		FullVersion: fullVersion,
		IsLTS:       strings.HasPrefix(version, "8.0") || strings.HasPrefix(version, "5.7"),
		ReleaseDate: time.Now(),
		EOLDate:     time.Now().AddDate(3, 0, 0),
		Features:    "auto-detected from agent",
		Engines:     "innodb",
	}
	if err := s.repo.CreateVersion(ctx, versionRow); err != nil {
		return nil, fmt.Errorf("failed to create version: %w", err)
	}
	return versionRow, nil
}

func (s *InstanceService) Deploy(ctx context.Context, id string) (*DeployResult, error) {
	instance, err := s.repo.GetByID(ctx, id)
	if err != nil {
		s.auditDeploy(ctx, id, "", "failed", 0, "", fmt.Sprintf("instance not found: %v", err), nil)
		return nil, fmt.Errorf("instance not found: %w", err)
	}

	conn, err := s.repo.GetConnection(ctx, id)
	if err != nil {
		s.auditDeploy(ctx, id, "", "failed", 0, "", fmt.Sprintf("instance connection not found: %v", err), nil)
		return nil, fmt.Errorf("instance connection not found: %w", err)
	}

	task := &models.Task{
		TaskType:   "deploy",
		InstanceID: id,
		Status:     "pending",
		Progress:   0,
		CreatedAt:  time.Now(),
	}
	if err := s.taskRepo.Create(ctx, task); err != nil {
		s.auditDeploy(ctx, id, "", "failed", 0, "", fmt.Sprintf("failed to create task: %v", err), nil)
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	taskRepo := s.taskRepo
	taskRepo.UpdateStatus(ctx, task.ID, "running", 0)

	var agentHost string
	var agentPort int

	if instance.HostID != nil && *instance.HostID != "" {
		host, err := s.hostRepo.GetByID(ctx, *instance.HostID)
		if err == nil {
			agentHost = host.Address
			agentPort = host.AgentPort
		}
	}
	if agentHost == "" {
		agentHost = conn.Host
		agentPort = 9090
	}
	if s.agentClient == nil {
		taskRepo.UpdateStatus(ctx, task.ID, "failed", 0)
		msg := "Deploy failed: agent client not configured"
		s.auditDeploy(ctx, id, task.ID, "failed", 0, msg, "agent client not configured", map[string]interface{}{
			"agent_host": agentHost,
			"agent_port": agentPort,
		})
		return &DeployResult{
			TaskID:   task.ID,
			Status:   "failed",
			Progress: 0,
			Message:  msg,
		}, nil
	}

	mysqlPassword, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		taskRepo.UpdateStatus(ctx, task.ID, "failed", 0)
		resp := &DeployResult{
			TaskID:   task.ID,
			Status:   "failed",
			Progress: 0,
			Message:  fmt.Sprintf("Deploy failed: decrypt instance password: %v", err),
		}
		s.auditDeploy(ctx, id, task.ID, resp.Status, resp.Progress, resp.Message, err.Error(), map[string]interface{}{
			"agent_host": agentHost,
			"agent_port": agentPort,
		})
		return resp, nil
	}
	instance.Connection = *conn

	// Retry logic: if deployment fails due to existing data directory,
	// auto-increment port and try again (up to 3 times).
	maxRetries := 3
	currentPort := conn.Port
	var lastResult *AgentTaskResult
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Auto-increment port for retry
			currentPort = conn.Port + attempt
			conn.Port = currentPort
			instance.Connection = *conn
			taskRepo.UpdateStatus(ctx, task.ID, "running", 0)
		}

		result, err := s.agentClient.DeployInstance(ctx, agentHost, agentPort, instance, task.ID, mysqlPassword)
		lastResult = result
		lastErr = err

		if err == nil && result.Status == "completed" {
			break
		}

		// Check if the failure is due to existing data directory
		if err == nil && result.Status == "failed" &&
			(strings.Contains(result.Message, "Can't create directory") ||
				strings.Contains(result.Message, "already exists") ||
				strings.Contains(result.Message, "File exists")) {
			// Retry with different port
			continue
		}

		// Other failure - don't retry
		break
	}

	if lastErr != nil {
		taskRepo.UpdateStatus(ctx, task.ID, "failed", 0)
		resp := &DeployResult{
			TaskID:   task.ID,
			Status:   "failed",
			Progress: 0,
			Message:  fmt.Sprintf("Deploy failed: %v", lastErr),
		}
		s.auditDeploy(ctx, id, task.ID, resp.Status, resp.Progress, resp.Message, lastErr.Error(), map[string]interface{}{
			"agent_host": agentHost,
			"agent_port": agentPort,
		})
		return resp, nil
	}

	taskRepo.UpdateStatus(ctx, task.ID, lastResult.Status, lastResult.Progress)

	resp := &DeployResult{
		TaskID:   task.ID,
		Status:   lastResult.Status,
		Progress: lastResult.Progress,
		Message:  "MySQL instance deploy: " + lastResult.Message,
	}
	s.auditDeploy(ctx, id, task.ID, resp.Status, resp.Progress, resp.Message, "", map[string]interface{}{
		"agent_host": agentHost,
		"agent_port": agentPort,
	})
	return resp, nil
}

func (s *InstanceService) auditDeploy(ctx context.Context, instanceID, taskID, status string, progress int, message, errMsg string, details map[string]interface{}) {
	if s.auditSvc == nil {
		return
	}
	if details == nil {
		details = map[string]interface{}{}
	}
	details["instance_id"] = instanceID
	if taskID != "" {
		details["task_id"] = taskID
	}
	if status != "" {
		details["status"] = status
	}
	details["progress"] = progress
	if message != "" {
		details["message"] = message
	}
	detailBytes, _ := json.Marshal(details)
	resourceType := "instance"
	resourceID := instanceID
	if taskID != "" {
		resourceType = "instance_deploy_task"
		resourceID = taskID
	}
	_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID:       userIDFromCtx(ctx),
		Operation:    "deploy_instance",
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       "deploy",
		Details:      string(detailBytes),
		Result:       instanceDeployAuditResult(status, errMsg),
		ErrorMsg:     errMsg,
	})
}

func instanceDeployAuditResult(status, errMsg string) string {
	if errMsg != "" {
		return "failed"
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "timeout", "cancelled", "canceled":
		return "failed"
	default:
		return "success"
	}
}

func (s *InstanceService) HealthCheck(ctx context.Context, id string) (*InstanceAdminResult, error) {
	instance, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.repo.GetConnection(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("instance connection not found: %w", err)
	}
	agentHost, agentPort, err := s.resolveAgentEndpoint(ctx, instance, conn)
	if err != nil {
		return nil, err
	}
	targetHost := s.resolveAgentMySQLTarget(ctx, instance, conn, agentHost)
	if s.agentClient == nil {
		_ = s.updateInstanceHealthStatus(ctx, id, "unhealthy")
		return &InstanceAdminResult{
			TaskID:   "health-check-" + uuid.New().String(),
			Status:   "failed",
			Message:  "agent client not configured",
			Progress: 100,
		}, nil
	}
	password, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt instance password: %w", err)
	}
	result, err := s.agentClient.ExecuteMySQLHealthCheck(ctx, agentHost, agentPort, id, map[string]interface{}{
		"target_host": targetHost,
		"target_port": conn.Port,
		"target_user": conn.Username,
		"target_pass": password,
	})
	if err != nil {
		_ = s.updateInstanceHealthStatus(ctx, id, "unhealthy")
		return &InstanceAdminResult{
			TaskID:   "health-check-" + uuid.New().String(),
			Status:   "failed",
			Message:  err.Error(),
			Progress: 100,
		}, nil
	}
	if isFailedTaskStatus(result.Status) {
		_ = s.updateInstanceHealthStatus(ctx, id, "unhealthy")
	} else if isSuccessfulTaskStatus(result.Status) {
		_ = s.updateInstanceHealthStatus(ctx, id, "healthy")
	}
	return &InstanceAdminResult{
		TaskID:   result.TaskID,
		Status:   result.Status,
		Message:  result.Message,
		Data:     result.Data,
		Progress: result.Progress,
	}, nil
}

func (s *InstanceService) updateInstanceHealthStatus(ctx context.Context, instanceID, health string) error {
	status := &models.InstanceStatus{
		InstanceID:          instanceID,
		RunStatus:           "unknown",
		HealthStatus:        health,
		Role:                "",
		ReplicationStatus:   "",
		SecondsBehindMaster: -1,
	}
	if existing, err := s.repo.GetStatus(ctx, instanceID); err == nil && existing != nil {
		status = existing
		status.HealthStatus = health
	}
	return s.repo.UpsertStatus(ctx, instanceID, status)
}

func isFailedTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "unhealthy", "timeout", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func isSuccessfulTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "succeeded", "healthy", "ok", "completed":
		return true
	default:
		return false
	}
}

type CreateInstanceRequest struct {
	Name       string `json:"name" binding:"required"`
	ClusterID  string `json:"cluster_id"`
	HostID     string `json:"host_id"`
	Host       string `json:"host" binding:"required"`
	Port       int    `json:"port" binding:"required"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	SSLEnabled bool   `json:"ssl_enabled"`
	// Version-agnostic install fields. Optional at create time; if absent the
	// install/upgrade can still be triggered later via the deploy endpoint
	// after a version is selected from /api/v1/versions.
	VersionID  string `json:"version_id"`
	PackageURL string `json:"package_url"`
	Basedir    string `json:"basedir"`
	Datadir    string `json:"datadir"`
	OSUser     string `json:"os_user"`
}

type BatchCreateInstanceRequest struct {
	Instances []CreateInstanceRequest `json:"instances" binding:"required"`
}

type BatchCreateInstanceResult struct {
	Total   int                      `json:"total"`
	Created int                      `json:"created"`
	Rows    []BatchCreateInstanceRow `json:"rows"`
}

type BatchCreateInstanceRow struct {
	Index    int              `json:"index"`
	Name     string           `json:"name"`
	Host     string           `json:"host"`
	Port     int              `json:"port"`
	Status   string           `json:"status"`
	Message  string           `json:"message,omitempty"`
	Instance *models.Instance `json:"instance,omitempty"`
}

type UpdateInstanceRequest struct {
	Name       string `json:"name"`
	ClusterID  string `json:"cluster_id"`
	HostID     string `json:"host_id"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	SSLEnabled *bool  `json:"ssl_enabled"`
	VersionID  string `json:"version_id"`
	PackageURL string `json:"package_url"`
	Basedir    string `json:"basedir"`
	Datadir    string `json:"datadir"`
	OSUser     string `json:"os_user"`
}

func (r UpdateInstanceRequest) HasConnectionUpdate() bool {
	return r.Host != "" ||
		r.Port != 0 ||
		r.Username != "" ||
		r.Password != "" ||
		r.SSLEnabled != nil ||
		r.VersionID != "" ||
		r.PackageURL != "" ||
		r.Basedir != "" ||
		r.Datadir != "" ||
		r.OSUser != ""
}

type DeployResult struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Message  string `json:"message"`
}

func (s *InstanceService) BatchCreate(ctx context.Context, req BatchCreateInstanceRequest) (*BatchCreateInstanceResult, error) {
	result := &BatchCreateInstanceResult{
		Total: len(req.Instances),
		Rows:  make([]BatchCreateInstanceRow, 0, len(req.Instances)),
	}
	for i, item := range req.Instances {
		row := BatchCreateInstanceRow{
			Index: i + 1,
			Name:  item.Name,
			Host:  item.Host,
			Port:  item.Port,
		}
		instance, err := s.Create(ctx, item)
		if err != nil {
			row.Status = "failed"
			row.Message = err.Error()
		} else {
			row.Status = "created"
			row.Instance = instance
			result.Created++
		}
		result.Rows = append(result.Rows, row)
	}
	return result, nil
}

type InstanceAdminRequest struct {
	Action               string `json:"action" binding:"required"`
	Username             string `json:"username"`
	UserHost             string `json:"user_host"`
	Password             string `json:"password"`
	Privileges           string `json:"privileges"`
	Scope                string `json:"scope"`
	Pattern              string `json:"pattern"`
	Name                 string `json:"name"`
	Value                string `json:"value"`
	Path                 string `json:"path"`
	Content              string `json:"content"`
	Service              string `json:"service"`
	Verb                 string `json:"verb"`
	UpdateStoredPassword bool   `json:"update_stored_password"`
}

type BatchPasswordRequest struct {
	Host            string `json:"host" binding:"required"`
	Ports           []int  `json:"ports" binding:"required"`
	Username        string `json:"username" binding:"required"`
	UserHost        string `json:"user_host"`
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password" binding:"required"`
	UpdateStored    bool   `json:"update_stored"`
}

type InstanceAdminResult struct {
	TaskID   string      `json:"task_id"`
	Status   string      `json:"status"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data,omitempty"`
	Progress int         `json:"progress"`
}

func failedInstanceAdminResult(message string) *InstanceAdminResult {
	return &InstanceAdminResult{
		TaskID:   "instance-admin-" + uuid.New().String(),
		Status:   "failed",
		Message:  message,
		Progress: 100,
	}
}

func validateInstanceAdminMetadata(conn *models.InstanceConnection, req InstanceAdminRequest) error {
	if conn == nil {
		return fmt.Errorf("instance connection metadata is required")
	}
	if strings.TrimSpace(req.Action) != "service_control" {
		return nil
	}
	verb := strings.ToLower(strings.TrimSpace(req.Verb))
	if verb == "" {
		verb = "status"
	}
	if strings.TrimSpace(conn.Datadir) == "" {
		return fmt.Errorf("service control requires instance datadir metadata")
	}
	if verb == "start" || verb == "restart" {
		if strings.TrimSpace(conn.Basedir) == "" {
			return fmt.Errorf("service control %s requires instance basedir metadata", verb)
		}
		if conn.Port <= 0 {
			return fmt.Errorf("service control %s requires instance port metadata", verb)
		}
	}
	return nil
}

func (s *InstanceService) AdminAction(ctx context.Context, id string, req InstanceAdminRequest) (*InstanceAdminResult, error) {
	instance, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	conn, err := s.repo.GetConnection(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("instance connection not found: %w", err)
	}
	if err := validateInstanceAdminMetadata(conn, req); err != nil {
		return failedInstanceAdminResult(err.Error()), nil
	}
	if req.Action == "change_password" {
		if err := utils.ValidatePasswordComplexity(req.Password); err != nil {
			return failedInstanceAdminResult(err.Error()), nil
		}
	}

	agentHost, agentPort, err := s.resolveAgentEndpoint(ctx, instance, conn)
	if err != nil {
		return nil, err
	}
	targetHost := s.resolveAgentMySQLTarget(ctx, instance, conn, agentHost)

	if s.agentClient == nil {
		return failedInstanceAdminResult("agent client not configured"), nil
	}

	// For change_password, try multiple password candidates to handle stored/actual mismatch
	if req.Action == "change_password" {
		candidates := s.passwordCandidates(conn, "")
		var lastResult *InstanceAdminResult
		var lastErr error
		for _, candidate := range candidates {
			original := conn.PasswordEncrypted
			encrypted, encErr := utils.Encrypt(candidate, s.encKey)
			if encErr != nil {
				conn.PasswordEncrypted = original
				continue
			}
			conn.PasswordEncrypted = encrypted
			result, adminErr := s.adminActionWithConnection(ctx, instance, conn, req)
			conn.PasswordEncrypted = original
			if adminErr == nil && result.Status == "completed" {
				lastResult = result
				lastErr = nil
				break
			}
			lastResult = result
			lastErr = adminErr
		}
		if lastErr != nil {
			return failedInstanceAdminResult("instance admin call failed: " + lastErr.Error()), nil
		}
		if lastResult == nil || lastResult.Status != "completed" {
			return failedInstanceAdminResult("failed to connect to MySQL with any known password"), nil
		}
		if req.UpdateStoredPassword && req.Username == conn.Username && req.Password != "" {
			enc, encErr := utils.Encrypt(req.Password, s.encKey)
			if encErr == nil {
				_ = s.repo.UpdateConnectionPassword(ctx, id, enc)
			}
		}
		return lastResult, nil
	}

	password, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt instance password: %w", err)
	}
	configPath := resolveInstanceConfigPath(conn, req.Path)
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/instance-admin", map[string]interface{}{
		"task_id":     "instance-admin-" + uuid.New().String(),
		"instance_id": id,
		"config": map[string]interface{}{
			"action":      req.Action,
			"target_host": targetHost,
			"target_port": conn.Port,
			"target_user": conn.Username,
			"target_pass": password,
			"username":    req.Username,
			"user_host":   req.UserHost,
			"password":    req.Password,
			"privileges":  req.Privileges,
			"scope":       req.Scope,
			"pattern":     req.Pattern,
			"name":        req.Name,
			"value":       req.Value,
			"path":        configPath,
			"content":     req.Content,
			"service":     req.Service,
			"verb":        req.Verb,
			"basedir":     conn.Basedir,
			"datadir":     conn.Datadir,
			"os_user":     conn.OSUser,
		},
	})
	if err != nil {
		return failedInstanceAdminResult("instance admin call failed: " + err.Error()), nil
	}
	if result.Status == "completed" && req.Action == "change_password" && req.UpdateStoredPassword {
		if req.Username == conn.Username && req.Password != "" {
			enc, encErr := utils.Encrypt(req.Password, s.encKey)
			if encErr != nil {
				return nil, fmt.Errorf("failed to encrypt updated password: %w", encErr)
			}
			if updateErr := s.repo.UpdateConnectionPassword(ctx, id, enc); updateErr != nil {
				return nil, updateErr
			}
		}
	}
	return &InstanceAdminResult{
		TaskID:   result.TaskID,
		Status:   result.Status,
		Message:  result.Message,
		Data:     result.Data,
		Progress: result.Progress,
	}, nil
}

func resolveInstanceConfigPath(conn *models.InstanceConnection, requested string) string {
	path := strings.TrimSpace(requested)
	if path != "" && filepath.ToSlash(filepath.Clean(path)) != "/etc/my.cnf" {
		return path
	}
	if conn == nil {
		return defaultString(path, "/etc/my.cnf")
	}

	// PXC: managed config at /etc/dbops-pxc/dbops-pxc-{port}.cnf
	if strings.Contains(conn.Datadir, "/pxc-") && conn.Port > 0 {
		return fmt.Sprintf("/etc/dbops-pxc/dbops-pxc-%d.cnf", conn.Port)
	}

	// MHA: standard config at /etc/mha/app1.cnf
	// Detect via datadir pattern /mha- or /mha (e.g. /data/mysql/mha-3307)
	if strings.Contains(filepath.ToSlash(conn.Datadir), "/mha") {
		return "/etc/mha/app1.cnf"
	}

	// MGR: check datadir for mgr-related patterns, or fallback
	if strings.Contains(filepath.ToSlash(conn.Datadir), "/mgr-") ||
		strings.Contains(filepath.ToSlash(conn.Datadir), "/group_repl") {
		return "/etc/my.cnf"
	}

	// HA / standard: return default path or datadir/my.cnf
	if path == "" {
		return "/etc/my.cnf"
	}
	return path
}

func (s *InstanceService) BatchUpdatePassword(ctx context.Context, req BatchPasswordRequest) (*InstanceAdminResult, error) {
	if err := utils.ValidatePasswordComplexity(req.NewPassword); err != nil {
		return failedInstanceAdminResult(err.Error()), nil
	}
	if req.UserHost == "" {
		req.UserHost = "%"
	}
	instances, err := s.repo.List(ctx, 1000, 0)
	if err != nil {
		return nil, err
	}
	portSet := map[int]bool{}
	for _, port := range req.Ports {
		portSet[port] = true
	}
	results := make([]map[string]interface{}, 0)
	successCount := 0
	for _, instance := range instances {
		conn, connErr := s.repo.GetConnection(ctx, instance.ID)
		if connErr != nil || conn.Host != req.Host || !portSet[conn.Port] {
			continue
		}
		candidates := s.passwordCandidates(conn, req.CurrentPassword)
		var lastErr error
		var actionResult *InstanceAdminResult
		for _, candidate := range candidates {
			original := conn.PasswordEncrypted
			encrypted, encErr := utils.Encrypt(candidate, s.encKey)
			if encErr != nil {
				conn.PasswordEncrypted = original
				continue
			}
			conn.PasswordEncrypted = encrypted
			actionResult, lastErr = s.adminActionWithConnection(ctx, &instance, conn, InstanceAdminRequest{
				Action:               "change_password",
				Username:             req.Username,
				UserHost:             req.UserHost,
				Password:             req.NewPassword,
				UpdateStoredPassword: false,
			})
			conn.PasswordEncrypted = original
			if lastErr == nil && actionResult != nil && actionResult.Status == "completed" {
				break
			}
		}
		row := map[string]interface{}{
			"instance_id": instance.ID,
			"name":        instance.Name,
			"host":        conn.Host,
			"port":        conn.Port,
		}
		if actionResult != nil && actionResult.Status == "completed" {
			successCount++
			row["status"] = "completed"
			if req.UpdateStored && req.Username == conn.Username {
				enc, encErr := utils.Encrypt(req.NewPassword, s.encKey)
				if encErr == nil {
					_ = s.repo.UpdateConnectionPassword(ctx, instance.ID, enc)
				}
			}
		} else {
			row["status"] = "failed"
			if lastErr != nil {
				row["message"] = lastErr.Error()
			} else if actionResult != nil {
				row["message"] = actionResult.Message
			}
		}
		results = append(results, row)
	}
	status := "completed"
	if len(results) == 0 || successCount != len(results) {
		status = "failed"
	}
	return &InstanceAdminResult{
		TaskID:   "batch-password-" + uuid.New().String(),
		Status:   status,
		Progress: 100,
		Message:  fmt.Sprintf("matched %d instances, updated %d", len(results), successCount),
		Data:     map[string]interface{}{"rows": results},
	}, nil
}

func (s *InstanceService) adminActionWithConnection(ctx context.Context, instance *models.Instance, conn *models.InstanceConnection, req InstanceAdminRequest) (*InstanceAdminResult, error) {
	if err := validateInstanceAdminMetadata(conn, req); err != nil {
		return failedInstanceAdminResult(err.Error()), nil
	}
	password, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt instance password: %w", err)
	}
	agentHost, agentPort, err := s.resolveAgentEndpoint(ctx, instance, conn)
	if err != nil {
		return nil, err
	}
	targetHost := s.resolveAgentMySQLTarget(ctx, instance, conn, agentHost)
	if s.agentClient == nil {
		return nil, fmt.Errorf("agent client not configured")
	}
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/instance-admin", map[string]interface{}{
		"task_id":     "instance-admin-" + uuid.New().String(),
		"instance_id": instance.ID,
		"config": map[string]interface{}{
			"action":      req.Action,
			"target_host": targetHost,
			"target_port": conn.Port,
			"target_user": conn.Username,
			"target_pass": password,
			"username":    req.Username,
			"user_host":   req.UserHost,
			"password":    req.Password,
			"privileges":  req.Privileges,
			"scope":       req.Scope,
			"pattern":     req.Pattern,
			"name":        req.Name,
			"value":       req.Value,
			"path":        req.Path,
			"content":     req.Content,
			"service":     req.Service,
			"verb":        req.Verb,
			"basedir":     conn.Basedir,
			"datadir":     conn.Datadir,
			"os_user":     conn.OSUser,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("instance admin call failed: %w", err)
	}
	return &InstanceAdminResult{
		TaskID:   result.TaskID,
		Status:   result.Status,
		Message:  result.Message,
		Data:     result.Data,
		Progress: result.Progress,
	}, nil
}

// UpdateInstanceStatusRequest 用于更新实例的角色、运行状态和复制状态。
// 通过 PUT /api/v1/instances/:id/status 使用，无需经过 Agent。
type UpdateInstanceStatusRequest struct {
	Role                string `json:"role"`
	RunStatus           string `json:"run_status"`
	HealthStatus        string `json:"health_status"`
	ReplicationStatus   string `json:"replication_status"`
	SecondsBehindMaster int    `json:"seconds_behind_master"`
	MasterID            string `json:"master_id"`
	SlaveIDs            string `json:"slave_ids"`
}

func (s *InstanceService) hostKeyCallback(hostAddr string) ssh.HostKeyCallback {
	dataDir := "./data"
	if s.hostRepo != nil {
		// 尝试从已知的主机信息推断data目录
	}
	knownHostsPath := filepath.Join(dataDir, "known_hosts")
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		log.Printf("WARN: cannot load known_hosts at %s, falling back to key recording: %v", knownHostsPath, err)
		return s.hostKeyRecorder(knownHostsPath, hostAddr)
	}
	return callback
}

func (s *InstanceService) hostKeyRecorder(knownHostsPath, hostAddr string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if data, err := os.ReadFile(knownHostsPath); err == nil && len(data) > 0 {
			if cb, cbErr := knownhosts.New(knownHostsPath); cbErr == nil {
				if verifyErr := cb(hostname, remote, key); verifyErr == nil {
					return nil
				}
				return fmt.Errorf("host key verification failed for %s: host key has changed! Possible MITM attack", hostAddr)
			}
		}
		log.Printf("WARN: First SSH connection to %s — recording host key to %s", hostAddr, knownHostsPath)
		_ = os.MkdirAll(filepath.Dir(knownHostsPath), 0o755)
		f, err := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("cannot write known_hosts: %w", err)
		}
		defer f.Close()
		line := knownhosts.Line([]string{knownhosts.Normalize(hostAddr)}, key)
		if _, err := fmt.Fprintln(f, line); err != nil {
			return fmt.Errorf("write known_hosts: %w", err)
		}
		return nil
	}
}

func (s *InstanceService) UpdateInstanceStatus(ctx context.Context, id string, req UpdateInstanceStatusRequest) (*models.Instance, error) {
	instance, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("instance not found: %w", err)
	}
	status := &models.InstanceStatus{
		InstanceID:          id,
		RunStatus:           defaultString(req.RunStatus, defaultString(instance.Status.RunStatus, "running")),
		HealthStatus:        defaultString(req.HealthStatus, defaultString(instance.Status.HealthStatus, "healthy")),
		Role:                req.Role,
		ReplicationStatus:   req.ReplicationStatus,
		SecondsBehindMaster: req.SecondsBehindMaster,
	}
	if err := s.repo.UpsertStatus(ctx, id, status); err != nil {
		return nil, fmt.Errorf("failed to update instance status: %w", err)
	}
	if req.MasterID != "" || req.SlaveIDs != "" {
		topo := &models.InstanceTopology{
			InstanceID:      id,
			ClusterID:       instance.ClusterID,
			MasterID:        req.MasterID,
			SlaveIDs:        req.SlaveIDs,
			ReplicationMode: req.ReplicationStatus,
		}
		if err := s.repo.UpsertTopology(ctx, id, topo); err != nil {
			return nil, fmt.Errorf("failed to update instance topology: %w", err)
		}
	}
	if s.auditSvc != nil {
		_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
			UserID:       userIDFromCtx(ctx),
			Action:       "update_status",
			ResourceType: "instance",
			ResourceID:   id,
			Details:      fmt.Sprintf("role=%s replication=%s", req.Role, req.ReplicationStatus),
			Result:       "success",
		})
	}
	return s.repo.GetByID(ctx, id)
}

type ForceResetPasswordRequest struct {
	Username    string `json:"username"`
	UserHost    string `json:"user_host"`
	NewPassword string `json:"new_password"`
}

func (s *InstanceService) ForceResetInstancePassword(ctx context.Context, id string, req ForceResetPasswordRequest) error {
	if strings.TrimSpace(req.Username) == "" {
		req.Username = "root"
	}
	if strings.TrimSpace(req.UserHost) == "" {
		req.UserHost = "%"
	}
	if strings.TrimSpace(req.NewPassword) == "" {
		return fmt.Errorf("new password is required")
	}
	if err := utils.ValidatePasswordComplexity(req.NewPassword); err != nil {
		return err
	}
	instance, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("instance not found: %w", err)
	}
	conn, err := s.repo.GetConnection(ctx, id)
	if err != nil {
		return fmt.Errorf("instance connection not found: %w", err)
	}
	log.Printf("ForceChangePassword [%s]: changing password online for %s@%s", id, req.Username, req.UserHost)
	candidates := s.passwordCandidates(conn, "")
	var lastResult *InstanceAdminResult
	var lastErr error
	for _, candidate := range candidates {
		original := conn.PasswordEncrypted
		encrypted, encErr := utils.Encrypt(candidate, s.encKey)
		if encErr != nil {
			conn.PasswordEncrypted = original
			continue
		}
		conn.PasswordEncrypted = encrypted
		lastResult, lastErr = s.adminActionWithConnection(ctx, instance, conn, InstanceAdminRequest{
			Action:               "change_password",
			Username:             req.Username,
			UserHost:             req.UserHost,
			Password:             req.NewPassword,
			UpdateStoredPassword: false,
		})
		conn.PasswordEncrypted = original
		if lastErr == nil && lastResult != nil && lastResult.Status == "completed" {
			break
		}
	}
	if lastErr != nil {
		return fmt.Errorf("instance admin call failed: %w", lastErr)
	}
	if lastResult == nil || lastResult.Status != "completed" {
		if lastResult != nil && strings.TrimSpace(lastResult.Message) != "" {
			return fmt.Errorf("failed to change password online: %s", lastResult.Message)
		}
		return fmt.Errorf("failed to connect to MySQL with any known password")
	}
	if req.Username == conn.Username {
		enc, encErr := utils.Encrypt(req.NewPassword, s.encKey)
		if encErr != nil {
			return fmt.Errorf("failed to encrypt new password: %w", encErr)
		}
		if updateErr := s.repo.UpdateConnectionPassword(ctx, id, enc); updateErr != nil {
			return fmt.Errorf("failed to update stored password: %w", updateErr)
		}
	}
	log.Printf("ForceChangePassword [%s]: completed successfully", id)
	return nil
}

func sshClient(host *models.Host, credential string) (*ssh.Client, error) {
	auth := []ssh.AuthMethod{ssh.Password(credential)}
	if signer, err := ssh.ParsePrivateKey([]byte(credential)); err == nil {
		auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}
	dataDir := "./data"
	knownHostsPath := filepath.Join(dataDir, "known_hosts")
	knownHostsCB, err := knownhosts.New(knownHostsPath)
	var hostKeyCallback ssh.HostKeyCallback
	if err != nil {
		hostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if data, readErr := os.ReadFile(knownHostsPath); readErr == nil && len(data) > 0 {
				if cb, cbErr := knownhosts.New(knownHostsPath); cbErr == nil {
					if verifyErr := cb(hostname, remote, key); verifyErr == nil {
						return nil
					}
					return fmt.Errorf("host key verification failed for %s: host key has changed! Possible MITM attack", host.Address)
				}
			}
			log.Printf("WARN: First SSH connection to %s — recording host key to %s", host.Address, knownHostsPath)
			_ = os.MkdirAll(filepath.Dir(knownHostsPath), 0o755)
			f, openErr := os.OpenFile(knownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if openErr != nil {
				return fmt.Errorf("cannot write known_hosts: %w", openErr)
			}
			defer f.Close()
			line := knownhosts.Line([]string{knownhosts.Normalize(host.Address)}, key)
			if _, writeErr := fmt.Fprintln(f, line); writeErr != nil {
				return fmt.Errorf("write known_hosts: %w", writeErr)
			}
			return nil
		}
	} else {
		hostKeyCallback = knownHostsCB
	}
	config := &ssh.ClientConfig{
		User:            host.SSHUser,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", net.JoinHostPort(host.Address, strconv.Itoa(host.SSHPort)), config)
}

func runSSHCommand(client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	var out bytes.Buffer
	session.Stdout = &out
	session.Stderr = &out
	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()
	select {
	case err := <-done:
		return out.String(), err
	case <-time.After(60 * time.Second):
		_ = session.Close()
		return out.String(), fmt.Errorf("ssh command timed out")
	}
}

func escapeSQLValue(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "'", "''")
	s = strings.ReplaceAll(s, "\x00", "")
	return s
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func firstRemotePath(client *ssh.Client, paths []string) string {
	for _, p := range paths {
		if strings.TrimSpace(p) == "" || strings.Contains(p, "//") {
			continue
		}
		out, err := runSSHCommand(client, fmt.Sprintf("test -f %s && echo OK", shellQuote(p)))
		if err == nil && strings.TrimSpace(out) == "OK" {
			return p
		}
	}
	return ""
}

func defaultMySQLPasswordForConnection(conn *models.InstanceConnection) string {
	return ""
}

func (s *InstanceService) passwordCandidates(conn *models.InstanceConnection, explicit string) []string {
	seen := map[string]bool{}
	add := func(out *[]string, value string) {
		if !seen[value] {
			seen[value] = true
			*out = append(*out, value)
		}
	}
	out := make([]string, 0, 4)
	if explicit != "" {
		add(&out, explicit)
	}
	if stored, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey); err == nil {
		add(&out, stored)
	}
	return out
}

// GetInstanceCredentials returns the decrypted MySQL password for an instance (admin only).
func (s *InstanceService) GetInstanceCredentials(ctx context.Context, id string) (*InstanceCredentials, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, fmt.Errorf("instance not found: %w", err)
	}
	conn, err := s.repo.GetConnection(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("instance connection not found: %w", err)
	}
	password, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		log.Printf("WARN: failed to decrypt password for instance %s, returning empty password: %v", id, err)
		password = ""
	}
	if s.auditSvc != nil {
		_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
			UserID:       userIDFromCtx(ctx),
			Action:       "view_credentials",
			ResourceType: "instance",
			ResourceID:   id,
			Details:      "viewed instance credentials (password)",
			Result:       "success",
		})
	}
	return &InstanceCredentials{
		InstanceID: id,
		Username:   conn.Username,
		Password:   password,
		Host:       conn.Host,
		Port:       conn.Port,
	}, nil
}

type InstanceCredentials struct {
	InstanceID string `json:"instance_id"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
}

func (s *InstanceService) resolveAgentEndpoint(ctx context.Context, instance *models.Instance, conn *models.InstanceConnection) (string, int, error) {
	if instance.HostID != nil && *instance.HostID != "" {
		host, err := s.hostRepo.GetByID(ctx, *instance.HostID)
		if err == nil && host.Address != "" {
			port := host.AgentPort
			if port == 0 {
				port = 9090
			}
			return host.Address, port, nil
		}
	}
	if conn.Host == "" {
		return "", 0, fmt.Errorf("cannot determine agent host")
	}
	return conn.Host, 9090, nil
}

func (s *InstanceService) resolveAgentMySQLTarget(ctx context.Context, instance *models.Instance, conn *models.InstanceConnection, agentHost string) string {
	if conn == nil || strings.TrimSpace(conn.Host) == "" {
		return "127.0.0.1"
	}
	if s.hostRepo != nil && instance != nil && instance.HostID != nil && *instance.HostID != "" {
		if host, err := s.hostRepo.GetByID(ctx, *instance.HostID); err == nil && host != nil {
			if sameHost(host.Address, conn.Host) && sameHost(host.Address, agentHost) {
				return "127.0.0.1"
			}
		}
	}
	return conn.Host
}
