package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/config"
)

type ClusterDeployService struct {
	repo        *repositories.ClusterDeployRepository
	hostRepo    *repositories.HostRepository
	instRepo    *repositories.InstanceRepository
	agentClient *AgentClient
	auditSvc    *AuditService
	defaults    config.ClusterDefaults
}

type deploymentHost struct {
	Address   string
	AgentPort int
}

type pseudoNode struct {
	Host string
	Port int
	Role string
}

func NewClusterDeployService(
	repo *repositories.ClusterDeployRepository,
	hostRepo *repositories.HostRepository,
	instRepo *repositories.InstanceRepository,
	agentClient *AgentClient,
	defaults config.ClusterDefaults,
	auditSvc ...*AuditService,
) *ClusterDeployService {
	var audit *AuditService
	if len(auditSvc) > 0 {
		audit = auditSvc[0]
	}
	return &ClusterDeployService{
		repo:        repo,
		hostRepo:    hostRepo,
		instRepo:    instRepo,
		agentClient: agentClient,
		auditSvc:    audit,
		defaults:    defaults,
	}
}

func (s *ClusterDeployService) DeployMHA(ctx context.Context, req DeployMHARequest) (resp *DeployResponse, err error) {
	defer func() { s.auditDeployment(ctx, "deploy_mha_cluster", "mha", req.ClusterID, req.Name, resp, err) }()
	if err := s.resolveMHARequestHosts(ctx, &req); err != nil {
		return nil, err
	}
	if req.PseudoMode {
		return s.deployPseudoCluster(ctx, "mha", req.ClusterID, req.Name, []pseudoNode{
			{Host: req.MasterHost, Port: req.MasterPort, Role: "master"},
		}, slavePseudoNodes(req.SlaveHosts, "slave"))
	}
	deployment := &models.ClusterDeployment{
		ID:          req.ClusterID,
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

func (s *ClusterDeployService) DeployMGR(ctx context.Context, req DeployMGRRequest) (resp *DeployResponse, err error) {
	defer func() { s.auditDeployment(ctx, "deploy_mgr_cluster", "mgr", req.ClusterID, req.Name, resp, err) }()
	if err := s.resolveMGRRequestHosts(ctx, &req); err != nil {
		return nil, err
	}
	if req.PseudoMode {
		return s.deployPseudoCluster(ctx, "mgr", req.ClusterID, req.Name, []pseudoNode{
			{Host: req.PrimaryHost, Port: req.PrimaryPort, Role: "primary"},
		}, secondaryPseudoNodes(req.SecondaryHosts, "secondary"))
	}
	deployment := &models.ClusterDeployment{
		ID:          req.ClusterID,
		ClusterType: "mgr",
		Name:        req.Name,
		Status:      "pending",
	}
	if err := s.repo.Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	allHosts := append([]SecondaryNode{{Host: req.PrimaryHost, Port: req.PrimaryPort}}, req.SecondaryHosts...)
	localPorts := make([]int, len(allHosts))
	for i := range allHosts {
		localPorts[i] = 33061 + i
	}

	groupName := defaultString(req.ConfigParams["group_name"], uuid.New().String())
	for i, node := range allHosts {
		var groupSeeds []string
		for j, other := range allHosts {
			if i != j {
				groupSeeds = append(groupSeeds, fmt.Sprintf("%s:%d", other.Host, localPorts[j]))
			}
		}

		isPrimary := i == 0
		deployMode := "mgr"
		config := map[string]interface{}{
			"deploy_mode":    deployMode,
			"group_name":     groupName,
			"group_seeds":    groupSeeds,
			"local_address":  node.Host,
			"local_port":     localPorts[i],
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
	if err := s.syncClusterManagement(ctx, "mgr", deployment.ID, allHostsToPseudoNodes(allHosts, "primary", "secondary")); err != nil {
		s.repo.UpdateStatus(ctx, deployment.ID, "partial")
		return &DeployResponse{
			DeploymentID: deployment.ID,
			ClusterType:  "mgr",
			Name:         req.Name,
			Status:       "partial",
			Message:      fmt.Sprintf("MGR deployed but management sync failed: %v", err),
			CreatedAt:    deployment.CreatedAt,
		}, nil
	}
	return &DeployResponse{
		DeploymentID: deployment.ID,
		ClusterType:  "mgr",
		Name:         req.Name,
		Status:       "completed",
		Message:      "MGR cluster deployed successfully",
		CreatedAt:    deployment.CreatedAt,
	}, nil
}

func (s *ClusterDeployService) DeployPXC(ctx context.Context, req DeployPXCRequest) (resp *DeployResponse, err error) {
	defer func() { s.auditDeployment(ctx, "deploy_pxc_cluster", "pxc", req.ClusterID, req.Name, resp, err) }()
	if err := s.resolvePXCRequestHosts(ctx, &req); err != nil {
		return nil, err
	}
	if req.PseudoMode {
		nodes := []pseudoNode{{Host: req.BootstrapNode.Host, Port: req.BootstrapNode.Port, Role: "primary"}}
		nodes = append(nodes, pxcPseudoNodes(req.OtherNodes, "secondary")...)
		return s.deployPseudoCluster(ctx, "pxc", req.ClusterID, req.Name, nodes, nil)
	}
	deployment := &models.ClusterDeployment{
		ID:          req.ClusterID,
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

func (s *ClusterDeployService) DeployHA(ctx context.Context, req DeployHARequest) (resp *DeployResponse, err error) {
	defer func() { s.auditDeployment(ctx, "deploy_ha_cluster", "ha", req.ClusterID, req.Name, resp, err) }()
	if err := s.resolveHARequestHosts(ctx, &req); err != nil {
		return nil, err
	}
	if req.PseudoMode {
		return s.deployPseudoCluster(ctx, "ha", req.ClusterID, req.Name, []pseudoNode{
			{Host: req.MasterHost, Port: req.MasterPort, Role: "master"},
			{Host: req.ReplicaHost, Port: req.ReplicaPort, Role: "slave"},
		}, nil)
	}
	deployment := &models.ClusterDeployment{
		ID:          req.ClusterID,
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
		Nodes:        s.deploymentNodes(ctx, dep.ID),
	}, nil
}

func (s *ClusterDeployService) ListDeployments(ctx context.Context, limit, offset int) ([]DeployResponse, error) {
	deployments, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	responses := make([]DeployResponse, 0, len(deployments))
	for _, dep := range deployments {
		responses = append(responses, DeployResponse{
			DeploymentID: dep.ID,
			ClusterType:  dep.ClusterType,
			Name:         dep.Name,
			Status:       dep.Status,
			CreatedAt:    dep.CreatedAt,
			Nodes:        s.deploymentNodes(ctx, dep.ID),
		})
	}
	return responses, nil
}

func (s *ClusterDeployService) DestroyCluster(ctx context.Context, clusterID string) (resp *DeployResponse, err error) {
	defer func() { s.auditDeployment(ctx, "destroy_cluster", "", clusterID, "", resp, err) }()
	dep, err := s.repo.GetByID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	nodes := s.deploymentNodes(ctx, clusterID)
	if err := s.clearClusterManagement(ctx, clusterID); err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, clusterID); err != nil {
		return nil, err
	}
	return &DeployResponse{
		DeploymentID: dep.ID,
		ClusterType:  dep.ClusterType,
		Name:         dep.Name,
		Status:       "destroyed",
		Message:      fmt.Sprintf("Cluster %s destroyed from platform management; database services were not stopped", clusterID),
		CreatedAt:    dep.CreatedAt,
		Nodes:        nodes,
	}, nil
}

func (s *ClusterDeployService) deploymentNodes(ctx context.Context, clusterID string) []DeployNode {
	if s.instRepo == nil || clusterID == "" {
		return nil
	}
	instances, err := s.instRepo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return nil
	}
	nodes := make([]DeployNode, 0, len(instances))
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		if full, err := s.instRepo.GetByID(ctx, inst.ID); err == nil && full != nil {
			inst = full
		}
		node := DeployNode{
			InstanceID: inst.ID,
			Name:       inst.Name,
			Host:       inst.Connection.Host,
			Port:       inst.Connection.Port,
			Role:       inst.Status.Role,
		}
		if node.Host == "" {
			if conn, err := s.instRepo.GetConnection(ctx, inst.ID); err == nil && conn != nil {
				node.Host = conn.Host
				node.Port = conn.Port
			}
		}
		if node.Role == "" {
			node.Role = "unknown"
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (s *ClusterDeployService) auditDeployment(ctx context.Context, operation, clusterType, requestedID, requestedName string, resp *DeployResponse, err error) {
	if s.auditSvc == nil {
		return
	}
	resourceID := requestedID
	if resp != nil && resp.DeploymentID != "" {
		resourceID = resp.DeploymentID
	}
	if resourceID == "" {
		resourceID = requestedName
	}
	if clusterType == "" && resp != nil {
		clusterType = resp.ClusterType
	}
	name := requestedName
	if resp != nil && resp.Name != "" {
		name = resp.Name
	}
	status := ""
	message := ""
	if resp != nil {
		status = resp.Status
		message = resp.Message
	}
	auditResult := deployAuditResult(status, err)
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	} else if auditResult == "failed" {
		errorMsg = message
	}
	details := fmt.Sprintf("cluster_id=%s cluster_type=%s name=%s status=%s message=%s",
		resourceID, clusterType, name, status, message)
	action := "deploy"
	if operation == "destroy_cluster" {
		action = "destroy"
	}
	_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID:       userIDFromCtx(ctx),
		Operation:    operation,
		ResourceType: "cluster_deployment",
		ResourceID:   resourceID,
		Action:       action,
		Details:      details,
		Result:       auditResult,
		ErrorMsg:     errorMsg,
	})
}

func deployAuditResult(status string, err error) string {
	if err != nil {
		return "failed"
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "timeout", "cancelled", "canceled", "partial", "partial_success":
		return "failed"
	default:
		return "success"
	}
}

func (s *ClusterDeployService) clearClusterManagement(ctx context.Context, clusterID string) error {
	instances, err := s.instRepo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return err
	}
	for _, inst := range instances {
		inst.ClusterID = ""
		if err := s.instRepo.Update(ctx, inst); err != nil {
			return err
		}
		_ = s.instRepo.UpsertStatus(ctx, inst.ID, &models.InstanceStatus{
			RunStatus:           "running",
			HealthStatus:        "healthy",
			Role:                "",
			ReplicationStatus:   "",
			SecondsBehindMaster: 0,
		})
		_ = s.instRepo.UpsertTopology(ctx, inst.ID, &models.InstanceTopology{
			InstanceID:      inst.ID,
			ClusterID:       "",
			MasterID:        "",
			SlaveIDs:        "",
			ReplicationMode: "",
		})
	}
	return nil
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
	PseudoMode       bool              `json:"pseudo_mode"`
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
	PseudoMode       bool              `json:"pseudo_mode"`
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
	PseudoMode      bool              `json:"pseudo_mode"`
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
	PseudoMode      bool   `json:"pseudo_mode"`
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
	DeploymentID string       `json:"deployment_id"`
	ClusterType  string       `json:"cluster_type"`
	Name         string       `json:"name"`
	Status       string       `json:"status"`
	Message      string       `json:"message"`
	CreatedAt    time.Time    `json:"created_at"`
	Nodes        []DeployNode `json:"nodes,omitempty"`
}

type DeployNode struct {
	InstanceID string `json:"instance_id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Role       string `json:"role"`
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

func (s *ClusterDeployService) deployPseudoCluster(ctx context.Context, clusterType, clusterID, name string, primaryNodes []pseudoNode, replicaNodes []pseudoNode) (*DeployResponse, error) {
	clusterID = defaultString(clusterID, defaultString(name, fmt.Sprintf("%s-%d", clusterType, time.Now().Unix())))
	name = defaultString(name, clusterID)
	nodes := append([]pseudoNode{}, primaryNodes...)
	nodes = append(nodes, replicaNodes...)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("pseudo cluster requires at least one node")
	}
	if err := s.clearClusterManagement(ctx, clusterID); err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, clusterID); err != nil {
		// Ignore missing cluster rows; Create below will surface real database errors.
		_ = err
	}
	deployment := &models.ClusterDeployment{
		ID:          clusterID,
		ClusterType: clusterType,
		Name:        name,
		Status:      "success",
	}
	if err := s.repo.Create(ctx, deployment); err != nil {
		return nil, fmt.Errorf("failed to create pseudo deployment: %w", err)
	}

	if err := s.syncClusterManagement(ctx, clusterType, clusterID, nodes); err != nil {
		s.repo.UpdateStatus(ctx, deployment.ID, "failed")
		return &DeployResponse{
			DeploymentID: deployment.ID,
			ClusterType:  clusterType,
			Name:         name,
			Status:       "failed",
			Message:      err.Error(),
			CreatedAt:    deployment.CreatedAt,
		}, nil
	}

	return &DeployResponse{
		DeploymentID: deployment.ID,
		ClusterType:  clusterType,
		Name:         name,
		Status:       "success",
		Message:      fmt.Sprintf("Pseudo %s cluster %s is managed with %d instances", clusterType, clusterID, len(nodes)),
		CreatedAt:    deployment.CreatedAt,
	}, nil
}

func (s *ClusterDeployService) syncClusterManagement(ctx context.Context, clusterType, clusterID string, nodes []pseudoNode) error {
	matched := make([]*models.Instance, 0, len(nodes))
	for _, node := range nodes {
		inst, err := s.findInstanceByEndpoint(ctx, node.Host, node.Port)
		if err != nil {
			return err
		}
		inst.ClusterID = clusterID
		if err := s.instRepo.Update(ctx, inst); err != nil {
			return err
		}
		if err := s.instRepo.UpsertStatus(ctx, inst.ID, &models.InstanceStatus{
			RunStatus:           "running",
			HealthStatus:        "healthy",
			Role:                node.Role,
			ReplicationStatus:   clusterType,
			SecondsBehindMaster: 0,
		}); err != nil {
			return err
		}
		matched = append(matched, inst)
	}
	if len(matched) == 0 {
		return fmt.Errorf("no managed instances found for cluster %s", clusterID)
	}

	primaryID := matched[0].ID
	replicaIDs := make([]string, 0, len(matched)-1)
	for i, inst := range matched {
		if i > 0 {
			replicaIDs = append(replicaIDs, inst.ID)
		}
	}
	replicaJSON, _ := json.Marshal(replicaIDs)
	for i, inst := range matched {
		topology := &models.InstanceTopology{
			InstanceID:      inst.ID,
			ClusterID:       clusterID,
			ReplicationMode: clusterType,
		}
		if i == 0 {
			topology.SlaveIDs = string(replicaJSON)
		} else {
			topology.MasterID = primaryID
		}
		if err := s.instRepo.UpsertTopology(ctx, inst.ID, topology); err != nil {
			return err
		}
	}
	return nil
}

func (s *ClusterDeployService) findInstanceByEndpoint(ctx context.Context, host string, port int) (*models.Instance, error) {
	instances, err := s.instRepo.List(ctx, 1000, 0)
	if err != nil {
		return nil, err
	}
	for _, item := range instances {
		inst := item
		conn, err := s.instRepo.GetConnection(ctx, inst.ID)
		if err != nil {
			continue
		}
		if conn.Port == port && sameHost(conn.Host, host) {
			return &inst, nil
		}
	}
	return nil, fmt.Errorf("no managed instance found for %s:%d", host, port)
}

func sameHost(a, b string) bool {
	if a == b {
		return true
	}
	if (a == "127.0.0.1" || a == "localhost") && (b == "127.0.0.1" || b == "localhost") {
		return true
	}
	return false
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
	if len(req.OtherHostIDs) > 0 {
		resolved := make([]PXCNode, 0, len(req.OtherHostIDs))
		for _, id := range req.OtherHostIDs {
			idx := len(resolved)
			host, err := s.resolveHostRef(ctx, id, "")
			if err != nil {
				return err
			}
			port := req.BootstrapNode.Port
			if idx < len(req.OtherNodes) && req.OtherNodes[idx].Port != 0 {
				port = req.OtherNodes[idx].Port
			}
			resolved = append(resolved, PXCNode{Host: host.Address, Port: port})
		}
		req.OtherNodes = resolved
	}
	for i := range req.OtherNodes {
		if req.OtherNodes[i].Host == "" {
			req.OtherNodes[i].Host = req.BootstrapNode.Host
		}
		if req.OtherNodes[i].Port == 0 {
			req.OtherNodes[i].Port = req.BootstrapNode.Port
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

func slavePseudoNodes(nodes []SlaveNode, role string) []pseudoNode {
	out := make([]pseudoNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, pseudoNode{Host: node.Host, Port: node.Port, Role: role})
	}
	return out
}

func secondaryPseudoNodes(nodes []SecondaryNode, role string) []pseudoNode {
	out := make([]pseudoNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, pseudoNode{Host: node.Host, Port: node.Port, Role: role})
	}
	return out
}

func allHostsToPseudoNodes(nodes []SecondaryNode, primaryRole, replicaRole string) []pseudoNode {
	out := make([]pseudoNode, 0, len(nodes))
	for i, node := range nodes {
		role := replicaRole
		if i == 0 {
			role = primaryRole
		}
		out = append(out, pseudoNode{Host: node.Host, Port: node.Port, Role: role})
	}
	return out
}

func pxcPseudoNodes(nodes []PXCNode, role string) []pseudoNode {
	out := make([]pseudoNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, pseudoNode{Host: node.Host, Port: node.Port, Role: role})
	}
	return out
}
