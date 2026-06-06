package services

import (
	"context"
	"fmt"
	"time"

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

	// P1-5: 之前直接写死 Flavor=mysql Version=8.0 FullVersion=8.0.36 落库,
	// 任何 5.6 / 5.7 / 8.4 / MariaDB 都被标记成 8.0.36,
	// 后续 PlanUpgradePath 用这个假版本做"升级路径校验"全部 OK, 真实升级时炸.
	// 修: 走 agent 真探测 (SELECT @@version + @@version_comment).
	// agent 端没对应路由时返 error, 业务层按需处理.
	if s.agentClient == nil {
		return nil, fmt.Errorf("agent client not configured; cannot detect version for instance %s", id)
	}
	hostAddr := conn.Host
	port := conn.Port
	if hostAddr == "" {
		return nil, fmt.Errorf("instance %s has no host/port to probe", id)
	}
	// Agent 端暂无 /version-detect 专用路由, 这里用一个标记字段让 agent 端知道
	// (后续可加 tasks.POST("/version-detect")). 现阶段返明确失败.
	return nil, fmt.Errorf("version detection requires agent support (probe %s:%d) which is not yet wired; manual entry needed", hostAddr, port)
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
