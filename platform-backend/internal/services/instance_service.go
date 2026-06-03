package services

import (
	"context"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type InstanceService struct {
	repo        *repositories.InstanceRepository
	hostRepo    *repositories.HostRepository
	taskRepo    *repositories.TaskRepository
	agentClient *AgentClient
}

func NewInstanceService(repo *repositories.InstanceRepository, hostRepo *repositories.HostRepository, taskRepo *repositories.TaskRepository, agentClient *AgentClient) *InstanceService {
	return &InstanceService{
		repo:        repo,
		hostRepo:    hostRepo,
		taskRepo:    taskRepo,
		agentClient: agentClient,
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
		InstanceID:       instance.ID,
		Host:             req.Host,
		Port:             req.Port,
		Username:         req.Username,
		PasswordEncrypted: req.Password,
		SSLEnabled:       req.SSLEnabled,
	}

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
	_, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	version := &models.InstanceVersion{
		InstanceID:  id,
		Flavor:      "mysql",
		Version:     "8.0",
		FullVersion: "8.0.36",
		IsLTS:       true,
		ReleaseDate: time.Now(),
		EOLDate:     time.Now().AddDate(5, 0, 0),
		Features:    "gtid,mgr,window_functions,cte",
		Engines:     "innodb,myisam",
	}

	if err := s.repo.CreateVersion(ctx, version); err != nil {
		return nil, fmt.Errorf("failed to create version: %w", err)
	}

	return version, nil
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
