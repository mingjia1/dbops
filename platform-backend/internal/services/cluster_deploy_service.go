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

type deploymentHost struct {
	Address   string
	AgentPort int
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
	if err := s.resolveMHARequestHosts(ctx, &req); err != nil {
		return nil, err
	}
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
		"slave_ports":    slavePorts(req.SlaveHosts),
		"repl_user":      defaultString(req.ReplUser, s.defaults.ReplicationUser),
		"repl_pass":      defaultString(req.ReplPassword, s.defaults.ReplicationPass),
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
	agentPort := req.ManagerAgentPort
	if agentPort == 0 {
		agentPort = 9090
	}

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
	if err := s.resolveMGRRequestHosts(ctx, &req); err != nil {
		return nil, err
	}
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
			"mysql_port":     node.Port,
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
	if err := s.resolvePXCRequestHosts(ctx, &req); err != nil {
		return nil, err
	}
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
		"mysql_port":     defaultInt(req.BootstrapNode.Port, 3306),
		"wsrep_port":     defaultInt(req.WSREPPort, 4567),
		"sst_method":     "xtrabackup-v2",
		"replicate_user": s.defaults.SSTUser,
		"replicate_pass": s.defaults.SSTPass,
		"mysql_user":     req.MySQLUser,
		"mysql_password": req.MySQLPassword,
		"data_dir":       fmt.Sprintf("/data/mysql/pxc-%d", defaultInt(req.BootstrapNode.Port, 3306)),
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
			"mysql_port":     defaultInt(node.Port, 3306),
			"wsrep_port":     defaultInt(req.WSREPPort, 4567),
			"sst_method":     "xtrabackup-v2",
			"replicate_user": s.defaults.SSTUser,
			"replicate_pass": s.defaults.SSTPass,
			"mysql_user":     req.MySQLUser,
			"mysql_password": req.MySQLPassword,
			"data_dir":       fmt.Sprintf("/data/mysql/pxc-%d", defaultInt(node.Port, 3306)),
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

func (s *ClusterDeployService) DeployHA(ctx context.Context, req DeployHARequest) (*DeployResponse, error) {
	if err := s.resolveHARequestHosts(ctx, &req); err != nil {
		return nil, err
	}
	deployment := &models.ClusterDeployment{
		ClusterType: "ha",
		Name:        req.Name,
		Status:      "pending",
	}
	if err := s.repo.Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	config := map[string]interface{}{
		"deploy_mode":      "master-slave",
		"master_host":      req.MasterHost,
		"master_port":      req.MasterPort,
		"slave_host":       req.ReplicaHost,
		"slave_port":       req.ReplicaPort,
		"replicate_user":   defaultString(req.ReplUser, s.defaults.ReplicationUser),
		"replicate_pass":   defaultString(req.ReplPassword, s.defaults.ReplicationPass),
		"mysql_user":       defaultString(req.MySQLUser, "root"),
		"mysql_password":   req.MySQLPassword,
		"replication_mode": "async",
	}

	agentPort := req.MasterAgentPort
	if agentPort == 0 {
		agentPort = 9090
	}
	result, err := s.agentClient.callAgent(ctx, req.MasterHost, agentPort, "/agent/tasks/deploy", map[string]interface{}{
		"task_id":     deployment.ID,
		"instance_id": "",
		"config":      config,
	})
	if err != nil {
		s.repo.UpdateStatus(ctx, deployment.ID, "failed")
		return &DeployResponse{
			DeploymentID: deployment.ID,
			ClusterType:  "ha",
			Name:         req.Name,
			Status:       "failed",
			Message:      fmt.Sprintf("HA master-replica deploy failed: %v", err),
			CreatedAt:    deployment.CreatedAt,
		}, nil
	}
	status := normalizeDeployStatus(result.Status)
	s.repo.UpdateStatus(ctx, deployment.ID, status)
	return &DeployResponse{
		DeploymentID: deployment.ID,
		ClusterType:  "ha",
		Name:         req.Name,
		Status:       status,
		Message:      result.Message,
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
	Name             string            `json:"name"`
	ClusterID        string            `json:"cluster_id"`
	MasterHostID     string            `json:"master_host_id"`
	ManagerHostID    string            `json:"manager_host_id"`
	ReplicaHostIDs   []string          `json:"replica_host_ids"`
	MasterHost       string            `json:"master_host"`
	MasterPort       int               `json:"master_port"`
	SlaveHosts       []SlaveNode       `json:"slave_hosts"`
	VIP              string            `json:"vip"`
	ManagerHost      string            `json:"manager_host"`
	ManagerAgentPort int               `json:"manager_agent_port"`
	ReplicaPort      int               `json:"replica_port"`
	ReplUser         string            `json:"repl_user"`
	ReplPassword     string            `json:"repl_password"`
	MySQLUser        string            `json:"mysql_user"`
	MySQLPassword    string            `json:"mysql_password"`
	ConfigParams     map[string]string `json:"config_params"`
}

type DeployMGRRequest struct {
	Name             string            `json:"name"`
	ClusterID        string            `json:"cluster_id"`
	PrimaryHostID    string            `json:"master_host_id"`
	SecondaryHostIDs []string          `json:"replica_host_ids"`
	PrimaryHost      string            `json:"primary_host"`
	PrimaryPort      int               `json:"primary_port"`
	ReplicaPort      int               `json:"replica_port"`
	SecondaryHosts   []SecondaryNode   `json:"secondary_hosts"`
	GroupMode        string            `json:"group_mode"`
	MySQLUser        string            `json:"mysql_user"`
	MySQLPassword    string            `json:"mysql_password"`
	ConfigParams     map[string]string `json:"config_params"`
}

type DeployPXCRequest struct {
	Name            string            `json:"name"`
	ClusterID       string            `json:"cluster_id"`
	BootstrapHostID string            `json:"master_host_id"`
	OtherHostIDs    []string          `json:"replica_host_ids"`
	BootstrapNode   BootstrapNode     `json:"bootstrap_node"`
	OtherNodes      []PXCNode         `json:"other_nodes"`
	SSLEnabled      bool              `json:"ssl_enabled"`
	WSREPPort       int               `json:"wsrep_port"`
	MySQLUser       string            `json:"mysql_user"`
	MySQLPassword   string            `json:"mysql_password"`
	ConfigParams    map[string]string `json:"config_params"`
}

type DeployHARequest struct {
	Name            string `json:"name"`
	ClusterID       string `json:"cluster_id"`
	MasterHostID    string `json:"master_host_id"`
	ReplicaHostID   string `json:"replica_host_id"`
	MasterHost      string `json:"master_host"`
	ReplicaHost     string `json:"replica_host"`
	MasterPort      int    `json:"master_port"`
	ReplicaPort     int    `json:"replica_port"`
	MasterAgentPort int    `json:"master_agent_port"`
	ReplUser        string `json:"repl_user"`
	ReplPassword    string `json:"repl_password"`
	MySQLUser       string `json:"mysql_user"`
	MySQLPassword   string `json:"mysql_password"`
}

type SlaveNode struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type SecondaryNode struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type BootstrapNode struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type PXCNode struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type DeployResponse struct {
	DeploymentID string    `json:"deployment_id"`
	ClusterType  string    `json:"cluster_type"`
	Name         string    `json:"name"`
	Status       string    `json:"status"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *ClusterDeployService) resolveHostRef(ctx context.Context, hostID, fallbackAddress string) (deploymentHost, error) {
	if hostID == "" {
		return deploymentHost{Address: fallbackAddress, AgentPort: 9090}, nil
	}
	if s.hostRepo == nil {
		return deploymentHost{}, fmt.Errorf("host repository is not configured")
	}
	host, err := s.hostRepo.GetByID(ctx, hostID)
	if err != nil {
		return deploymentHost{}, err
	}
	port := host.AgentPort
	if port == 0 {
		port = 9090
	}
	return deploymentHost{Address: host.Address, AgentPort: port}, nil
}

func (s *ClusterDeployService) resolveMHARequestHosts(ctx context.Context, req *DeployMHARequest) error {
	if req.Name == "" {
		req.Name = defaultString(req.ClusterID, fmt.Sprintf("mha-%d", time.Now().Unix()))
	}
	master, err := s.resolveHostRef(ctx, req.MasterHostID, req.MasterHost)
	if err != nil {
		return err
	}
	manager, err := s.resolveHostRef(ctx, req.ManagerHostID, req.ManagerHost)
	if err != nil {
		return err
	}
	if master.Address != "" {
		req.MasterHost = master.Address
	}
	if manager.Address != "" {
		req.ManagerHost = manager.Address
		req.ManagerAgentPort = manager.AgentPort
	}
	if req.MasterPort == 0 {
		req.MasterPort = 3306
	}
	if len(req.SlaveHosts) == 0 {
		for _, id := range req.ReplicaHostIDs {
			host, err := s.resolveHostRef(ctx, id, "")
			if err != nil {
				return err
			}
			req.SlaveHosts = append(req.SlaveHosts, SlaveNode{Host: host.Address, Port: req.MasterPort})
		}
	}
	if req.ReplicaPort != 0 {
		for i := range req.SlaveHosts {
			req.SlaveHosts[i].Port = req.ReplicaPort
		}
	}
	return nil
}

func (s *ClusterDeployService) resolveMGRRequestHosts(ctx context.Context, req *DeployMGRRequest) error {
	if req.Name == "" {
		req.Name = defaultString(req.ClusterID, fmt.Sprintf("mgr-%d", time.Now().Unix()))
	}
	primary, err := s.resolveHostRef(ctx, req.PrimaryHostID, req.PrimaryHost)
	if err != nil {
		return err
	}
	if primary.Address != "" {
		req.PrimaryHost = primary.Address
	}
	if req.PrimaryPort == 0 {
		req.PrimaryPort = 3306
	}
	if len(req.SecondaryHosts) == 0 {
		for _, id := range req.SecondaryHostIDs {
			host, err := s.resolveHostRef(ctx, id, "")
			if err != nil {
				return err
			}
			req.SecondaryHosts = append(req.SecondaryHosts, SecondaryNode{Host: host.Address, Port: req.PrimaryPort})
		}
	}
	if req.ReplicaPort != 0 {
		for i := range req.SecondaryHosts {
			req.SecondaryHosts[i].Port = req.ReplicaPort
		}
	}
	if req.GroupMode == "" {
		req.GroupMode = "single-primary"
	}
	return nil
}

func (s *ClusterDeployService) resolvePXCRequestHosts(ctx context.Context, req *DeployPXCRequest) error {
	if req.Name == "" {
		req.Name = defaultString(req.ClusterID, fmt.Sprintf("pxc-%d", time.Now().Unix()))
	}
	bootstrap, err := s.resolveHostRef(ctx, req.BootstrapHostID, req.BootstrapNode.Host)
	if err != nil {
		return err
	}
	if bootstrap.Address != "" {
		req.BootstrapNode.Host = bootstrap.Address
	}
	if req.BootstrapNode.Port == 0 {
		req.BootstrapNode.Port = 3306
	}
	if len(req.OtherNodes) == 0 {
		for _, id := range req.OtherHostIDs {
			host, err := s.resolveHostRef(ctx, id, "")
			if err != nil {
				return err
			}
			req.OtherNodes = append(req.OtherNodes, PXCNode{Host: host.Address, Port: req.BootstrapNode.Port})
		}
	}
	return nil
}

func (s *ClusterDeployService) resolveHARequestHosts(ctx context.Context, req *DeployHARequest) error {
	if req.Name == "" {
		req.Name = defaultString(req.ClusterID, fmt.Sprintf("ha-%d", time.Now().Unix()))
	}
	master, err := s.resolveHostRef(ctx, req.MasterHostID, req.MasterHost)
	if err != nil {
		return err
	}
	replica, err := s.resolveHostRef(ctx, req.ReplicaHostID, req.ReplicaHost)
	if err != nil {
		return err
	}
	if master.Address != "" {
		req.MasterHost = master.Address
		req.MasterAgentPort = master.AgentPort
	}
	if replica.Address != "" {
		req.ReplicaHost = replica.Address
	}
	if req.MasterPort == 0 {
		req.MasterPort = 3306
	}
	if req.ReplicaPort == 0 {
		req.ReplicaPort = 3307
	}
	if req.MasterHost == "" || req.ReplicaHost == "" {
		return fmt.Errorf("master_host and replica_host are required")
	}
	return nil
}

func normalizeDeployStatus(status string) string {
	if status == "completed" {
		return "success"
	}
	return status
}

func defaultString(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

func defaultInt(v, fallback int) int {
	if v != 0 {
		return v
	}
	return fallback
}

func slavePorts(nodes []SlaveNode) []int {
	ports := make([]int, 0, len(nodes))
	for _, node := range nodes {
		ports = append(ports, node.Port)
	}
	return ports
}
