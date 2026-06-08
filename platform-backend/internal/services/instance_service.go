package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type InstanceService struct {
	repo        *repositories.InstanceRepository
	hostRepo    *repositories.HostRepository
	taskRepo    *repositories.TaskRepository
	agentClient *AgentClient
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

func (s *InstanceService) Create(ctx context.Context, req CreateInstanceRequest) (*models.Instance, error) {
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
	return s.repo.List(ctx, limit, offset)
}

func (s *InstanceService) ListByHostID(ctx context.Context, hostID string, limit, offset int) ([]models.Instance, error) {
	return s.repo.ListByHostID(ctx, hostID, limit, offset)
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
	if s.auditSvc != nil {
		_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
			UserID:       userIDFromCtx(ctx),
			Action:       "update",
			ResourceType: "instance",
			ResourceID:   id,
			Result:       "success",
		})
	}

	return instance, nil
}

func (s *InstanceService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	// P0: 写 audit, 之前删 instance 0 记录, SOC2 不通过.
	if s.auditSvc != nil {
		_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
			UserID:       userIDFromCtx(ctx),
			Action:       "delete",
			ResourceType: "instance",
			ResourceID:   id,
			Result:       "success",
		})
	}
	return nil
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
	cfg := map[string]interface{}{
		"target_host": host,
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
		return nil, fmt.Errorf("instance not found: %w", err)
	}

	conn, err := s.repo.GetConnection(ctx, id)
	if err != nil {
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

	result, err := s.agentClient.DeployInstance(ctx, agentHost, agentPort, instance, task.ID)
	if err != nil {
		taskRepo.UpdateStatus(ctx, task.ID, "failed", 0)
		return &DeployResult{
			TaskID:   task.ID,
			Status:   "failed",
			Progress: 0,
			Message:  fmt.Sprintf("Deploy failed: %v", err),
		}, nil
	}

	taskRepo.UpdateStatus(ctx, task.ID, result.Status, result.Progress)

	return &DeployResult{
		TaskID:   task.ID,
		Status:   result.Status,
		Progress: result.Progress,
		Message:  result.Message,
	}, nil
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

type UpdateInstanceRequest struct {
	Name      string `json:"name"`
	ClusterID string `json:"cluster_id"`
	HostID    string `json:"host_id"`
}

type DeployResult struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Message  string `json:"message"`
}

type InstanceAdminRequest struct {
	Action     string `json:"action" binding:"required"`
	Username   string `json:"username"`
	UserHost   string `json:"user_host"`
	Password   string `json:"password"`
	Privileges string `json:"privileges"`
	Scope      string `json:"scope"`
	Pattern    string `json:"pattern"`
	Name       string `json:"name"`
	Value      string `json:"value"`
	Path       string `json:"path"`
	Content    string `json:"content"`
	Service    string `json:"service"`
	Verb       string `json:"verb"`
}

type InstanceAdminResult struct {
	TaskID   string      `json:"task_id"`
	Status   string      `json:"status"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data,omitempty"`
	Progress int         `json:"progress"`
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
	password, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt instance password: %w", err)
	}
	agentHost, agentPort, err := s.resolveAgentEndpoint(ctx, instance, conn)
	if err != nil {
		return nil, err
	}

	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/instance-admin", map[string]interface{}{
		"task_id":     "instance-admin-" + uuid.New().String(),
		"instance_id": id,
		"config": map[string]interface{}{
			"action":      req.Action,
			"target_host": conn.Host,
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
