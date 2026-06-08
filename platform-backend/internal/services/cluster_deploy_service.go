package services

import (
	"context"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/config"
)

type ClusterDeployService struct {
	repo        *repositories.ClusterDeployRepository
	hostRepo    *repositories.HostRepository
	instRepo    *repositories.InstanceRepository
	agentClient *AgentClient
	defaults    config.ClusterDefaults
}

func NewClusterDeployService(
	repo *repositories.ClusterDeployRepository,
	hostRepo *repositories.HostRepository,
	instRepo *repositories.InstanceRepository,
	agentClient *AgentClient,
	defaults config.ClusterDefaults,
) *ClusterDeployService {
	return &ClusterDeployService{
		repo:        repo,
		hostRepo:    hostRepo,
		instRepo:    instRepo,
		agentClient: agentClient,
		defaults:    defaults,
	}
}

func (s *ClusterDeployService) DeployMHA(ctx context.Context, req DeployMHARequest) (*DeployResponse, error) {
	deployment := &models.ClusterDeployment{
		ClusterType: "mha",
		Name:        req.Name,
		Status:      "pending",
	}
	if err := s.repo.Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	config := map[string]interface{}{
		"deploy_mode":    "mha",
		"manager_host":   req.ManagerHost,
		"master_host":    req.MasterHost,
		"master_port":    req.MasterPort,
		"vip":            req.VIP,
		"repl_user":      s.defaults.ReplicationUser,
		"repl_pass":      s.defaults.ReplicationPass,
		"ssh_user":       s.defaults.SSHUser,
		"mysql_user":     req.MySQLUser,
		"mysql_password": req.MySQLPassword,
		"ping_interval":  3,
		"ping_retry":     3,
	}

	var slaveHosts []string
	var slavePorts []int
	for _, slave := range req.SlaveHosts {
		slaveHosts = append(slaveHosts, slave.Host)
		slavePorts = append(slavePorts, slave.Port)
	}
	config["slave_hosts"] = slaveHosts
	config["slave_ports"] = slavePorts

	// B6: 删 hostRepo.GetByID(ctx, "") 重复两次的死代码.
	// 真实 agent host 走 req.ManagerHost, 不再尝试从 hostRepo 反查空字符串.
	agentHost := req.ManagerHost
	agentPort := 9090

	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/deploy", map[string]interface{}{
		"task_id":     deployment.ID,
		"instance_id": "",
		"config":      config,
	})
	if err != nil {
		s.repo.UpdateStatus(ctx, deployment.ID, "failed")
		return &DeployResponse{
			DeploymentID: deployment.ID,
			ClusterType:  "mha",
			Name:         req.Name,
			Status:       "failed",
			Message:      fmt.Sprintf("Deploy failed: %v", err),
			CreatedAt:    deployment.CreatedAt,
		}, nil
	}

	status := result.Status
	s.repo.UpdateStatus(ctx, deployment.ID, status)

	return &DeployResponse{
		DeploymentID: deployment.ID,
		ClusterType:  "mha",
		Name:         req.Name,
		Status:       status,
		Message:      result.Message,
		CreatedAt:    deployment.CreatedAt,
	}, nil
}

func (s *ClusterDeployService) DeployMGR(ctx context.Context, req DeployMGRRequest) (*DeployResponse, error) {
	deployment := &models.ClusterDeployment{
		ClusterType: "mgr",
		Name:        req.Name,
		Status:      "pending",
	}
	if err := s.repo.Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	allHosts := append([]SecondaryNode{{Host: req.PrimaryHost, Port: req.PrimaryPort}}, req.SecondaryHosts...)

	for i, node := range allHosts {
		var groupSeeds []string
		for j, other := range allHosts {
			if i != j {
				groupSeeds = append(groupSeeds, fmt.Sprintf("%s:%d", other.Host, 33061))
			}
			_ = j
		}

		isPrimary := i == 0
		deployMode := "mgr"
		config := map[string]interface{}{
			"deploy_mode":    deployMode,
			"group_name":     req.Name,
			"group_seeds":    groupSeeds,
			"local_address":  node.Host,
			"local_port":     33061,
			"server_id":      i + 1,
			"primary_host":   req.PrimaryHost,
			"primary_port":   req.PrimaryPort,
			"replicate_user": s.defaults.ReplicationUser,
			"replicate_pass": s.defaults.ReplicationPass,
			"mysql_user":     req.MySQLUser,
			"mysql_password": req.MySQLPassword,
			"bootstrap":      isPrimary,
		}

		agentPort := 9090
		result, err := s.agentClient.callAgent(ctx, node.Host, agentPort, "/agent/tasks/deploy", map[string]interface{}{
			"task_id":     deployment.ID,
			"instance_id": "",
			"config":      config,
		})
		if err != nil {
			s.repo.UpdateStatus(ctx, deployment.ID, "failed")
			return &DeployResponse{
				DeploymentID: deployment.ID,
				ClusterType:  "mgr",
				Name:         req.Name,
				Status:       "failed",
				Message:      fmt.Sprintf("MGR deploy failed on %s: %v", node.Host, err),
				CreatedAt:    deployment.CreatedAt,
			}, nil
		}
		if result.Status == "failed" {
			s.repo.UpdateStatus(ctx, deployment.ID, "failed")
			return &DeployResponse{
				DeploymentID: deployment.ID,
				ClusterType:  "mgr",
				Name:         req.Name,
				Status:       "failed",
				Message:      result.Message,
				CreatedAt:    deployment.CreatedAt,
			}, nil
		}
	}

	s.repo.UpdateStatus(ctx, deployment.ID, "completed")
	return &DeployResponse{
		DeploymentID: deployment.ID,
		ClusterType:  "mgr",
		Name:         req.Name,
		Status:       "completed",
		Message:      "MGR cluster deployed successfully",
		CreatedAt:    deployment.CreatedAt,
	}, nil
}

func (s *ClusterDeployService) DeployPXC(ctx context.Context, req DeployPXCRequest) (*DeployResponse, error) {
	deployment := &models.ClusterDeployment{
		ClusterType: "pxc",
		Name:        req.Name,
		Status:      "pending",
	}
	if err := s.repo.Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	bootstrapConfig := map[string]interface{}{
		"deploy_mode":    "pxc",
		"cluster_name":   req.Name,
		"bootstrap":      true,
		"nodes":          []string{req.BootstrapNode.Host},
		"wsrep_port":     4567,
		"sst_method":     "xtrabackup-v2",
		"replicate_user": s.defaults.SSTUser,
		"replicate_pass": s.defaults.SSTPass,
		"mysql_user":     req.MySQLUser,
		"mysql_password": req.MySQLPassword,
		"data_dir":       "/var/lib/mysql",
	}

	agentPort := 9090
	result, err := s.agentClient.callAgent(ctx, req.BootstrapNode.Host, agentPort, "/agent/tasks/deploy", map[string]interface{}{
		"task_id":     deployment.ID,
		"instance_id": "",
		"config":      bootstrapConfig,
	})
	if err != nil {
		s.repo.UpdateStatus(ctx, deployment.ID, "failed")
		return &DeployResponse{
			DeploymentID: deployment.ID,
			ClusterType:  "pxc",
			Name:         req.Name,
			Status:       "failed",
			Message:      fmt.Sprintf("PXC bootstrap failed: %v", err),
			CreatedAt:    deployment.CreatedAt,
		}, nil
	}
	if result.Status == "failed" {
		s.repo.UpdateStatus(ctx, deployment.ID, "failed")
		return &DeployResponse{
			DeploymentID: deployment.ID,
			ClusterType:  "pxc",
			Name:         req.Name,
			Status:       "failed",
			Message:      result.Message,
			CreatedAt:    deployment.CreatedAt,
		}, nil
	}

	for _, node := range req.OtherNodes {
		joinConfig := map[string]interface{}{
			"deploy_mode":    "pxc",
			"cluster_name":   req.Name,
			"bootstrap":      false,
			"nodes":          []string{node.Host},
			"wsrep_port":     4567,
			"sst_method":     "xtrabackup-v2",
			"replicate_user": s.defaults.SSTUser,
			"replicate_pass": s.defaults.SSTPass,
			"mysql_user":     req.MySQLUser,
			"mysql_password": req.MySQLPassword,
			"data_dir":       "/var/lib/mysql",
		}
		result, err := s.agentClient.callAgent(ctx, node.Host, agentPort, "/agent/tasks/deploy", map[string]interface{}{
			"task_id":     deployment.ID,
			"instance_id": "",
			"config":      joinConfig,
		})
		if err != nil || result.Status == "failed" {
			s.repo.UpdateStatus(ctx, deployment.ID, "partial")
			msg := "PXC cluster partially deployed (bootstrap OK, some nodes failed)"
			if err != nil {
				msg = fmt.Sprintf("%s: %v", msg, err)
			}
			return &DeployResponse{
				DeploymentID: deployment.ID,
				ClusterType:  "pxc",
				Name:         req.Name,
				Status:       "partial",
				Message:      msg,
				CreatedAt:    deployment.CreatedAt,
			}, nil
		}
	}

	s.repo.UpdateStatus(ctx, deployment.ID, "completed")
	return &DeployResponse{
		DeploymentID: deployment.ID,
		ClusterType:  "pxc",
		Name:         req.Name,
		Status:       "completed",
		Message:      "PXC cluster deployed successfully",
		CreatedAt:    deployment.CreatedAt,
	}, nil
}

func (s *ClusterDeployService) GetDeploymentStatus(ctx context.Context, deploymentID string) (*DeployResponse, error) {
	dep, err := s.repo.GetByID(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	return &DeployResponse{
		DeploymentID: dep.ID,
		ClusterType:  dep.ClusterType,
		Name:         dep.Name,
		Status:       dep.Status,
		CreatedAt:    dep.CreatedAt,
	}, nil
}

type DeployMHARequest struct {
	Name          string            `json:"name" binding:"required"`
	MasterHost    string            `json:"master_host" binding:"required"`
	MasterPort    int               `json:"master_port" binding:"required"`
	SlaveHosts    []SlaveNode       `json:"slave_hosts" binding:"required"`
	VIP           string            `json:"vip" binding:"required"`
	ManagerHost   string            `json:"manager_host" binding:"required"`
	MySQLUser     string            `json:"mysql_user"`
	MySQLPassword string            `json:"mysql_password"`
	ConfigParams  map[string]string `json:"config_params"`
}

type DeployMGRRequest struct {
	Name           string            `json:"name" binding:"required"`
	PrimaryHost    string            `json:"primary_host" binding:"required"`
	PrimaryPort    int               `json:"primary_port" binding:"required"`
	SecondaryHosts []SecondaryNode   `json:"secondary_hosts" binding:"required"`
	GroupMode      string            `json:"group_mode" binding:"required"`
	MySQLUser      string            `json:"mysql_user"`
	MySQLPassword  string            `json:"mysql_password"`
	ConfigParams   map[string]string `json:"config_params"`
}

type DeployPXCRequest struct {
	Name          string            `json:"name" binding:"required"`
	BootstrapNode BootstrapNode     `json:"bootstrap_node" binding:"required"`
	OtherNodes    []PXCNode         `json:"other_nodes" binding:"required"`
	SSLEnabled    bool              `json:"ssl_enabled"`
	MySQLUser     string            `json:"mysql_user"`
	MySQLPassword string            `json:"mysql_password"`
	ConfigParams  map[string]string `json:"config_params"`
}

type SlaveNode struct {
	Host string `json:"host" binding:"required"`
	Port int    `json:"port" binding:"required"`
}

type SecondaryNode struct {
	Host string `json:"host" binding:"required"`
	Port int    `json:"port" binding:"required"`
}

type BootstrapNode struct {
	Host string `json:"host" binding:"required"`
	Port int    `json:"port" binding:"required"`
}

type PXCNode struct {
	Host string `json:"host" binding:"required"`
	Port int    `json:"port" binding:"required"`
}

type DeployResponse struct {
	DeploymentID string    `json:"deployment_id"`
	ClusterType  string    `json:"cluster_type"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}
