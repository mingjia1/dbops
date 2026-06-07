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
	encKey      string
}

func NewInstanceService(repo *repositories.InstanceRepository, hostRepo *repositories.HostRepository, taskRepo *repositories.TaskRepository, agentClient *AgentClient, encKey string) *InstanceService {
	return &InstanceService{
		repo:        repo,
		hostRepo:    hostRepo,
		taskRepo:    taskRepo,
		agentClient: agentClient,
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

	return instance, nil
}

func (s *InstanceService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *InstanceService) DetectVersion(ctx context.Context, id string) (*models.InstanceVersion, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
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
	// PasswordEncrypted 是 AES-GCM 密文, agent 端自己解密再连 MySQL.
	cfg := map[string]interface{}{
		"target_host": host,
		"target_port": port,
		"target_user": conn.Username,
		"target_pass": conn.PasswordEncrypted,
	}
	result, err := s.agentClient.callAgent(ctx, host, 9090, "/agent/tasks/version-detect", map[string]interface{}{
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
