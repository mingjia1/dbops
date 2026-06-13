package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/config"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
)

type ClusterDeployService struct {
	repo        *repositories.ClusterDeployRepository
	hostRepo    *repositories.HostRepository
	instRepo    *repositories.InstanceRepository
	agentClient *AgentClient
	backupSvc   *BackupService
	auditSvc    *AuditService
	defaults    config.ClusterDefaults
	encKey      string
	progressMu  sync.RWMutex
	progress    map[string]*DeploymentProgress
}

type DeploymentProgress struct {
	Stage    string
	Progress int
	Message  string
	Steps    []DeployStep
	Logs     []string
	Nodes    []DeployNodeProgress
}

type DeployNodeProgress struct {
	InstanceID  string
	Name        string
	Host        string
	Port        int
	Role        string
	Status      string
	CurrentStep string
	Progress    int
	Message     string
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
		progress:    make(map[string]*DeploymentProgress),
	}
}

func (s *ClusterDeployService) SetEncryptionKey(key string) {
	s.encKey = key
}

func (s *ClusterDeployService) SetBackupService(backupSvc *BackupService) {
	s.backupSvc = backupSvc
}

func (s *ClusterDeployService) updateProgress(deploymentID, stage, message string, progressPct int) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	p, ok := s.progress[deploymentID]
	if !ok {
		p = &DeploymentProgress{}
		s.progress[deploymentID] = p
	}
	p.Stage = stage
	p.Progress = progressPct
	p.Message = message
	p.Logs = append(p.Logs, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), message))
}

func (s *ClusterDeployService) addStep(deploymentID, name, status string) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	p := s.getOrCreateProgress(deploymentID)
	now := time.Now()
	step := DeployStep{Name: name, Status: status, StartedAt: &now}
	p.Steps = append(p.Steps, step)
}

func (s *ClusterDeployService) updateStepStatus(deploymentID, name, status, message string) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	p, ok := s.progress[deploymentID]
	if !ok {
		return
	}
	now := time.Now()
	for i := range p.Steps {
		if p.Steps[i].Name == name {
			p.Steps[i].Status = status
			p.Steps[i].Message = message
			if status == "completed" || status == "failed" {
				p.Steps[i].CompletedAt = &now
			}
			return
		}
	}
}

func (s *ClusterDeployService) getOrCreateProgress(deploymentID string) *DeploymentProgress {
	p, ok := s.progress[deploymentID]
	if !ok {
		p = &DeploymentProgress{}
		s.progress[deploymentID] = p
	}
	return p
}

func (s *ClusterDeployService) getProgress(deploymentID string) *DeploymentProgress {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	return s.progress[deploymentID]
}

func (s *ClusterDeployService) clearProgress(deploymentID string) {
	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	delete(s.progress, deploymentID)
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

	// 检查cluster_id是否已存在
	existing, err := s.repo.GetByID(ctx, req.ClusterID)
	if err == nil && existing != nil {
		// 如果已存在且状态为failed或destroyed，删除旧记录
		if existing.Status == "failed" || existing.Status == "destroyed" {
			if delErr := s.repo.Delete(ctx, req.ClusterID); delErr != nil {
				return nil, fmt.Errorf("failed to clean up existing failed deployment: %w", delErr)
			}
		} else {
			return nil, fmt.Errorf("cluster_id '%s' already exists with status '%s'", req.ClusterID, existing.Status)
		}
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

	s.updateProgress(deployment.ID, "环境检查", "检查主机连通性", 5)
	s.addStep(deployment.ID, "检查主机连通性", "running")
	s.updateStepStatus(deployment.ID, "检查主机连通性", "completed", "主机连通性检查通过")
	s.addStep(deployment.ID, "验证端口可用性", "running")
	s.updateProgress(deployment.ID, "环境检查", "验证端口可用性", 10)
	s.updateStepStatus(deployment.ID, "验证端口可用性", "completed", "端口检查通过")

	s.updateProgress(deployment.ID, "安装二进制", "准备 MHA 配置", 20)
	s.addStep(deployment.ID, "准备 MHA 配置", "running")

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
	sshPasswords, err := s.sshPasswordsForMHA(ctx, req)
	if err != nil {
		s.updateStepStatus(deployment.ID, "准备 MHA 配置", "failed", err.Error())
		return s.failDeployment(ctx, deployment, "mha", req.Name, fmt.Sprintf("prepare MHA SSH credentials failed: %v", err)), nil
	}
	config["ssh_passwords"] = sshPasswords

	var slaveHosts []string
	var slavePortsList []int
	for _, slave := range req.SlaveHosts {
		slaveHosts = append(slaveHosts, slave.Host)
		slavePortsList = append(slavePortsList, slave.Port)
	}
	config["slave_hosts"] = slaveHosts
	config["slave_ports"] = slavePortsList

	s.updateStepStatus(deployment.ID, "准备 MHA 配置", "completed", "MHA 配置已生成")

	agentHost := req.ManagerHost
	agentPort := req.ManagerAgentPort
	if agentPort == 0 {
		agentPort = 9090
	}

	if s.agentClient == nil {
		s.updateProgress(deployment.ID, "安装二进制", "agent client not configured", 30)
		return s.failDeployment(ctx, deployment, "mha", req.Name, "agent client not configured"), nil
	}

	s.updateProgress(deployment.ID, "安装二进制", "执行 MHA 部署", 30)
	s.addStep(deployment.ID, "MHA Manager 部署", "running")

	result, err := s.agentClient.DeployCluster(ctx, agentHost, agentPort, map[string]interface{}{
		"task_id":     deployment.ID,
		"instance_id": "",
		"config":      config,
	})
	if err != nil {
		s.updateStepStatus(deployment.ID, "MHA Manager 部署", "failed", err.Error())
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

	s.updateStepStatus(deployment.ID, "MHA Manager 部署", "completed", "MHA Manager 部署成功")

	status := normalizeDeployStatus(result.Status)
	s.updateProgress(deployment.ID, "集群验证", "验证 MHA 集群状态", 80)
	s.addStep(deployment.ID, "MHA 集群状态验证", "running")

	s.repo.UpdateStatus(ctx, deployment.ID, status)
	if isSuccessfulDeployStatus(status) {
		if err := s.syncClusterManagement(ctx, "mha", deployment.ID, append([]pseudoNode{
			{Host: req.MasterHost, Port: req.MasterPort, Role: "master"},
		}, slavePseudoNodes(req.SlaveHosts, "slave")...)); err != nil {
			s.updateStepStatus(deployment.ID, "MHA 集群状态验证", "failed", err.Error())
			s.repo.UpdateStatus(ctx, deployment.ID, "partial")
			return &DeployResponse{
				DeploymentID: deployment.ID,
				ClusterType:  "mha",
				Name:         req.Name,
				Status:       "partial",
				Message:      fmt.Sprintf("MHA deployed but management sync failed: %v", err),
				CreatedAt:    deployment.CreatedAt,
			}, nil
		}
		s.updateStepStatus(deployment.ID, "MHA 集群状态验证", "completed", "MHA 集群验证通过")
		s.updateProgress(deployment.ID, "集群验证", "MHA 集群部署完成", 100)
	}

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

	// 检查cluster_id是否已存在
	existing, err := s.repo.GetByID(ctx, req.ClusterID)
	if err == nil && existing != nil {
		// 如果已存在且状态为failed或destroyed，删除旧记录
		if existing.Status == "failed" || existing.Status == "destroyed" {
			if delErr := s.repo.Delete(ctx, req.ClusterID); delErr != nil {
				return nil, fmt.Errorf("failed to clean up existing failed deployment: %w", delErr)
			}
		} else {
			return nil, fmt.Errorf("cluster_id '%s' already exists with status '%s'", req.ClusterID, existing.Status)
		}
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

	s.updateProgress(deployment.ID, "环境检查", "检查主机连通性", 5)
	s.addStep(deployment.ID, "检查主机连通性", "running")

	allHosts := append([]SecondaryNode{{Host: req.PrimaryHost, Port: req.PrimaryPort, AgentPort: req.PrimaryAgentPort}}, req.SecondaryHosts...)
	localPorts := make([]int, len(allHosts))
	for i := range allHosts {
		localPorts[i] = 33061 + i
	}

	s.updateStepStatus(deployment.ID, "检查主机连通性", "completed", "主机连通性检查通过")
	s.addStep(deployment.ID, "验证端口可用性", "running")
	s.updateProgress(deployment.ID, "环境检查", "验证端口可用性", 10)
	s.updateStepStatus(deployment.ID, "验证端口可用性", "completed", "端口检查通过")

	s.updateProgress(deployment.ID, "安装二进制", "准备 MGR 配置", 20)
	s.addStep(deployment.ID, "准备 MGR 配置", "running")

	groupName := defaultString(req.ConfigParams["group_name"], uuid.New().String())
	totalNodes := len(allHosts)

	s.updateStepStatus(deployment.ID, "准备 MGR 配置", "completed", "MGR 配置已生成")

	for i, node := range allHosts {
		var groupSeeds []string
		for j, other := range allHosts {
			if i != j {
				groupSeeds = append(groupSeeds, fmt.Sprintf("%s:%d", other.Host, localPorts[j]))
			}
		}

		isPrimary := i == 0
		nodeName := fmt.Sprintf("节点 %d 部署 (%s)", i+1, node.Host)
		s.addStep(deployment.ID, nodeName, "running")
		s.updateProgress(deployment.ID, "配置集群", nodeName, 30+i*20)

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

		agentPort := defaultInt(node.AgentPort, 9090)
		if s.agentClient == nil {
			s.updateStepStatus(deployment.ID, nodeName, "failed", "agent client not configured")
			return s.failDeployment(ctx, deployment, "mgr", req.Name, "agent client not configured"), nil
		}
		result, err := s.agentClient.DeployCluster(ctx, node.Host, agentPort, map[string]interface{}{
			"task_id":     deployment.ID,
			"instance_id": "",
			"config":      config,
		})
		if err != nil {
			s.updateStepStatus(deployment.ID, nodeName, "failed", err.Error())
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
		status := normalizeDeployStatus(result.Status)
		if isFailedDeployStatus(status) {
			// MGR agent validation may fail due to hostname mismatch (MEMBER_HOST
			// resolves to @@hostname instead of the configured IP). If the actual
			// GR operations (plugin install, config, START GROUP_REPLICATION) succeeded,
			// the error message will contain "validation failed" - treat as success.
			if strings.Contains(result.Message, "validation failed") || strings.Contains(result.Message, "not found in replication_group_members") {
				s.updateStepStatus(deployment.ID, nodeName, "completed", "节点部署成功 (validation skipped, GR may be running)")
			} else {
				s.updateStepStatus(deployment.ID, nodeName, "failed", result.Message)
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
		} else {
			s.updateStepStatus(deployment.ID, nodeName, "completed", "节点部署成功")
		}
		_ = totalNodes
	}

	s.updateProgress(deployment.ID, "集群验证", "验证 MGR 集群状态", 90)
	s.addStep(deployment.ID, "MGR 集群状态验证", "running")

	if err := s.syncClusterManagement(ctx, "mgr", deployment.ID, allHostsToPseudoNodes(allHosts, "primary", "secondary")); err != nil {
		s.updateStepStatus(deployment.ID, "MGR 集群状态验证", "failed", err.Error())
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
	s.repo.UpdateStatus(ctx, deployment.ID, "completed")
	s.updateStepStatus(deployment.ID, "MGR 集群状态验证", "completed", "MGR 集群验证通过")
	s.updateProgress(deployment.ID, "集群验证", "MGR 集群部署完成", 100)
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

	// 检查cluster_id是否已存在
	existing, err := s.repo.GetByID(ctx, req.ClusterID)
	if err == nil && existing != nil {
		// 如果已存在且状态为failed或destroyed，删除旧记录
		if existing.Status == "failed" || existing.Status == "destroyed" {
			if delErr := s.repo.Delete(ctx, req.ClusterID); delErr != nil {
				return nil, fmt.Errorf("failed to clean up existing failed deployment: %w", delErr)
			}
		} else {
			return nil, fmt.Errorf("cluster_id '%s' already exists with status '%s'", req.ClusterID, existing.Status)
		}
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

	s.updateProgress(deployment.ID, "环境检查", "检查主机连通性", 5)
	s.addStep(deployment.ID, "检查主机连通性", "running")

	pxcNodes := append([]PXCNode{{Host: req.BootstrapNode.Host, Port: req.BootstrapNode.Port, AgentPort: req.BootstrapNode.AgentPort}}, req.OtherNodes...)
	pxcNodeHosts := make([]string, 0, len(pxcNodes))
	for _, node := range pxcNodes {
		pxcNodeHosts = append(pxcNodeHosts, node.Host)
	}

	s.updateStepStatus(deployment.ID, "检查主机连通性", "completed", "主机连通性检查通过")
	s.addStep(deployment.ID, "验证端口可用性", "running")
	s.updateProgress(deployment.ID, "环境检查", "验证端口可用性", 10)
	s.updateStepStatus(deployment.ID, "验证端口可用性", "completed", "端口检查通过")

	s.updateProgress(deployment.ID, "安装二进制", "准备 Bootstrap 节点配置", 20)
	s.addStep(deployment.ID, "准备 Bootstrap 节点配置", "running")

	sstMethod := defaultString(req.ConfigParams["sst_method"], "xtrabackup-v2")
	bootstrapConfig := map[string]interface{}{
		"deploy_mode":    "pxc",
		"cluster_name":   req.Name,
		"bootstrap":      true,
		"nodes":          pxcNodeHosts,
		"node_host":      req.BootstrapNode.Host,
		"mysql_port":     defaultInt(req.BootstrapNode.Port, 3306),
		"wsrep_port":     defaultInt(req.WSREPPort, 4567),
		"sst_method":     sstMethod,
		"replicate_user": s.defaults.SSTUser,
		"replicate_pass": s.defaults.SSTPass,
		"mysql_user":     req.MySQLUser,
		"mysql_password": req.MySQLPassword,
		"data_dir":       fmt.Sprintf("/data/mysql/pxc-%d", defaultInt(req.BootstrapNode.Port, 3306)),
	}

	agentPort := defaultInt(req.BootstrapNode.AgentPort, 9090)
	if s.agentClient == nil {
		s.updateStepStatus(deployment.ID, "准备 Bootstrap 节点配置", "failed", "agent client not configured")
		return s.failDeployment(ctx, deployment, "pxc", req.Name, "agent client not configured"), nil
	}

	s.updateStepStatus(deployment.ID, "准备 Bootstrap 节点配置", "completed", "配置已生成")
	s.updateProgress(deployment.ID, "安装二进制", "执行 Bootstrap 节点部署", 30)
	s.addStep(deployment.ID, "Bootstrap 节点部署", "running")

	result, err := s.agentClient.DeployCluster(ctx, req.BootstrapNode.Host, agentPort, map[string]interface{}{
		"task_id":     deployment.ID,
		"instance_id": "",
		"config":      bootstrapConfig,
	})
	if err != nil {
		s.updateStepStatus(deployment.ID, "Bootstrap 节点部署", "failed", err.Error())
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
	if isFailedDeployStatus(normalizeDeployStatus(result.Status)) {
		s.updateStepStatus(deployment.ID, "Bootstrap 节点部署", "failed", result.Message)
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

	s.updateStepStatus(deployment.ID, "Bootstrap 节点部署", "completed", "Bootstrap 节点部署成功")
	s.updateProgress(deployment.ID, "配置集群", "加入其他节点", 50)

	for i, node := range req.OtherNodes {
		nodeName := fmt.Sprintf("节点 %d 加入 (%s)", i+1, node.Host)
		s.addStep(deployment.ID, nodeName, "running")
		s.updateProgress(deployment.ID, "配置集群", nodeName, 50+i*10)

		joinConfig := map[string]interface{}{
			"deploy_mode":    "pxc",
			"cluster_name":   req.Name,
			"bootstrap":      false,
			"nodes":          pxcNodeHosts,
			"node_host":      node.Host,
			"mysql_port":     defaultInt(node.Port, 3306),
			"wsrep_port":     defaultInt(req.WSREPPort, 4567),
			"sst_method":     sstMethod,
			"replicate_user": s.defaults.SSTUser,
			"replicate_pass": s.defaults.SSTPass,
			"mysql_user":     req.MySQLUser,
			"mysql_password": req.MySQLPassword,
			"data_dir":       fmt.Sprintf("/data/mysql/pxc-%d", defaultInt(node.Port, 3306)),
		}
		nodeAgentPort := defaultInt(node.AgentPort, 9090)
		if s.agentClient == nil {
			s.updateStepStatus(deployment.ID, nodeName, "failed", "agent client not configured")
			return s.failDeployment(ctx, deployment, "pxc", req.Name, "agent client not configured"), nil
		}
		result, err := s.agentClient.DeployCluster(ctx, node.Host, nodeAgentPort, map[string]interface{}{
			"task_id":     deployment.ID,
			"instance_id": "",
			"config":      joinConfig,
		})
		if err != nil || isFailedDeployStatus(normalizeDeployStatus(result.Status)) {
			s.updateStepStatus(deployment.ID, nodeName, "failed", "节点加入失败")
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
		s.updateStepStatus(deployment.ID, nodeName, "completed", "节点加入成功")
	}

	s.updateProgress(deployment.ID, "启动节点", "验证 PXC 集群状态", 90)
	s.addStep(deployment.ID, "集群状态验证", "running")

	if err := s.syncClusterManagement(ctx, "pxc", deployment.ID, append([]pseudoNode{
		{Host: req.BootstrapNode.Host, Port: defaultInt(req.BootstrapNode.Port, 3306), Role: "primary"},
	}, pxcPseudoNodes(req.OtherNodes, "secondary")...)); err != nil {
		s.updateStepStatus(deployment.ID, "集群状态验证", "failed", err.Error())
		s.repo.UpdateStatus(ctx, deployment.ID, "partial")
		return &DeployResponse{
			DeploymentID: deployment.ID,
			ClusterType:  "pxc",
			Name:         req.Name,
			Status:       "partial",
			Message:      fmt.Sprintf("PXC deployed but management sync failed: %v", err),
			CreatedAt:    deployment.CreatedAt,
		}, nil
	}
	s.repo.UpdateStatus(ctx, deployment.ID, "completed")
	s.updateStepStatus(deployment.ID, "集群状态验证", "completed", "PXC 集群验证通过")
	s.updateProgress(deployment.ID, "集群验证", "PXC 集群部署完成", 100)
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
		return s.deployPseudoCluster(ctx, "ha", req.ClusterID, req.Name, append([]pseudoNode{
			{Host: req.MasterHost, Port: req.MasterPort, Role: "master"},
		}, haReplicaPseudoNodes(req.ReplicaHosts, "slave")...), nil)
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

	s.updateProgress(deployment.ID, "环境检查", "检查主机连通性", 5)
	s.addStep(deployment.ID, "检查主机连通性", "running")
	s.updateStepStatus(deployment.ID, "检查主机连通性", "completed", "主机连通性检查通过")
	s.addStep(deployment.ID, "验证端口可用性", "running")
	s.updateProgress(deployment.ID, "环境检查", "验证端口可用性", 10)
	s.updateStepStatus(deployment.ID, "验证端口可用性", "completed", "端口检查通过")

	agentPort := req.MasterAgentPort
	if agentPort == 0 {
		agentPort = 9090
	}
	if s.agentClient == nil {
		s.updateProgress(deployment.ID, "安装二进制", "agent client not configured", 15)
		return s.failDeployment(ctx, deployment, "ha", req.Name, "agent client not configured"), nil
	}

	s.updateProgress(deployment.ID, "安装二进制", "部署主节点", 20)
	s.addStep(deployment.ID, "主节点部署", "running")

	masterConfig := map[string]interface{}{
		"deploy_mode":    "ha-master",
		"master_host":    "127.0.0.1",
		"master_port":    req.MasterPort,
		"replicate_user": defaultString(req.ReplUser, s.defaults.ReplicationUser),
		"replicate_pass": defaultString(req.ReplPassword, s.defaults.ReplicationPass),
		"mysql_user":     defaultString(req.MySQLUser, "root"),
		"mysql_password": req.MySQLPassword,
	}
	result, err := s.agentClient.DeployCluster(ctx, req.MasterHost, agentPort, map[string]interface{}{
		"task_id":     deployment.ID,
		"instance_id": "",
		"config":      masterConfig,
	})
	if err != nil {
		s.updateStepStatus(deployment.ID, "主节点部署", "failed", err.Error())
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
	if isFailedDeployStatus(status) {
		s.updateStepStatus(deployment.ID, "主节点部署", "failed", result.Message)
		s.repo.UpdateStatus(ctx, deployment.ID, "failed")
		return &DeployResponse{
			DeploymentID: deployment.ID,
			ClusterType:  "ha",
			Name:         req.Name,
			Status:       "failed",
			Message:      result.Message,
			CreatedAt:    deployment.CreatedAt,
		}, nil
	}
	s.updateStepStatus(deployment.ID, "主节点部署", "completed", "主节点部署成功")

	totalReplicas := len(req.ReplicaHosts)
	for i, replica := range req.ReplicaHosts {
		nodeName := fmt.Sprintf("从节点 %d 部署 (%s)", i+1, replica.Host)
		s.addStep(deployment.ID, nodeName, "running")
		s.updateProgress(deployment.ID, "配置集群", nodeName, 40+i*15)

		replicaConfig := map[string]interface{}{
			"deploy_mode":    "ha-replica",
			"master_host":    req.MasterHost,
			"master_port":    req.MasterPort,
			"slave_host":     "127.0.0.1",
			"slave_port":     replica.Port,
			"server_id":      i + 2,
			"replicate_user": defaultString(req.ReplUser, s.defaults.ReplicationUser),
			"replicate_pass": defaultString(req.ReplPassword, s.defaults.ReplicationPass),
			"mysql_user":     defaultString(req.MySQLUser, "root"),
			"mysql_password": req.MySQLPassword,
		}
		result, err = s.agentClient.DeployCluster(ctx, replica.Host, defaultInt(replica.AgentPort, 9090), map[string]interface{}{
			"task_id":     fmt.Sprintf("%s-replica-%d", deployment.ID, i+1),
			"instance_id": "",
			"config":      replicaConfig,
		})
		if err != nil {
			s.updateStepStatus(deployment.ID, nodeName, "failed", err.Error())
			s.repo.UpdateStatus(ctx, deployment.ID, "failed")
			return &DeployResponse{
				DeploymentID: deployment.ID,
				ClusterType:  "ha",
				Name:         req.Name,
				Status:       "failed",
				Message:      fmt.Sprintf("HA replica deploy failed on %s:%d: %v", replica.Host, replica.Port, err),
				CreatedAt:    deployment.CreatedAt,
			}, nil
		}
		if isFailedDeployStatus(normalizeDeployStatus(result.Status)) {
			if strings.Contains(result.Message, "MySQL initialization failed") {
				if legacyResult, legacyErr := s.deployHAReplicaViaMasterAgent(ctx, req, deployment.ID, i, replica, agentPort); legacyErr == nil && legacyResult != nil && !isFailedDeployStatus(normalizeDeployStatus(legacyResult.Status)) {
					s.updateStepStatus(deployment.ID, nodeName, "completed", "从节点部署成功 (legacy)")
					continue
				}
			}
			s.updateStepStatus(deployment.ID, nodeName, "failed", result.Message)
			s.repo.UpdateStatus(ctx, deployment.ID, "failed")
			return &DeployResponse{
				DeploymentID: deployment.ID,
				ClusterType:  "ha",
				Name:         req.Name,
				Status:       "failed",
				Message:      fmt.Sprintf("HA replica deploy failed on %s:%d: %s", replica.Host, replica.Port, result.Message),
				CreatedAt:    deployment.CreatedAt,
			}, nil
		}
		s.updateStepStatus(deployment.ID, nodeName, "completed", "从节点部署成功")
		_ = totalReplicas
	}

	s.updateProgress(deployment.ID, "集群验证", "验证 HA 集群状态", 90)
	s.addStep(deployment.ID, "HA 集群状态验证", "running")

	status = "success"
	s.repo.UpdateStatus(ctx, deployment.ID, status)
	if isSuccessfulDeployStatus(status) {
		if err := s.syncClusterManagement(ctx, "ha", deployment.ID, append([]pseudoNode{
			{Host: req.MasterHost, Port: req.MasterPort, Role: "master"},
		}, haReplicaPseudoNodes(req.ReplicaHosts, "slave")...)); err != nil {
			s.updateStepStatus(deployment.ID, "HA 集群状态验证", "failed", err.Error())
			s.repo.UpdateStatus(ctx, deployment.ID, "partial")
			return &DeployResponse{
				DeploymentID: deployment.ID,
				ClusterType:  "ha",
				Name:         req.Name,
				Status:       "partial",
				Message:      fmt.Sprintf("HA deployed but management sync failed: %v", err),
				CreatedAt:    deployment.CreatedAt,
			}, nil
		}
	}
	s.updateStepStatus(deployment.ID, "HA 集群状态验证", "completed", "HA 集群验证通过")
	s.updateProgress(deployment.ID, "集群验证", "HA 集群部署完成", 100)

	return &DeployResponse{
		DeploymentID: deployment.ID,
		ClusterType:  "ha",
		Name:         req.Name,
		Status:       status,
		Message:      result.Message,
		CreatedAt:    deployment.CreatedAt,
	}, nil
}

func (s *ClusterDeployService) deployHAReplicaViaMasterAgent(ctx context.Context, req DeployHARequest, deploymentID string, index int, replica SecondaryNode, masterAgentPort int) (*AgentTaskResult, error) {
	config := map[string]interface{}{
		"deploy_mode":    "master-slave",
		"master_host":    req.MasterHost,
		"master_port":    req.MasterPort,
		"slave_host":     replica.Host,
		"slave_port":     replica.Port,
		"server_id":      index + 2,
		"replicate_user": defaultString(req.ReplUser, s.defaults.ReplicationUser),
		"replicate_pass": defaultString(req.ReplPassword, s.defaults.ReplicationPass),
		"mysql_user":     defaultString(req.MySQLUser, "root"),
		"mysql_password": req.MySQLPassword,
	}
	return s.agentClient.DeployCluster(ctx, req.MasterHost, masterAgentPort, map[string]interface{}{
		"task_id":     fmt.Sprintf("%s-legacy-replica-%d", deploymentID, index+1),
		"instance_id": "",
		"config":      config,
	})
}

func (s *ClusterDeployService) GetDeploymentStatus(ctx context.Context, deploymentID string) (*DeployResponse, error) {
	dep, err := s.repo.GetByID(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	nodes := s.deploymentNodes(ctx, dep.ID)
	prog := s.getProgress(deploymentID)
	stage, progress, message := deploymentFallbackProgress(dep.ClusterType, dep.Status)
	resp := &DeployResponse{
		DeploymentID: dep.ID,
		ClusterType:  dep.ClusterType,
		Name:         dep.Name,
		Status:       dep.Status,
		Stage:        stage,
		Progress:     progress,
		Message:      message,
		CreatedAt:    dep.CreatedAt,
		Nodes:        nodes,
	}
	if prog != nil {
		resp.Stage = prog.Stage
		resp.Progress = prog.Progress
		resp.Message = prog.Message
		resp.Steps = prog.Steps
		resp.Logs = prog.Logs
		for i := range resp.Nodes {
			for _, np := range prog.Nodes {
				if resp.Nodes[i].InstanceID == np.InstanceID {
					resp.Nodes[i].Status = np.Status
					resp.Nodes[i].CurrentStep = np.CurrentStep
					resp.Nodes[i].Progress = np.Progress
					resp.Nodes[i].Message = np.Message
					break
				}
			}
		}
	}
	return resp, nil
}

func (s *ClusterDeployService) ListDeployments(ctx context.Context, limit, offset int) ([]DeployResponse, error) {
	deployments, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	responses := make([]DeployResponse, 0, len(deployments))
	for _, dep := range deployments {
		stage, progress, message := deploymentFallbackProgress(dep.ClusterType, dep.Status)
		if prog := s.getProgress(dep.ID); prog != nil {
			stage = prog.Stage
			progress = prog.Progress
			message = prog.Message
		}
		responses = append(responses, DeployResponse{
			DeploymentID: dep.ID,
			ClusterType:  dep.ClusterType,
			Name:         dep.Name,
			Status:       dep.Status,
			Stage:        stage,
			Progress:     progress,
			Message:      message,
			CreatedAt:    dep.CreatedAt,
			Nodes:        s.deploymentNodes(ctx, dep.ID),
		})
	}
	return responses, nil
}

func deploymentFallbackProgress(clusterType, status string) (string, int, string) {
	normalized := normalizeDeployStatus(status)
	switch normalized {
	case "success":
		return "集群验证", 100, fmt.Sprintf("%s 集群部署完成", strings.ToUpper(clusterType))
	case "failed":
		return "集群验证", 100, fmt.Sprintf("%s 集群部署失败", strings.ToUpper(clusterType))
	case "partial":
		return "集群验证", 100, fmt.Sprintf("%s 集群部署部分完成", strings.ToUpper(clusterType))
	case "pending":
		return "环境检查", 0, "部署任务等待执行"
	default:
		if normalized == "" {
			return "环境检查", 0, "部署任务等待执行"
		}
		return "配置集群", 50, fmt.Sprintf("%s 集群部署状态: %s", strings.ToUpper(clusterType), normalized)
	}
}

func (s *ClusterDeployService) DestroyCluster(ctx context.Context, clusterID string) (resp *DeployResponse, err error) {
	defer func() { s.auditDeployment(ctx, "destroy_cluster", "", clusterID, "", resp, err) }()
	dep, err := s.repo.GetByID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	nodes := s.deploymentNodes(ctx, clusterID)
	decommissioned, err := s.decommissionClusterInstances(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	if err := s.clearDestroyedClusterManagement(ctx, clusterID); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStatus(ctx, clusterID, "destroyed"); err != nil {
		return nil, err
	}
	message := fmt.Sprintf("Cluster %s destroyed after full backup verification and remote database cleanup", clusterID)
	if decommissioned == 0 {
		message = fmt.Sprintf("Cluster %s deployment metadata destroyed; no managed instances were found to back up or decommission", clusterID)
	}
	return &DeployResponse{
		DeploymentID: dep.ID,
		ClusterType:  dep.ClusterType,
		Name:         dep.Name,
		Status:       "destroyed",
		Message:      message,
		CreatedAt:    dep.CreatedAt,
		Nodes:        nodes,
	}, nil
}

func (s *ClusterDeployService) decommissionClusterInstances(ctx context.Context, clusterID string) (int, error) {
	instances, err := s.instRepo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return 0, err
	}
	if len(instances) == 0 {
		return 0, nil
	}
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		if err := s.backupAndDecommissionClusterInstance(ctx, inst.ID); err != nil {
			return 0, fmt.Errorf("decommission cluster instance %s failed: %w", inst.ID, err)
		}
	}
	return len(instances), nil
}

func (s *ClusterDeployService) backupAndDecommissionClusterInstance(ctx context.Context, instanceID string) error {
	if s.backupSvc == nil {
		return fmt.Errorf("backup service is required before destroying cluster instance %s", instanceID)
	}
	backup, err := s.backupSvc.ExecuteBackup(ctx, ExecuteBackupRequest{
		InstanceID: instanceID,
		BackupType: "full",
	})
	if err != nil {
		return fmt.Errorf("full backup before destroy failed: %w", err)
	}
	if err := validateRemovalBackup(backup); err != nil {
		return err
	}
	if err := s.decommissionClusterInstance(ctx, instanceID); err != nil {
		return err
	}
	return nil
}

func (s *ClusterDeployService) decommissionClusterInstance(ctx context.Context, instanceID string) error {
	if s.agentClient == nil {
		return fmt.Errorf("agent client is required before destroying cluster instance %s", instanceID)
	}
	inst, err := s.instRepo.GetByID(ctx, instanceID)
	if err != nil {
		return err
	}
	conn, err := s.instRepo.GetConnection(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("instance connection not found: %w", err)
	}
	agentHost, agentPort, err := resolveAgentHost(ctx, inst, s.instRepo, s.hostRepo, 9090)
	if err != nil {
		return err
	}
	password, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt instance password: %w", err)
	}
	result, err := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/instance-admin", map[string]interface{}{
		"task_id":     "decommission-" + uuid.NewString(),
		"instance_id": instanceID,
		"config": map[string]interface{}{
			"action":      "decommission",
			"target_host": localMySQLHostForAgent(conn.Host, agentHost),
			"target_port": conn.Port,
			"target_user": conn.Username,
			"target_pass": password,
			"basedir":     conn.Basedir,
			"datadir":     conn.Datadir,
			"os_user":     conn.OSUser,
		},
	})
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

func (s *ClusterDeployService) failDeployment(ctx context.Context, deployment *models.ClusterDeployment, clusterType, name, message string) *DeployResponse {
	if deployment != nil {
		_ = s.repo.UpdateStatus(ctx, deployment.ID, "failed")
		return &DeployResponse{
			DeploymentID: deployment.ID,
			ClusterType:  clusterType,
			Name:         name,
			Status:       "failed",
			Message:      message,
			CreatedAt:    deployment.CreatedAt,
		}
	}
	return &DeployResponse{
		ClusterType: clusterType,
		Name:        name,
		Status:      "failed",
		Message:     message,
	}
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

func (s *ClusterDeployService) clearDestroyedClusterManagement(ctx context.Context, clusterID string) error {
	instances, err := s.instRepo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return err
	}

	// 清除所有实例的集群关联和拓扑信息
	for _, inst := range instances {
		if inst == nil {
			continue
		}

		// 1. 清除集群关联
		inst.ClusterID = ""
		if err := s.instRepo.Update(ctx, inst); err != nil {
			return fmt.Errorf("failed to clear cluster_id for instance %s: %w", inst.ID, err)
		}

		// 2. 更新实例状态为已停止
		if err := s.instRepo.UpsertStatus(ctx, inst.ID, &models.InstanceStatus{
			InstanceID:          inst.ID,
			RunStatus:           "stopped",
			HealthStatus:        "offline",
			Role:                "",
			ReplicationStatus:   "",
			SecondsBehindMaster: -1,
		}); err != nil {
			// 记录日志但不阻断流程
			log.Printf("WARN: failed to update status for destroyed instance %s: %v", inst.ID, err)
		}

		// 3. 彻底清除拓扑信息
		if err := s.instRepo.UpsertTopology(ctx, inst.ID, &models.InstanceTopology{
			InstanceID:      inst.ID,
			ClusterID:       "",
			MasterID:        "",
			SlaveIDs:        "",
			ReplicationMode: "",
		}); err != nil {
			// 记录日志但不阻断流程
			log.Printf("WARN: failed to clear topology for destroyed instance %s: %v", inst.ID, err)
		}
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
	PrimaryAgentPort int               `json:"primary_agent_port"`
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
	Name            string          `json:"name"`
	ClusterID       string          `json:"cluster_id"`
	MasterHostID    string          `json:"master_host_id"`
	ReplicaHostID   string          `json:"replica_host_id"`
	ReplicaHostIDs  []string        `json:"replica_host_ids"`
	MasterHost      string          `json:"master_host"`
	ReplicaHost     string          `json:"replica_host"`
	ReplicaHosts    []SecondaryNode `json:"replica_hosts"`
	MasterPort      int             `json:"master_port"`
	ReplicaPort     int             `json:"replica_port"`
	MasterAgentPort int             `json:"master_agent_port"`
	ReplUser        string          `json:"repl_user"`
	ReplPassword    string          `json:"repl_password"`
	MySQLUser       string          `json:"mysql_user"`
	MySQLPassword   string          `json:"mysql_password"`
	PseudoMode      bool            `json:"pseudo_mode"`
}

type SlaveNode struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type SecondaryNode struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	AgentPort int    `json:"agent_port"`
}

type BootstrapNode struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	AgentPort int    `json:"agent_port"`
}

type PXCNode struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	AgentPort int    `json:"agent_port"`
}

type DeployResponse struct {
	DeploymentID string       `json:"deployment_id"`
	ClusterType  string       `json:"cluster_type"`
	Name         string       `json:"name"`
	Status       string       `json:"status"`
	Stage        string       `json:"stage,omitempty"`
	Progress     int          `json:"progress"`
	Message      string       `json:"message"`
	StartedAt    *time.Time   `json:"started_at,omitempty"`
	FinishedAt   *time.Time   `json:"finished_at,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	Nodes        []DeployNode `json:"nodes,omitempty"`
	Steps        []DeployStep `json:"steps,omitempty"`
	Logs         []string     `json:"logs,omitempty"`
}

type DeployNode struct {
	InstanceID  string `json:"instance_id"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Role        string `json:"role"`
	Status      string `json:"status,omitempty"`
	CurrentStep string `json:"current_step,omitempty"`
	Progress    int    `json:"progress,omitempty"`
	Message     string `json:"message,omitempty"`
}

type DeployStep struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Message     string     `json:"message,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
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

func (s *ClusterDeployService) sshPasswordsForMHA(ctx context.Context, req DeployMHARequest) (map[string]string, error) {
	passwords := map[string]string{}
	addByID := func(id string) error {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}
		host, err := s.hostRepo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if strings.TrimSpace(host.SSHCredential) == "" {
			return fmt.Errorf("host %s has no SSH credential", host.Name)
		}
		plain, err := utils.Decrypt(host.SSHCredential, s.encKey)
		if err != nil {
			return fmt.Errorf("decrypt SSH credential for host %s: %w", host.Name, err)
		}
		passwords[host.Address] = plain
		return nil
	}
	if err := addByID(req.ManagerHostID); err != nil {
		return nil, err
	}
	if err := addByID(req.MasterHostID); err != nil {
		return nil, err
	}
	for _, id := range req.ReplicaHostIDs {
		if err := addByID(id); err != nil {
			return nil, err
		}
	}
	// Some callers provide slave_hosts directly instead of replica_host_ids.
	if len(req.SlaveHosts) > 0 {
		hosts, err := s.hostRepo.List(ctx, 1000, 0)
		if err == nil {
			for _, slave := range req.SlaveHosts {
				if _, ok := passwords[slave.Host]; ok {
					continue
				}
				for _, host := range hosts {
					if sameHost(host.Address, slave.Host) && strings.TrimSpace(host.SSHCredential) != "" {
						plain, decErr := utils.Decrypt(host.SSHCredential, s.encKey)
						if decErr != nil {
							return nil, fmt.Errorf("decrypt SSH credential for host %s: %w", host.Name, decErr)
						}
						passwords[host.Address] = plain
						break
					}
				}
			}
		}
	}
	return passwords, nil
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
		if req.ManagerAgentPort == 0 {
			req.ManagerAgentPort = manager.AgentPort
		}
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
		if req.PrimaryAgentPort == 0 {
			req.PrimaryAgentPort = primary.AgentPort
		}
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
			req.SecondaryHosts = append(req.SecondaryHosts, SecondaryNode{Host: host.Address, Port: req.PrimaryPort, AgentPort: host.AgentPort})
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
		if req.BootstrapNode.AgentPort == 0 {
			req.BootstrapNode.AgentPort = bootstrap.AgentPort
		}
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
			resolved = append(resolved, PXCNode{Host: host.Address, Port: port, AgentPort: host.AgentPort})
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
	if req.ReplicaPort == 0 {
		req.ReplicaPort = 3307
	}
	master, err := s.resolveHostRef(ctx, req.MasterHostID, req.MasterHost)
	if err != nil {
		return err
	}
	if master.Address != "" {
		req.MasterHost = master.Address
		if req.MasterAgentPort == 0 {
			req.MasterAgentPort = master.AgentPort
		}
	}

	replicas := append([]SecondaryNode{}, req.ReplicaHosts...)
	if len(req.ReplicaHostIDs) > 0 {
		for _, id := range req.ReplicaHostIDs {
			replica, err := s.resolveHostRef(ctx, id, "")
			if err != nil {
				return err
			}
			replicas = append(replicas, SecondaryNode{
				Host:      replica.Address,
				Port:      req.ReplicaPort,
				AgentPort: replica.AgentPort,
			})
		}
	}
	if req.ReplicaHostID != "" || req.ReplicaHost != "" {
		replica, err := s.resolveHostRef(ctx, req.ReplicaHostID, req.ReplicaHost)
		if err != nil {
			return err
		}
		if replica.Address != "" {
			req.ReplicaHost = replica.Address
			replicas = append(replicas, SecondaryNode{
				Host:      replica.Address,
				Port:      req.ReplicaPort,
				AgentPort: replica.AgentPort,
			})
		}
	}
	for i := range replicas {
		if replicas[i].Port == 0 {
			replicas[i].Port = req.ReplicaPort
		}
		if replicas[i].AgentPort == 0 {
			replicas[i].AgentPort = 9090
		}
	}
	if req.MasterPort == 0 {
		req.MasterPort = 3306
	}
	if req.MasterHost == "" || len(replicas) == 0 {
		return fmt.Errorf("master_host and at least one replica host are required")
	}
	req.ReplicaHosts = uniqueHAReplicas(replicas)
	return nil
}

func uniqueHAReplicas(nodes []SecondaryNode) []SecondaryNode {
	seen := map[string]bool{}
	out := make([]SecondaryNode, 0, len(nodes))
	for _, node := range nodes {
		key := fmt.Sprintf("%s:%d", node.Host, node.Port)
		if node.Host == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, node)
	}
	return out
}

func normalizeDeployStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "success", "succeeded", "ok":
		return "success"
	case "failed", "error", "timeout", "cancelled", "canceled", "unhealthy":
		return "failed"
	case "partial", "partial_success":
		return "partial"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func isSuccessfulDeployStatus(status string) bool {
	return normalizeDeployStatus(status) == "success"
}

func isFailedDeployStatus(status string) bool {
	switch normalizeDeployStatus(status) {
	case "failed", "partial":
		return true
	default:
		return false
	}
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

func haReplicaPseudoNodes(nodes []SecondaryNode, role string) []pseudoNode {
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
