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
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/config"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type ClusterDeployService struct {
	repo        *repositories.ClusterDeployRepository
	nodeRepo    *repositories.ClusterDeployNodeRepository
	hostRepo    *repositories.HostRepository
	instRepo    *repositories.InstanceRepository
	agentClient *AgentClient
	hostService *HostService
	backupSvc   *BackupService
	auditSvc    *AuditService
	vault       *CredentialVault
	versions    *VersionCatalog
	defaults    config.ClusterDefaults
	encKey      string
	progressMu  sync.RWMutex
	progress    map[string]*DeploymentProgress
}

type ClusterDestroyOperationError struct {
	ClusterID  string
	InstanceID string
	Stage      string
	Err        error
}

func (e *ClusterDestroyOperationError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{"destroy cluster failed"}
	if e.ClusterID != "" {
		parts = append(parts, "cluster="+e.ClusterID)
	}
	if e.InstanceID != "" {
		parts = append(parts, "instance="+e.InstanceID)
	}
	if e.Stage != "" {
		parts = append(parts, "stage="+e.Stage)
	}
	if e.Err != nil {
		parts = append(parts, "error="+e.Err.Error())
	}
	return strings.Join(parts, " ")
}

func (e *ClusterDestroyOperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
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
	Host          string
	Port          int
	Role          string
	HostID        string
	DataDir       string
	Basedir       string
	MySQLUser     string
	MySQLPassword string
}

func NewClusterDeployService(
	repo *repositories.ClusterDeployRepository,
	nodeRepo *repositories.ClusterDeployNodeRepository,
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
		nodeRepo:    nodeRepo,
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

func (s *ClusterDeployService) SetCredentialVault(vault *CredentialVault) {
	s.vault = vault
}

func (s *ClusterDeployService) SetVersionCatalog(catalog *VersionCatalog) {
	s.versions = catalog
}

func (s *ClusterDeployService) SetHostService(hs *HostService) {
	s.hostService = hs
}

// ensureAgentReady checks if the agent on the given host is reachable and accepts
// our token. If not, it attempts to reinstall the agent via SSH (requires HostService).
func (s *ClusterDeployService) ensureAgentReady(ctx context.Context, host string, agentPort int, hostID string) error {
	if agentPort == 0 {
		agentPort = 9090
	}
	if s.agentClient == nil {
		return fmt.Errorf("agent client not configured")
	}
	// Quick probe: try a lightweight agent call
	_, err := s.agentClient.callAgent(ctx, host, agentPort, "/agent/tasks/health-check", map[string]interface{}{
		"task_id": "pre-flight-" + host,
	})
	if err == nil {
		return nil // agent is reachable and accepts our token
	}
	if !strings.Contains(err.Error(), "401") {
		return nil // agent is reachable but returned other error (still valid token)
	}
	// Agent returned 401 — token mismatch. Try to fix via SSH.
	if s.hostService == nil {
		return fmt.Errorf("agent token mismatch on %s and no HostService available for auto-fix", host)
	}
	if hostID == "" {
		return fmt.Errorf("agent token mismatch on %s: host_id unknown, cannot auto-fix", host)
	}
	log.Printf("WARN: agent token mismatch on %s, attempting SSH reinstall", host)
	result, installErr := s.hostService.AgentAction(ctx, hostID, HostAgentActionRequest{Action: "install", AgentPort: agentPort})
	if installErr != nil {
		return fmt.Errorf("agent token mismatch on %s, SSH reinstall failed: %v", host, installErr)
	}
	if result != nil && result.Status == "failed" {
		return fmt.Errorf("agent token mismatch on %s, SSH reinstall returned: %s", host, result.Message)
	}
	// Wait for agent to restart
	time.Sleep(5 * time.Second)
	// Verify
	_, verifyErr := s.agentClient.callAgent(ctx, host, agentPort, "/agent/tasks/health-check", map[string]interface{}{
		"task_id": "post-fix-" + host,
	})
	if verifyErr != nil && strings.Contains(verifyErr.Error(), "401") {
		return fmt.Errorf("agent token still mismatched on %s after reinstall", host)
	}
	return nil
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

func (s *ClusterDeployService) syncNodesFromPlan(ctx context.Context, deploymentID string, plan *ClusterDeployPlan) {
	if s.nodeRepo == nil || plan == nil {
		return
	}
	nodes := make([]models.ClusterDeployNode, 0, len(plan.Nodes))
	for _, pn := range plan.Nodes {
		role := pn.Role
		if role == "" {
			role = "unknown"
		}
		nodes = append(nodes, models.ClusterDeployNode{
			DeploymentID: deploymentID,
			InstanceID:   pn.ID,
			Host:         pn.Host,
			MySQLPort:    pn.MySQLPort,
			Role:         role,
			Status:       "pending",
		})
	}
	if err := s.nodeRepo.BatchCreate(ctx, nodes); err != nil {
		log.Printf("WARN: failed to persist deploy nodes for %s: %v", deploymentID, err)
	}
}

func (s *ClusterDeployService) loadNodesFromDB(ctx context.Context, deploymentID string) []models.ClusterDeployNode {
	if s.nodeRepo == nil {
		return nil
	}
	nodes, err := s.nodeRepo.ListByDeploymentID(ctx, deploymentID)
	if err != nil {
		log.Printf("WARN: failed to load deploy nodes from DB for %s: %v", deploymentID, err)
		return nil
	}
	return nodes
}

func (s *ClusterDeployService) ExecuteClusterDeployPlan(ctx context.Context, plan *ClusterDeployPlan, req UniversalClusterDeployRequest) (finalResp *DeployResponse, finalErr error) {
	// This execution engine supports all modes:
	// - pseudo: metadata-only (no agent calls)
	// - validate_only: plan only, no execution
	// - real: full deployment, calling agent for each deploy step
	isReal := plan.Mode == DeployModeReal

	clusterID := plan.DeploymentID
	clusterType := plan.ClusterType
	name := req.Name
	if name == "" {
		name = clusterID
	}
	now := time.Now()

	// Ensure audit is always recorded on exit
	defer func() {
		if finalResp != nil || finalErr != nil {
			s.auditDeployment(ctx, "deploy_"+plan.ClusterType+"_cluster", plan.ClusterType, plan.DeploymentID, req.Name, finalResp, finalErr)
		}
	}()

	// Clean up any previous failed deployment
	s.clearClusterManagement(ctx, clusterID)
	_ = s.repo.Delete(ctx, clusterID) // ignore error if not found

	// Create deployment record with plan/request persistence
	dep := &models.ClusterDeployment{
		ID:          clusterID,
		ClusterType: clusterType,
		Name:        name,
		Status:      "running",
		StartedAt:   &now,
		RequestJSON: marshalString(req),
		PlanJSON:    marshalString(plan),
		CustomJSON:  marshalString(req.Custom),
		CreatedAt:   now,
	}
	if err := s.repo.Create(ctx, dep); err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	// Sync nodes from plan into cluster_deploy_nodes table
	s.syncNodesFromPlan(ctx, clusterID, plan)

	// In-memory progress tracking
	s.updateProgress(clusterID, "初始化", "部署计划已创建", 5)

	// Pre-flight: ensure agents on all nodes are reachable with correct token.
	if isReal {
		for _, node := range plan.Nodes {
			if err := s.ensureAgentReady(ctx, node.Host, node.AgentPort, node.HostID); err != nil {
				s.addStep(clusterID, "Pre-flight agent check", "running")
				errMsg := fmt.Sprintf("agent pre-flight failed on %s: %v", node.Host, err)
				s.updateStepStatus(clusterID, "Pre-flight agent check", "failed", errMsg)
				s.repo.UpdateStatusWithError(ctx, clusterID, "failed", errMsg)
				finish := time.Now()
				return s.buildPartialResponse(ctx, clusterID, clusterType, name, dep, errMsg, &finish), nil
			}
		}
	}

	// Execute each step in order
	totalSteps := len(plan.Steps)

	for i, step := range plan.Steps {
		progressPct := 5 + (i * 85 / totalSteps)
		s.updateProgress(clusterID, step.Name, fmt.Sprintf("Step %d/%d: %s", i+1, totalSteps, step.Name), progressPct)
		s.addStep(clusterID, step.Name, "running")

		switch step.Type {
		case "validate", "sync":
			// Backend-side steps: mark completed directly
			s.updateStepStatus(clusterID, step.Name, "completed", fmt.Sprintf("%s 已完成", step.Name))

		case "bootstrap", "join", "deploy", "configure":
			if step.TargetNode == "" {
				s.updateStepStatus(clusterID, step.Name, "completed", fmt.Sprintf("%s 已完成", step.Name))
				continue
			}
			// Find the target node for this step
			targetNode := findPlanNode(plan.Nodes, step.TargetNode)
			if targetNode == nil {
				errMsg := fmt.Sprintf("target node %q not found for step %s", step.TargetNode, step.ID)
				s.updateStepStatus(clusterID, step.Name, "failed", errMsg)
				s.repo.UpdateStatusWithError(ctx, clusterID, "failed", errMsg)
				finish := time.Now()
				return s.buildPartialResponse(ctx, clusterID, clusterType, name, dep, errMsg, &finish), nil
			}

			if isReal {
				// Real mode: call the agent
				if s.agentClient == nil {
					errMsg := "agent client not configured"
					s.updateStepStatus(clusterID, step.Name, "failed", errMsg)
					s.repo.UpdateStatusWithError(ctx, clusterID, "failed", errMsg)
					finish := time.Now()
					return s.buildPartialResponse(ctx, clusterID, clusterType, name, dep, errMsg, &finish), nil
				}

				// Build agent payload from step.Config
				agentPayload := map[string]interface{}{
					"task_id":     clusterID,
					"instance_id": "",
					"config":      step.Config,
				}

				nodeMsg := fmt.Sprintf("正在部署 %s 于 %s:%d", targetNode.Role, targetNode.Host, targetNode.AgentPort)
				s.updateProgress(clusterID, step.Name, nodeMsg, progressPct)

				result, err := s.agentClient.DeployCluster(ctx, targetNode.Host, targetNode.AgentPort, agentPayload)
				if err != nil {
					errMsg := fmt.Sprintf("agent call failed on %s: %v", targetNode.Host, err)
					s.updateStepStatus(clusterID, step.Name, "failed", errMsg)
					s.repo.UpdateStatusWithError(ctx, clusterID, "failed", errMsg)
					finish := time.Now()
					return s.buildPartialResponse(ctx, clusterID, clusterType, name, dep, errMsg, &finish), nil
				}

				if isFailedDeployStatus(normalizeDeployStatus(result.Status)) {
					errMsg := fmt.Sprintf("deploy failed on %s: %s", targetNode.Host, result.Message)
					s.updateStepStatus(clusterID, step.Name, "failed", result.Message)
					// Mark whole deployment as failed for critical steps
					// Secondary/join failures are marked as partial (some nodes may still be OK)
					isCritical := step.Type == "bootstrap" || step.Type == "deploy" || step.Type == "configure"
					if isCritical {
						s.repo.UpdateStatusWithError(ctx, clusterID, "failed", errMsg)
					} else {
						s.repo.UpdateStatusWithError(ctx, clusterID, "partial", errMsg)
					}
					finish := time.Now()
					return s.buildPartialResponse(ctx, clusterID, clusterType, name, dep, errMsg, &finish), nil
				}

				nodeReadyMsg := fmt.Sprintf("节点 %s (%s) 部署成功", targetNode.Host, targetNode.Role)
				s.updateStepStatus(clusterID, step.Name, "completed", nodeReadyMsg)
			} else {
				// Pseudo mode: mark as completed without calling agent
				nodeMsg := fmt.Sprintf("节点 %s (%s) 步骤完成（演练模式）", targetNode.Host, targetNode.Role)
				s.updateStepStatus(clusterID, step.Name, "completed", nodeMsg)
			}

		case "verify":
			s.updateStepStatus(clusterID, step.Name, "completed", "验证完成")

		default:
			s.updateStepStatus(clusterID, step.Name, "completed", "Step completed")
		}
	}

	// Sync cluster management metadata (register instances, topology, etc.)
	if err := s.syncClusterManagementFromPlan(ctx, clusterType, clusterID, plan.Nodes, req); err != nil {
		errMsg := fmt.Sprintf("元数据同步失败: %v", err)
		s.repo.UpdateStatus(ctx, clusterID, "partial")
		finish := time.Now()
		partialResp := &DeployResponse{
			DeploymentID: clusterID,
			ClusterType:  clusterType,
			Name:         name,
			Status:       "partial",
			Message:      errMsg,
			ErrorMessage: errMsg,
			StartedAt:    dep.StartedAt,
			FinishedAt:   &finish,
			CreatedAt:    dep.CreatedAt,
			Steps:        s.getProgress(clusterID).Steps,
		}
		finalResp = partialResp
		finalErr = err
		return partialResp, nil
	}

	// Success
	s.repo.UpdateStatus(ctx, clusterID, "completed")
	// Write back cluster base info (cluster_id, arch, nodes, mysql_version, config_json)
	nodeCount := len(plan.Nodes)
	mysqlVer := req.MySQL.Version
	if mysqlVer == "" {
		mysqlVer = "8.0"
	}
	_ = s.repo.UpdateClusterMeta(ctx, clusterID, nodeCount, mysqlVer, dep.RequestJSON)
	if s.nodeRepo != nil {
		_ = s.nodeRepo.UpdateStatusByDeploymentID(ctx, clusterID, "completed", "部署完成")
	}
	s.updateProgress(clusterID, "集群验证", "集群部署完成", 100)
	finish := time.Now()

	nodes := make([]DeployNode, 0, len(plan.Nodes))
	for _, pn := range plan.Nodes {
		nodes = append(nodes, DeployNode{
			InstanceID: pn.ID,
			Host:       pn.Host,
			Port:       pn.MySQLPort,
			Role:       pn.Role,
			Status:     "completed",
		})
	}

	resp := &DeployResponse{
		DeploymentID:  clusterID,
		ClusterType:   clusterType,
		Name:          name,
		Status:        "success",
		Message:       "集群部署成功",
		MySQLUser:     req.MySQL.User,
		MySQLPassword: req.MySQL.Password,
		StartedAt:     dep.StartedAt,
		FinishedAt:    &finish,
		CreatedAt:     dep.CreatedAt,
		Nodes:         nodes,
	}
	if prog := s.getProgress(clusterID); prog != nil {
		resp.Steps = prog.Steps
	}
	finalResp = resp
	return resp, nil
}

// syncPseudoClusterManagement syncs cluster metadata for plan-based pseudo deployments.
func (s *ClusterDeployService) syncPseudoClusterManagement(ctx context.Context, clusterType, clusterID string, planNodes []PlanNode) error {
	return s.syncClusterManagementFromPlan(ctx, clusterType, clusterID, planNodes, UniversalClusterDeployRequest{})
}

func (s *ClusterDeployService) syncClusterManagementFromPlan(ctx context.Context, clusterType, clusterID string, planNodes []PlanNode, req UniversalClusterDeployRequest) error {
	nodes := make([]pseudoNode, 0, len(planNodes))
	for _, pn := range planNodes {
		if !isMySQLDeployRole(clusterType, pn.Role) {
			continue
		}
		nodes = append(nodes, pseudoNode{
			Host: pn.Host, Port: pn.MySQLPort, Role: pn.Role, HostID: pn.HostID, DataDir: pn.DataDir, Basedir: pn.Basedir,
			MySQLUser: req.MySQL.User, MySQLPassword: req.MySQL.Password,
		})
	}
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes to sync")
	}
	return s.syncClusterManagement(ctx, clusterType, clusterID, nodes)
}

func isMySQLDeployRole(clusterType, role string) bool {
	switch clusterType {
	case ClusterTypeMHA:
		// MHA: include all three roles so manager/master/replica all appear in topology
		return role == "manager" || role == "master" || role == "replica"
	default:
		return role != "manager"
	}
}

type PreCheckResult struct {
	HostID  string         `json:"host_id"`
	Host    string         `json:"host"`
	Status  string         `json:"status"` // pass, warn, fail
	Message string         `json:"message"`
	Details []PreCheckItem `json:"details"`
}

type PreCheckItem struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Value   string `json:"value,omitempty"`
	Message string `json:"message,omitempty"`
}

type ClusterDeployCheckNode struct {
	HostID    string `json:"host_id"`
	Host      string `json:"host,omitempty"`
	Role      string `json:"role,omitempty"`
	MySQLPort int    `json:"mysql_port,omitempty"`
	DataDir   string `json:"data_dir,omitempty"`
}

func (s *ClusterDeployService) PreCheck(ctx context.Context, hostIDs []string, nodes []ClusterDeployCheckNode) ([]PreCheckResult, error) {
	if s.hostRepo == nil {
		return nil, fmt.Errorf("host repository not configured")
	}
	nodeByHostID := map[string][]ClusterDeployCheckNode{}
	for _, node := range nodes {
		if node.MySQLPort == 0 {
			node.MySQLPort = 3306
		}
		if node.HostID != "" {
			nodeByHostID[node.HostID] = append(nodeByHostID[node.HostID], node)
			if !containsString(hostIDs, node.HostID) {
				hostIDs = append(hostIDs, node.HostID)
			}
		}
	}
	if len(hostIDs) == 0 {
		return nil, fmt.Errorf("host_ids or nodes is required")
	}
	results := make([]PreCheckResult, 0, len(hostIDs))
	for _, hostID := range dedupeStrings(hostIDs) {
		host, err := s.hostRepo.GetByID(ctx, hostID)
		if err != nil {
			results = append(results, PreCheckResult{HostID: hostID, Status: "fail", Message: fmt.Sprintf("host not found: %v", err)})
			continue
		}
		agentPort := host.AgentPort
		if agentPort == 0 {
			agentPort = 9090
		}
		r := PreCheckResult{HostID: hostID, Host: host.Address, Status: "pass", Details: []PreCheckItem{}}
		s.appendPreCheckPortResults(ctx, &r, host.Address, nodeByHostID[hostID])

		// 1. Agent connectivity
		if s.agentClient == nil {
			r.Status = "fail"
			r.Details = append(r.Details, PreCheckItem{Name: "Agent", Passed: false, Message: "agent client not configured"})
			results = append(results, r)
			continue
		}
		healthResult, err := s.agentClient.ExecuteHealthCheck(ctx, host.Address, agentPort, "")
		if err != nil {
			r.Status = "fail"
			r.Details = append(r.Details, PreCheckItem{Name: "Agent", Passed: false, Message: fmt.Sprintf("agent unreachable: %v", err)})
			results = append(results, r)
			continue
		}
		r.Details = append(r.Details, PreCheckItem{Name: "Agent", Passed: true, Value: healthResult.Status})

		// 3. Check mysql client availability via agent environment check
		envData, envErr := s.agentClient.CheckEnvironmentDirect(ctx, host.Address, agentPort)
		if envErr == nil && envData != nil {
			if tools, ok := envData["tools"].(map[string]interface{}); ok {
				mysqlOK, _ := tools["mysql"].(map[string]interface{})
				if mysqlOK != nil {
					avail, _ := mysqlOK["available"].(bool)
					ver, _ := mysqlOK["version"].(string)
					r.Details = append(r.Details, PreCheckItem{Name: "MySQL Client", Passed: avail, Value: ver})
					if !avail {
						r.Status = "fail"
						r.Details[len(r.Details)-1].Message = "mysql client not available or missing shared libraries (e.g. libncurses.so.5)"
					}
				}
				mysqldOK, _ := tools["mysqld"].(map[string]interface{})
				if mysqldOK != nil {
					avail, _ := mysqldOK["available"].(bool)
					ver, _ := mysqldOK["version"].(string)
					r.Details = append(r.Details, PreCheckItem{Name: "mysqld", Passed: avail, Value: ver})
					if !avail {
						r.Status = "fail"
						r.Details[len(r.Details)-1].Message = "mysqld binary not found or not executable"
					}
				}
			}
			if resources, ok := envData["resources"].(map[string]interface{}); ok {
				diskGB, _ := resources["disk_space_gb"].(float64)
				memMB, _ := resources["memory_mb"].(float64)
				sufficient, _ := resources["sufficient"].(bool)
				r.Details = append(r.Details, PreCheckItem{
					Name:   "Resources",
					Passed: sufficient,
					Value:  fmt.Sprintf("disk=%dGB mem=%dMB", int(diskGB), int(memMB)),
					Message: func() string {
						if !sufficient {
							return "insufficient disk or memory"
						}
						return ""
					}(),
				})
				if !sufficient && r.Status == "pass" {
					r.Status = "warn"
				}
			}
		} else {
			r.Details = append(r.Details, PreCheckItem{Name: "Environment", Passed: false, Message: "could not check environment via agent"})
			if r.Status == "pass" {
				r.Status = "warn"
			}
		}

		// Summary message
		failCount := 0
		for _, d := range r.Details {
			if !d.Passed {
				failCount++
			}
		}
		if failCount > 0 {
			r.Message = fmt.Sprintf("%d check(s) failed", failCount)
		} else {
			r.Message = "all checks passed"
		}
		results = append(results, r)
	}
	return results, nil
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (s *ClusterDeployService) appendPreCheckPortResults(ctx context.Context, result *PreCheckResult, host string, nodes []ClusterDeployCheckNode) {
	for _, node := range nodes {
		if node.MySQLPort == 0 {
			continue
		}
		if conflict := s.findManagedPortConflict(ctx, host, node.MySQLPort, ""); conflict != "" {
			result.Status = "fail"
			result.Details = append(result.Details, PreCheckItem{
				Name:    fmt.Sprintf("MySQL Port %d", node.MySQLPort),
				Passed:  false,
				Value:   fmt.Sprintf("%s:%d", host, node.MySQLPort),
				Message: conflict,
			})
		} else {
			result.Details = append(result.Details, PreCheckItem{
				Name:   fmt.Sprintf("MySQL Port %d", node.MySQLPort),
				Passed: true,
				Value:  fmt.Sprintf("%s:%d available", host, node.MySQLPort),
			})
		}
	}
}

// Helper: find a PlanNode in a slice by ID
func findPlanNode(nodes []PlanNode, id string) *PlanNode {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

// Helper: marshal a value to JSON string; returns "{}" on error
func marshalString(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// buildPartialResponse creates a DeployResponse for a failed/partial deployment.
func (s *ClusterDeployService) buildPartialResponse(ctx context.Context, clusterID, clusterType, name string, dep *models.ClusterDeployment, errMsg string, finishedAt *time.Time) *DeployResponse {
	prog := s.getProgress(clusterID)
	resp := &DeployResponse{
		DeploymentID: clusterID,
		ClusterType:  clusterType,
		Name:         name,
		Status:       "failed",
		Message:      errMsg,
		ErrorMessage: errMsg,
		StartedAt:    dep.StartedAt,
		FinishedAt:   finishedAt,
		CreatedAt:    dep.CreatedAt,
	}
	if prog != nil {
		resp.Steps = prog.Steps
		resp.Logs = prog.Logs
	}
	return resp
}

// Deprecated: Use DeployCluster with TypedMHARequestToUniversal instead.
func (s *ClusterDeployService) DeployMHA(ctx context.Context, req DeployMHARequest) (resp *DeployResponse, err error) {
	universalReq := TypedMHARequestToUniversal(req)
	return s.DeployCluster(ctx, universalReq)
}

// Deprecated: Use DeployCluster with TypedMGRRequestToUniversal instead.
func (s *ClusterDeployService) DeployMGR(ctx context.Context, req DeployMGRRequest) (resp *DeployResponse, err error) {
	universalReq := TypedMGRRequestToUniversal(req)
	return s.DeployCluster(ctx, universalReq)
}

// Deprecated: Use DeployCluster with typedPXCRequestToUniversal instead.
func (s *ClusterDeployService) DeployPXC(ctx context.Context, req DeployPXCRequest) (resp *DeployResponse, err error) {
	universalReq := TypedPXCRequestToUniversal(req)
	return s.DeployCluster(ctx, universalReq)
}

// Deprecated: Use DeployCluster with typedHARequestToUniversal instead.
func (s *ClusterDeployService) DeployHA(ctx context.Context, req DeployHARequest) (resp *DeployResponse, err error) {
	universalReq := TypedHARequestToUniversal(req)
	return s.DeployCluster(ctx, universalReq)
}

func (s *ClusterDeployService) GetDeployPlan(ctx context.Context, deploymentID string) (*ClusterDeployPlan, error) {
	dep, err := s.repo.GetByID(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	if dep.PlanJSON == "" {
		return nil, fmt.Errorf("no plan found for deployment %s", deploymentID)
	}
	var plan ClusterDeployPlan
	if err := json.Unmarshal([]byte(dep.PlanJSON), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan for deployment %s: %w", deploymentID, err)
	}
	return &plan, nil
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
		Status:       normalizeDeployStatus(dep.Status),
		Stage:        stage,
		Progress:     progress,
		Message:      message,
		StartedAt:    dep.StartedAt,
		FinishedAt:   dep.FinishedAt,
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
	// Merge DB-persisted node status for nodes not covered by in-memory progress
	dbNodes := s.loadNodesFromDB(ctx, deploymentID)
	if len(dbNodes) > 0 && prog == nil {
		dbDeployNodes := make([]DeployNode, 0, len(dbNodes))
		for _, dn := range dbNodes {
			dbDeployNodes = append(dbDeployNodes, DeployNode{
				InstanceID:  dn.InstanceID,
				Host:        dn.Host,
				Port:        dn.MySQLPort,
				Role:        dn.Role,
				Status:      dn.Status,
				CurrentStep: dn.CurrentStep,
				Message:     dn.Message,
			})
		}
		if len(dbDeployNodes) > 0 {
			resp.Nodes = dbDeployNodes
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
			Status:       normalizeDeployStatus(dep.Status),
			Stage:        stage,
			Progress:     progress,
			Message:      message,
			StartedAt:    dep.StartedAt,
			FinishedAt:   dep.FinishedAt,
			CreatedAt:    dep.CreatedAt,
			Nodes:        s.deploymentNodes(ctx, dep.ID),
		})
	}
	return responses, nil
}

func (s *ClusterDeployService) ListClusters(ctx context.Context) ([]ClusterInfo, error) {
	deployments, err := s.repo.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ClusterInfo, 0, len(deployments))
	for _, dep := range deployments {
		nodeCount := len(s.deploymentNodes(ctx, dep.ID))
		info := ClusterInfo{
			ClusterID:    dep.ClusterID,
			DeploymentID: dep.ID,
			Name:         dep.Name,
			DisplayName:  dep.DisplayName,
			Arch:         dep.Arch,
			ClusterType:  dep.ClusterType,
			Status:       dep.Status,
			Nodes:        nodeCount,
			MySQLVersion: dep.MySQLVersion,
			ConfigJSON:   dep.ConfigJSON,
			CreatedAt:    dep.CreatedAt,
		}
		if info.Arch == "" {
			info.Arch = dep.ClusterType
		}
		if info.Nodes == 0 {
			info.Nodes = dep.Nodes
		}
		result = append(result, info)
	}
	return result, nil
}

type ClusterInfo struct {
	ClusterID    string    `json:"cluster_id"`
	DeploymentID string    `json:"deployment_id"`
	Name         string    `json:"name"`
	DisplayName  string    `json:"display_name"`
	Arch         string    `json:"arch"`
	ClusterType  string    `json:"cluster_type"`
	Status       string    `json:"status"`
	Nodes        int       `json:"nodes"`
	MySQLVersion string    `json:"mysql_version"`
	ConfigJSON   string    `json:"config_json"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *ClusterDeployService) GetClusterDetail(ctx context.Context, clusterID string) (*ClusterDetail, error) {
	dep, err := s.repo.GetByClusterID(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	nodeCount := len(s.deploymentNodes(ctx, dep.ID))
	info := ClusterInfo{
		ClusterID:    dep.ClusterID,
		DeploymentID: dep.ID,
		Name:         dep.Name,
		DisplayName:  dep.DisplayName,
		Arch:         dep.Arch,
		ClusterType:  dep.ClusterType,
		Status:       dep.Status,
		Nodes:        nodeCount,
		MySQLVersion: dep.MySQLVersion,
		ConfigJSON:   dep.ConfigJSON,
		CreatedAt:    dep.CreatedAt,
	}
	if info.Arch == "" {
		info.Arch = dep.ClusterType
	}
	if info.Nodes == 0 {
		info.Nodes = dep.Nodes
	}
	instances, _ := s.instRepo.ListByClusterID(ctx, clusterID)
	return &ClusterDetail{ClusterInfo: info, Instances: instances}, nil
}

type ClusterDetail struct {
	ClusterInfo
	Instances []*models.Instance `json:"instances"`
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
	dep, err := s.resolveDeploymentForDestroy(ctx, clusterID)
	if err != nil {
		return nil, err
	}
	nodes := s.deploymentNodes(ctx, dep.ID)
	decommissioned, err := s.decommissionClusterInstances(ctx, dep.ID)
	if err != nil {
		return nil, err
	}
	if err := s.clearDestroyedClusterManagement(ctx, dep.ID); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateStatus(ctx, dep.ID, "destroyed"); err != nil {
		return nil, err
	}
	message := fmt.Sprintf("Cluster %s destroyed after full backup verification and remote database cleanup", dep.ID)
	if decommissioned == 0 {
		message = fmt.Sprintf("Cluster %s deployment metadata destroyed; no managed instances were found to back up or decommission", dep.ID)
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

func (s *ClusterDeployService) resolveDeploymentForDestroy(ctx context.Context, requestedIDOrName string) (*models.ClusterDeployment, error) {
	dep, err := s.repo.GetByID(ctx, requestedIDOrName)
	if err == nil {
		return dep, nil
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not found") {
		return nil, err
	}
	byName, nameErr := s.repo.GetByName(ctx, requestedIDOrName)
	if nameErr == nil {
		return byName, nil
	}
	if strings.Contains(strings.ToLower(nameErr.Error()), "not found") {
		return nil, err
	}
	return nil, nameErr
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
			return 0, &ClusterDestroyOperationError{
				ClusterID:  clusterID,
				InstanceID: inst.ID,
				Stage:      "backup_and_decommission",
				Err:        err,
			}
		}
	}
	return len(instances), nil
}

func (s *ClusterDeployService) backupAndDecommissionClusterInstance(ctx context.Context, instanceID string) error {
	if s.backupSvc == nil {
		return fmt.Errorf("backup service not configured; cannot proceed with destroy without backup for instance %s", instanceID)
	}
	backup, err := s.backupSvc.ExecuteBackup(ctx, ExecuteBackupRequest{
		InstanceID: instanceID,
		BackupType: "full",
	})
	if err != nil {
		return fmt.Errorf("full backup failed for instance %s, destroy aborted: %w", instanceID, err)
	}
	if err := validateRemovalBackup(backup); err != nil {
		return fmt.Errorf("backup validation failed for instance %s, destroy aborted: %w", instanceID, err)
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
	if s.instRepo == nil {
		return nil
	}
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
	MasterDataDir    string            `json:"master_data_dir,omitempty"`
	MasterBasedir    string            `json:"master_basedir,omitempty"`
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
	PrimaryHostID    string            `json:"primary_host_id"`
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
	Name            string            `json:"name"`
	ClusterID       string            `json:"cluster_id"`
	MasterHostID    string            `json:"master_host_id"`
	ReplicaHostID   string            `json:"replica_host_id"`
	ReplicaHostIDs  []string          `json:"replica_host_ids"`
	MasterHost      string            `json:"master_host"`
	ReplicaHost     string            `json:"replica_host"`
	ReplicaHosts    []SecondaryNode   `json:"replica_hosts"`
	MasterPort      int               `json:"master_port"`
	ReplicaPort     int               `json:"replica_port"`
	MasterAgentPort int               `json:"master_agent_port"`
	MasterServerID  int               `json:"master_server_id,omitempty"`
	MasterDataDir   string            `json:"master_data_dir,omitempty"`
	MasterBasedir   string            `json:"master_basedir,omitempty"`
	ReplUser        string            `json:"repl_user"`
	ReplPassword    string            `json:"repl_password"`
	MySQLUser       string            `json:"mysql_user"`
	MySQLPassword   string            `json:"mysql_password"`
	PseudoMode      bool              `json:"pseudo_mode"`
	ConfigParams    map[string]string `json:"config_params"`
}

type SlaveNode struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	DataDir string `json:"data_dir,omitempty"`
	Basedir string `json:"basedir,omitempty"`
}

type SecondaryNode struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	AgentPort int    `json:"agent_port"`
	ServerID  int    `json:"server_id,omitempty"`
	LocalPort int    `json:"local_port,omitempty"`
	DataDir   string `json:"data_dir,omitempty"`
	Basedir   string `json:"basedir,omitempty"`
}

type BootstrapNode struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	AgentPort int    `json:"agent_port"`
	DataDir   string `json:"data_dir,omitempty"`
}

type PXCNode struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	AgentPort int    `json:"agent_port"`
	DataDir   string `json:"data_dir,omitempty"`
}

type DeployResponse struct {
	DeploymentID  string       `json:"deployment_id"`
	ClusterType   string       `json:"cluster_type"`
	Name          string       `json:"name"`
	Status        string       `json:"status"`
	Stage         string       `json:"stage,omitempty"`
	Progress      int          `json:"progress"`
	Message       string       `json:"message"`
	ErrorMessage  string       `json:"error_message,omitempty"`
	MySQLUser     string       `json:"mysql_user,omitempty"`
	MySQLPassword string       `json:"mysql_password,omitempty"`
	StartedAt     *time.Time   `json:"started_at,omitempty"`
	FinishedAt    *time.Time   `json:"finished_at,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	Nodes         []DeployNode `json:"nodes,omitempty"`
	Steps         []DeployStep `json:"steps,omitempty"`
	Logs          []string     `json:"logs,omitempty"`
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
	// DEBUG: Log the resolved host
	log.Printf("[DEBUG] resolveHostRef: hostID=%s -> Address='%s', AgentPort=%d", hostID, host.Address, port)
	return deploymentHost{Address: host.Address, AgentPort: port}, nil
}

// injectMHAStepPasswords resolves SSH passwords for MHA nodes and injects them
// into the MHA manager deploy step's config. This is called for plan-driven
// real-mode MHA deployments where the plan builder cannot access host repo.
func (s *ClusterDeployService) injectMHAStepPasswords(ctx context.Context, plan *ClusterDeployPlan, req UniversalClusterDeployRequest) error {
	if plan.ClusterType != ClusterTypeMHA {
		return nil
	}
	if s.hostRepo == nil || s.encKey == "" {
		return fmt.Errorf("host repository or encryption key not configured for MHA SSH password resolution")
	}

	// Find the MHA deploy step (type "deploy" with "mha" in config)
	var deployStep *PlanStep
	for i := range plan.Steps {
		if plan.Steps[i].Type == "deploy" && plan.Steps[i].Config != nil {
			if mode, ok := plan.Steps[i].Config["deploy_mode"].(string); ok && mode == "mha" {
				deployStep = &plan.Steps[i]
				break
			}
		}
	}
	if deployStep == nil {
		return fmt.Errorf("no MHA deploy step found in plan")
	}

	// Collect all unique hosts from plan nodes
	passwords := map[string]string{}
	for _, node := range plan.Nodes {
		hostAddr := node.Host
		if hostAddr == "" {
			continue
		}
		if _, already := passwords[hostAddr]; already {
			continue
		}

		// Resolve by HostID first
		if node.HostID != "" {
			host, err := s.hostRepo.GetByID(ctx, node.HostID)
			if err == nil && strings.TrimSpace(host.SSHCredential) != "" {
				plain, decErr := utils.Decrypt(host.SSHCredential, s.encKey)
				if decErr == nil {
					passwords[hostAddr] = plain
					continue
				}
			}
		}

		// Fallback: find by address in host list
		hosts, err := s.hostRepo.List(ctx, 1000, 0)
		if err != nil {
			continue
		}
		for _, h := range hosts {
			if sameHost(h.Address, hostAddr) && strings.TrimSpace(h.SSHCredential) != "" {
				plain, decErr := utils.Decrypt(h.SSHCredential, s.encKey)
				if decErr == nil {
					passwords[hostAddr] = plain
				}
				break
			}
		}
	}

	// Validate that we have passwords for all nodes
	for _, node := range plan.Nodes {
		if node.Host != "" {
			if _, ok := passwords[node.Host]; !ok {
				log.Printf("WARN: no SSH password found for MHA node %s (%s)", node.ID, node.Host)
			}
		}
	}

	// Inject into step config
	if len(passwords) > 0 {
		injected := deployStep.Config["ssh_passwords"]
		if injected == nil {
			deployStep.Config["ssh_passwords"] = passwords
		} else if existing, ok := injected.(map[string]string); ok {
			// Merge: existing entries take precedence
			for k, v := range passwords {
				if _, exists := existing[k]; !exists {
					existing[k] = v
				}
			}
		}
	} else {
		return fmt.Errorf("no SSH passwords could be resolved for MHA nodes")
	}

	return nil
}

func (s *ClusterDeployService) syncClusterManagement(ctx context.Context, clusterType, clusterID string, nodes []pseudoNode) error {
	if s.instRepo == nil {
		return fmt.Errorf("instance repository not configured")
	}
	matched := make([]*models.Instance, 0, len(nodes))
	for _, node := range nodes {
		inst, err := s.findInstanceByEndpoint(ctx, node.Host, node.Port)
		if err != nil {
			log.Printf("syncClusterManagement: findInstanceByEndpoint(%s:%d) failed: %v, creating new", node.Host, node.Port, err)
			inst, err = s.createManagedInstanceForDeployNode(ctx, clusterID, node)
			if err != nil {
				return fmt.Errorf("sync node %s:%d failed: %w", node.Host, node.Port, err)
			}
		} else {
			// Instance already exists — update its connection password if provided
			if strings.TrimSpace(node.MySQLPassword) != "" && s.encKey != "" {
				conn, connErr := s.instRepo.GetConnection(ctx, inst.ID)
				if connErr == nil {
					enc, encErr := utils.Encrypt(node.MySQLPassword, s.encKey)
					if encErr == nil {
						conn.PasswordEncrypted = enc
						if strings.TrimSpace(conn.Username) == "" {
							conn.Username = strings.TrimSpace(node.MySQLUser)
							if conn.Username == "" {
								conn.Username = "root"
							}
						}
						if uErr := s.instRepo.UpdateConnection(ctx, conn); uErr != nil {
							log.Printf("WARN: failed to update password for existing instance %s: %v", inst.ID, uErr)
						}
					}
				}
			}
		}
		inst.ClusterID = clusterID
		log.Printf("syncClusterManagement: updating instance name=%s id=%s clusterID=%s", inst.Name, inst.ID, clusterID)
		if err := s.instRepo.Update(ctx, inst); err != nil {
			log.Printf("syncClusterManagement: Update failed for %s: %v", inst.ID, err)
			// If update fails (e.g. RowsAffected=0), try to continue
			// The instance was just created, topology will still work
			log.Printf("syncClusterManagement: skipping Update error, continuing with topology")
		}
		if err := s.instRepo.UpsertStatus(ctx, inst.ID, &models.InstanceStatus{
			RunStatus:           "running",
			HealthStatus:        "healthy",
			Role:                node.Role,
			ReplicationStatus:   clusterType,
			SecondsBehindMaster: 0,
		}); err != nil {
			log.Printf("syncClusterManagement: UpsertStatus for %s failed: %v", inst.ID, err)
			return err
		}
		matched = append(matched, inst)
	}
	if len(matched) == 0 {
		return fmt.Errorf("no managed instances found for cluster %s", clusterID)
	}

	// Set topology for all matched instances
	if clusterType == ClusterTypeMHA {
		var masterID string
		var replicaIDs []string
		for _, inst := range matched {
			role := ""
			if st, err := s.instRepo.GetStatus(ctx, inst.ID); err == nil {
				role = st.Role
			}
			switch role {
			case "master":
				masterID = inst.ID
			case "replica":
				replicaIDs = append(replicaIDs, inst.ID)
			}
		}
		slaveJSON, _ := json.Marshal(replicaIDs)
		for _, inst := range matched {
			role := ""
			if st, err := s.instRepo.GetStatus(ctx, inst.ID); err == nil {
				role = st.Role
			}
			topology := &models.InstanceTopology{
				InstanceID:      inst.ID,
				ClusterID:       clusterID,
				ReplicationMode: clusterType,
			}
			switch role {
			case "manager":
			case "master":
				topology.SlaveIDs = string(slaveJSON)
			case "replica":
				topology.MasterID = masterID
			}
			if err := s.instRepo.UpsertTopology(ctx, inst.ID, topology); err != nil {
				return err
			}
		}
	} else {
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
	}
	return nil
}

func (s *ClusterDeployService) createManagedInstanceForDeployNode(ctx context.Context, clusterID string, node pseudoNode) (*models.Instance, error) {
	if s.instRepo == nil {
		return nil, fmt.Errorf("instance repository not configured")
	}
	name := fmt.Sprintf("%s-%s-%d", clusterID, strings.ReplaceAll(node.Host, ".", "-"), node.Port)

	// Check if instance with this name already exists (from a previous partial deployment)
	if existing, err := s.findInstanceByName(ctx, name); err == nil {
		// Update existing instance's cluster_id and connection
		existing.ClusterID = clusterID
		if hostID := strings.TrimSpace(node.HostID); hostID != "" {
			existing.HostID = &hostID
		}
		_ = s.instRepo.Update(ctx, existing)
		if strings.TrimSpace(node.MySQLPassword) != "" && s.encKey != "" {
			if enc, encErr := utils.Encrypt(node.MySQLPassword, s.encKey); encErr == nil {
				if conn, connErr := s.instRepo.GetConnection(ctx, existing.ID); connErr == nil {
					conn.PasswordEncrypted = enc
					_ = s.instRepo.UpdateConnection(ctx, conn)
				}
			}
		}
		return existing, nil
	}

	instanceID := uuid.New().String()
	hostID := strings.TrimSpace(node.HostID)
	username := strings.TrimSpace(node.MySQLUser)
	if username == "" {
		username = "root"
	}
	passwordEncrypted := ""
	if strings.TrimSpace(node.MySQLPassword) != "" && s.encKey != "" {
		enc, err := utils.Encrypt(node.MySQLPassword, s.encKey)
		if err != nil {
			return nil, fmt.Errorf("encrypt managed instance password for %s:%d: %w", node.Host, node.Port, err)
		}
		passwordEncrypted = enc
	}
	inst := &models.Instance{
		ID:        instanceID,
		Name:      name,
		ClusterID: clusterID,
	}
	if hostID != "" {
		inst.HostID = &hostID
	}
	if err := s.instRepo.Create(ctx, inst); err != nil {
		return nil, fmt.Errorf("create managed instance for %s:%d: %w", node.Host, node.Port, err)
	}
	log.Printf("createManagedInstance: created instance %s (id=%s) for %s:%d", name, inst.ID, node.Host, node.Port)
	if err := s.instRepo.CreateConnection(ctx, &models.InstanceConnection{
		InstanceID:        instanceID,
		Host:              node.Host,
		Port:              node.Port,
		Username:          username,
		PasswordEncrypted: passwordEncrypted,
		Basedir:           node.Basedir,
		Datadir:           node.DataDir,
	}); err != nil {
		return nil, fmt.Errorf("create managed instance connection for %s:%d: %w", node.Host, node.Port, err)
	}
	return inst, nil
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

// findInstanceByName looks up an existing instance by its generated deploy name.
// Used to recover from partial deployments where the instance was created but
// sync failed before completion.
func (s *ClusterDeployService) findInstanceByName(ctx context.Context, name string) (*models.Instance, error) {
	instances, err := s.instRepo.List(ctx, 1000, 0)
	if err != nil {
		return nil, err
	}
	for _, item := range instances {
		if item.Name == name {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("no instance found with name %s", name)
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

// ClusterPasswordChangeRequest 集群级密码修改请求。
// 适配所有4种架构：single / mha / mgr / pxc。
// CurrentPassword 可选：如果提供则优先使用，否则从各节点存储的密码自动推测。
type ClusterPasswordChangeRequest struct {
	NewPassword     string `json:"new_password" binding:"required"`
	CurrentPassword string `json:"current_password"`
	Username        string `json:"username"`
	UserHost        string `json:"user_host"`
}

type ClusterPasswordChangeResult struct {
	ClusterID    string                      `json:"cluster_id"`
	ClusterType  string                      `json:"cluster_type"`
	TotalNodes   int                         `json:"total_nodes"`
	SuccessNodes int                         `json:"success_nodes"`
	FailedNodes  int                         `json:"failed_nodes"`
	NodeResults  []ClusterPasswordNodeResult `json:"node_results,omitempty"`
}

type ClusterPasswordNodeResult struct {
	InstanceID string `json:"instance_id"`
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Role       string `json:"role"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
}

// ChangeClusterPassword 修改集群所有节点的 MySQL 密码。
// 根据集群架构类型（single/mha/mgr/pxc）自动决定需要修改的节点。
func (s *ClusterDeployService) ChangeClusterPassword(ctx context.Context, clusterID string, req ClusterPasswordChangeRequest) (*ClusterPasswordChangeResult, error) {
	dep, err := s.repo.GetByID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("cluster %s not found: %w", clusterID, err)
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = "root"
	}
	userHost := strings.TrimSpace(req.UserHost)
	if userHost == "" {
		userHost = "%"
	}

	instances, err := s.instRepo.ListByClusterID(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("list instances for cluster %s failed: %w", clusterID, err)
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("cluster %s has no managed instances", clusterID)
	}

	result := &ClusterPasswordChangeResult{
		ClusterID:   clusterID,
		ClusterType: dep.ClusterType,
		TotalNodes:  len(instances),
		NodeResults: make([]ClusterPasswordNodeResult, 0, len(instances)),
	}

	for _, inst := range instances {
		nodeResult := ClusterPasswordNodeResult{
			InstanceID: inst.ID,
			Name:       inst.Name,
			Role:       inst.Status.Role,
			Status:     "pending",
		}

		conn, connErr := s.instRepo.GetConnection(ctx, inst.ID)
		if connErr != nil {
			nodeResult.Status = "failed"
			nodeResult.Message = fmt.Sprintf("connection not found: %v", connErr)
			result.NodeResults = append(result.NodeResults, nodeResult)
			result.FailedNodes++
			continue
		}

		nodeResult.Host = conn.Host
		nodeResult.Port = conn.Port

		agentHost, agentPort, agentErr := resolveAgentHost(ctx, inst, s.instRepo, s.hostRepo, 9090)
		if agentErr != nil {
			nodeResult.Status = "failed"
			nodeResult.Message = fmt.Sprintf("agent host resolution failed: %v", agentErr)
			result.NodeResults = append(result.NodeResults, nodeResult)
			result.FailedNodes++
			continue
		}

		candidates := s.passwordCandidates(ctx, conn, req.CurrentPassword)
		var lastAgentResult *AgentTaskResult
		success := false

		for _, candidate := range candidates {
			targetHost := localMySQLHostForAgent(conn.Host, agentHost)
			agentResult, agentCallErr := s.agentClient.callAgent(ctx, agentHost, agentPort, "/agent/tasks/instance-admin", map[string]interface{}{
				"task_id":     "cluster-change-password-" + uuid.New().String(),
				"instance_id": inst.ID,
				"config": map[string]interface{}{
					"action":      "change_password",
					"target_host": targetHost,
					"target_port": conn.Port,
					"target_user": conn.Username,
					"target_pass": candidate,
					"username":    username,
					"user_host":   userHost,
					"password":    req.NewPassword,
				},
			})
			lastAgentResult = agentResult
			if agentCallErr == nil && agentResult != nil && isSuccessfulTaskStatus(agentResult.Status) {
				success = true
				break
			}
		}

		if success && username == conn.Username {
			enc, encErr := utils.Encrypt(req.NewPassword, s.encKey)
			if encErr == nil {
				_ = s.instRepo.UpdateConnectionPassword(ctx, inst.ID, enc)
			}
		}

		if success {
			nodeResult.Status = "completed"
			nodeResult.Message = "password changed successfully"
			result.SuccessNodes++
		} else {
			nodeResult.Status = "failed"
			if lastAgentResult != nil {
				nodeResult.Message = lastAgentResult.Message
			} else {
				nodeResult.Message = "failed to connect with any known password"
			}
			result.FailedNodes++
		}

		result.NodeResults = append(result.NodeResults, nodeResult)
	}

	if s.auditSvc != nil {
		_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
			UserID:       userIDFromCtx(ctx),
			Operation:    "change_cluster_password",
			Action:       "change_password",
			ResourceType: "cluster_deployment",
			ResourceID:   clusterID,
			Details:      fmt.Sprintf("cluster_type=%s total=%d success=%d failed=%d", dep.ClusterType, result.TotalNodes, result.SuccessNodes, result.FailedNodes),
			Result:       clusterPasswordAuditResult(result),
		})
	}

	return result, nil
}

func clusterPasswordAuditResult(r *ClusterPasswordChangeResult) string {
	if r.SuccessNodes == r.TotalNodes {
		return "success"
	}
	if r.SuccessNodes > 0 {
		return "partial_success"
	}
	return "failed"
}

func (s *ClusterDeployService) passwordCandidates(ctx context.Context, conn *models.InstanceConnection, explicitCurrentPassword string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 8)

	add := func(v string) {
		if !seen[v] && v != "" {
			seen[v] = true
			out = append(out, v)
		}
	}

	// 0. Explicit current_password from request (highest priority)
	add(explicitCurrentPassword)

	// 1. Stored encrypted password for THIS node (decrypted per node)
	if stored, err := utils.Decrypt(conn.PasswordEncrypted, s.encKey); err == nil && stored != "" {
		add(stored)
	}

	// 2. Empty password as last resort
	if !seen[""] {
		seen[""] = true
		out = append(out, "")
	}

	return out
}
