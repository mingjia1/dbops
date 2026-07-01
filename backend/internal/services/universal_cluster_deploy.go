package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	ClusterTypeHA  = "ha"
	ClusterTypeMHA = "mha"
	ClusterTypeMGR = "mgr"
	ClusterTypePXC = "pxc"

	DeployModeReal         = "real"
	DeployModePseudo       = "pseudo"
	DeployModeValidateOnly = "validate_only"
)

type UniversalClusterDeployRequest struct {
	ClusterID   string                 `json:"cluster_id"`
	Name        string                 `json:"name"`
	ClusterType string                 `json:"cluster_type"`
	Mode        string                 `json:"mode"`
	MySQL       MySQLDeployOptions     `json:"mysql"`
	Replication ReplicationOptions     `json:"replication"`
	Nodes       []ClusterDeployNode    `json:"nodes"`
	Custom      map[string]interface{} `json:"custom,omitempty"`
}

type MySQLDeployOptions struct {
	Version         string            `json:"version,omitempty"`
	User            string            `json:"user,omitempty"`
	Password        string            `json:"password,omitempty"`
	PackageURL      string            `json:"package_url,omitempty"`
	PackageChecksum string            `json:"package_checksum,omitempty"`
	Config          map[string]string `json:"config,omitempty"`
}

type ReplicationOptions struct {
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Mode     string `json:"mode,omitempty"`
}

type ClusterDeployNode struct {
	InstanceID string                 `json:"instance_id,omitempty"`
	HostID     string                 `json:"host_id,omitempty"`
	Host       string                 `json:"host,omitempty"`
	AgentPort  int                    `json:"agent_port,omitempty"`
	MySQLPort  int                    `json:"mysql_port,omitempty"`
	Role       string                 `json:"role"`
	DataDir    string                 `json:"data_dir,omitempty"`
	Basedir    string                 `json:"basedir,omitempty"`
	ServerID   int                    `json:"server_id,omitempty"`
	Custom     map[string]interface{} `json:"custom,omitempty"`
}

type ClusterDeployPlan struct {
	DeploymentID string                 `json:"deployment_id"`
	ClusterType  string                 `json:"cluster_type"`
	Mode         string                 `json:"mode"`
	Nodes        []PlanNode             `json:"nodes"`
	Steps        []PlanStep             `json:"steps"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
}

type PlanNode struct {
	ID        string                 `json:"id"`
	HostID    string                 `json:"host_id,omitempty"`
	Host      string                 `json:"host"`
	AgentPort int                    `json:"agent_port"`
	MySQLPort int                    `json:"mysql_port"`
	Role      string                 `json:"role"`
	DataDir   string                 `json:"data_dir,omitempty"`
	Basedir   string                 `json:"basedir,omitempty"`
	ServerID  int                    `json:"server_id,omitempty"`
	Custom    map[string]interface{} `json:"custom,omitempty"`
}

type PlanStep struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	TargetNode string                 `json:"target_node,omitempty"`
	AgentPath  string                 `json:"agent_path,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`
	DependsOn  []string               `json:"depends_on,omitempty"`
}

type UniversalDeployValidationResponse struct {
	Valid bool               `json:"valid"`
	Plan  *ClusterDeployPlan `json:"plan,omitempty"`
}

func (s *ClusterDeployService) DeployCluster(ctx context.Context, req UniversalClusterDeployRequest) (*DeployResponse, error) {
	normalized, err := s.normalizeUniversalDeployRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if normalized.Mode == DeployModeValidateOnly {
		plan, err := s.BuildClusterDeployPlan(ctx, normalized)
		if err != nil {
			return nil, err
		}
		s.syncNodesFromPlan(ctx, plan.DeploymentID, plan)
		return &DeployResponse{
			DeploymentID: plan.DeploymentID,
			ClusterType:  plan.ClusterType,
			Name:         normalized.Name,
			Status:       "validated",
			Message:      "cluster deployment request is valid",
			CreatedAt:    time.Now(),
			Nodes:        planNodesToDeployNodes(plan.Nodes),
			Steps:        planStepsToDeploySteps(plan.Steps),
		}, nil
	}

	plan, err := s.BuildClusterDeployPlan(ctx, normalized)
	if err != nil {
		return nil, err
	}

	// For pseudo mode, use plan-based execution directly.
	if normalized.Mode == DeployModePseudo {
		return s.ExecuteClusterDeployPlan(ctx, plan, normalized)
	}

	// For real mode: inject architecture-specific credentials before execution.
	if normalized.Mode == DeployModeReal && normalized.ClusterType == ClusterTypeMHA {
		if err := s.injectMHAStepPasswords(ctx, plan, normalized); err != nil {
			return nil, err
		}
	}

	// For real mode: execute the plan through the plan execution engine.
	// This replaces the old dispatch to typed deploy methods.
	return s.ExecuteClusterDeployPlan(ctx, plan, normalized)
}

func (s *ClusterDeployService) ValidateClusterDeploy(ctx context.Context, req UniversalClusterDeployRequest) (*UniversalDeployValidationResponse, error) {
	normalized, err := s.normalizeUniversalDeployRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	plan, err := s.BuildClusterDeployPlan(ctx, normalized)
	if err != nil {
		return nil, err
	}
	return &UniversalDeployValidationResponse{Valid: true, Plan: plan}, nil
}

func (s *ClusterDeployService) BuildClusterDeployPlan(ctx context.Context, req UniversalClusterDeployRequest) (*ClusterDeployPlan, error) {
	normalized, err := s.normalizeUniversalDeployRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	nodes := make([]PlanNode, 0, len(normalized.Nodes))
	for i, node := range normalized.Nodes {
		id := node.InstanceID
		if id == "" {
			id = fmt.Sprintf("node-%d", i+1)
		}
		nodes = append(nodes, PlanNode{
			ID:        id,
			HostID:    node.HostID,
			Host:      node.Host,
			AgentPort: node.AgentPort,
			MySQLPort: node.MySQLPort,
			Role:      node.Role,
			DataDir:   node.DataDir,
			Basedir:   node.Basedir,
			ServerID:  node.ServerID,
			Custom:    copyMap(node.Custom),
		})
	}

	// Build plan parameters with all relevant configuration
	params := map[string]interface{}{
		"mysql_version":    normalized.MySQL.Version,
		"mysql_user":       normalized.MySQL.User,
		"replication_mode": normalized.Replication.Mode,
		"replication_user": normalized.Replication.User,
		"custom_options":   normalized.Custom,
	}
	if normalized.MySQL.PackageURL != "" {
		params["package_url"] = normalized.MySQL.PackageURL
	}
	if len(normalized.MySQL.Config) > 0 {
		params["mysql_config"] = normalized.MySQL.Config
	}

	plan := &ClusterDeployPlan{
		DeploymentID: normalized.ClusterID,
		ClusterType:  normalized.ClusterType,
		Mode:         normalized.Mode,
		Nodes:        nodes,
		Parameters:   params,
	}
	plan.Steps = buildUniversalPlanSteps(normalized.ClusterType, nodes, normalized)
	return plan, nil
}

func (s *ClusterDeployService) normalizeUniversalDeployRequest(ctx context.Context, req UniversalClusterDeployRequest) (UniversalClusterDeployRequest, error) {
	req.ClusterType = strings.ToLower(strings.TrimSpace(req.ClusterType))
	req.Mode = strings.ToLower(strings.TrimSpace(req.Mode))
	if req.Mode == "" {
		req.Mode = DeployModeReal
	}
	if req.ClusterID == "" {
		req.ClusterID = defaultString(req.Name, fmt.Sprintf("%s-%d", req.ClusterType, time.Now().Unix()))
	}
	if req.Name == "" {
		req.Name = req.ClusterID
	}
	if req.MySQL.User == "" {
		req.MySQL.User = "root"
	}
	if req.MySQL.Version == "" {
		req.MySQL.Version = "8.0"
	}
	if req.MySQL.PackageURL == "" && s.versions != nil {
		versionID := req.MySQL.Version
		if !strings.Contains(versionID, "-") {
			versionID = "mysql-" + versionID
		}
		if entry, err := s.versions.Get(versionID); err == nil && entry != nil && entry.PackageURL != "" {
			req.MySQL.PackageURL = entry.PackageURL
			req.MySQL.Version = entry.Version
		}
	}
	// Relay server override: when relay_url is set in custom, override package_url
	// to ensure all nodes download from the relay server.
	if relayURL, ok := stringCustom(req.Custom, "relay_url"); ok && relayURL != "" && req.MySQL.Version != "" {
		tarball := fmt.Sprintf("mysql-%s-linux-glibc2.17-x86_64.tar.xz", req.MySQL.Version)
		relayBase := strings.TrimRight(relayURL, "/")
		req.MySQL.PackageURL = fmt.Sprintf("%s/mysql/%s/%s", relayBase, req.MySQL.Version, tarball)
	}
	if req.Replication.User == "" {
		req.Replication.User = defaultString(s.defaults.ReplicationUser, "repl")
	}
	if req.Replication.Password == "" {
		req.Replication.Password = s.defaults.ReplicationPass
	}
	if req.Replication.Mode == "" {
		if req.ClusterType == ClusterTypeMGR {
			req.Replication.Mode = "single-primary"
		} else {
			req.Replication.Mode = "async"
		}
	}
	if req.Custom == nil {
		req.Custom = map[string]interface{}{}
	}
	if err := validateUniversalClusterType(req.ClusterType); err != nil {
		return req, err
	}
	if req.Mode != DeployModeReal && req.Mode != DeployModePseudo && req.Mode != DeployModeValidateOnly {
		return req, fmt.Errorf("unsupported deploy mode %q", req.Mode)
	}
	if len(req.Nodes) == 0 {
		return req, fmt.Errorf("at least one node is required")
	}
	for i := range req.Nodes {
		node := &req.Nodes[i]
		node.Role = normalizeDeployRole(req.ClusterType, node.Role)
		if node.MySQLPort == 0 {
			node.MySQLPort = 3306
		}
		if node.AgentPort == 0 {
			node.AgentPort = 9090
		}
		if node.DataDir == "" {
			node.DataDir = fmt.Sprintf("/data/mysql/%d", node.MySQLPort)
		}
		if node.Custom == nil {
			node.Custom = map[string]interface{}{}
		}
		if node.HostID != "" {
			resolved, err := s.resolveHostRef(ctx, node.HostID, node.Host)
			if err != nil {
				return req, err
			}
			if resolved.Address != "" {
				node.Host = resolved.Address
			}
			if node.AgentPort == 9090 && resolved.AgentPort != 0 {
				node.AgentPort = resolved.AgentPort
			}
		}
		if strings.TrimSpace(node.Host) == "" {
			return req, fmt.Errorf("node %d host or host_id is required", i+1)
		}
	}
	if err := validateUniversalRoles(req.ClusterType, req.Nodes); err != nil {
		return req, err
	}
	if err := validateDeployCustomOptions(req.ClusterType, req.Custom, req.MySQL.Config, req.Nodes); err != nil {
		return req, err
	}
	if err := s.checkDeployConflicts(ctx, req); err != nil {
		return req, err
	}
	return req, nil
}

func (s *ClusterDeployService) checkDeployConflicts(ctx context.Context, req UniversalClusterDeployRequest) error {
	if s.instRepo == nil {
		return nil
	}
	if err := s.checkClusterIDConflict(ctx, req.ClusterID); err != nil {
		return err
	}
	if err := s.checkNodePortDataDirConflicts(ctx, req.Nodes, req.ClusterID); err != nil {
		return err
	}
	if req.ClusterType == ClusterTypeMGR {
		groupName := stringFromMap(req.Custom, "group_name")
		if groupName != "" {
			if err := s.checkMgrGroupNameConflict(ctx, groupName, req.ClusterID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ClusterDeployService) checkClusterIDConflict(ctx context.Context, clusterID string) error {
	deployments, err := s.repo.List(ctx, 1000, 0)
	if err != nil {
		return nil
	}
	for _, d := range deployments {
		if d.Status == "destroyed" || d.Status == "failed" {
			continue
		}
		dcid := extractClusterIDFromRequest(d.RequestJSON)
		if dcid == clusterID {
			return fmt.Errorf("cluster_id %q already exists (deployment %s, status=%s). Please choose a different cluster ID", clusterID, d.Name, d.Status)
		}
	}
	return nil
}

func (s *ClusterDeployService) checkNodePortDataDirConflicts(ctx context.Context, nodes []ClusterDeployNode, currentClusterID string) error {
	instances, err := s.instRepo.List(ctx, 10000, 0)
	if err != nil {
		return nil
	}
	type endpoint struct {
		host      string
		port      int
		dataDir   string
		instName  string
		clusterID string
	}
	var existing []endpoint
	for _, inst := range instances {
		conn, err := s.instRepo.GetConnection(ctx, inst.ID)
		if err != nil || conn == nil {
			continue
		}
		if conn.Host == "" {
			continue
		}
		existing = append(existing, endpoint{
			host:      conn.Host,
			port:      conn.Port,
			dataDir:   conn.Datadir,
			instName:  inst.Name,
			clusterID: inst.ClusterID,
		})
	}
	for _, dep := range nodes {
		host := strings.TrimSpace(dep.Host)
		for _, ex := range existing {
			if host != ex.host {
				continue
			}
			if ex.clusterID == currentClusterID {
				continue
			}

			// Skip unassigned instances (empty cluster_id) — they are available for adoption.
			if ex.clusterID == "" {
				continue
			}
			if dep.MySQLPort == ex.port {
				return fmt.Errorf("port conflict: %s:%d already in use by instance %q (cluster=%s). Please change mysql_port for this node", host, dep.MySQLPort, ex.instName, ex.clusterID)
			}
			if dep.DataDir != "" && dep.DataDir == ex.dataDir {
				return fmt.Errorf("datadir conflict: %s on %s already in use by instance %q (cluster=%s). Please change data_dir for this node", dep.DataDir, host, ex.instName, ex.clusterID)
			}
		}
	}
	return nil
}

func (s *ClusterDeployService) checkMgrGroupNameConflict(ctx context.Context, groupName, currentClusterID string) error {
	deployments, err := s.repo.List(ctx, 1000, 0)
	if err != nil {
		return nil
	}
	for _, d := range deployments {
		if d.Status == "destroyed" || d.Status == "failed" {
			continue
		}
		dcid := extractClusterIDFromRequest(d.RequestJSON)
		if dcid == currentClusterID {
			continue
		}
		if d.ClusterType != ClusterTypeMGR {
			continue
		}
		existingGroupName := extractGroupNameFromRequest(d.RequestJSON)
		if existingGroupName == groupName {
			return fmt.Errorf("MGR group_name %q already in use by deployment %q. Please choose a different group name", groupName, d.Name)
		}
	}
	return nil
}

func extractClusterIDFromRequest(requestJSON string) string {
	var m map[string]interface{}
	if json.Unmarshal([]byte(requestJSON), &m) != nil {
		return ""
	}
	if v, ok := m["cluster_id"].(string); ok {
		return v
	}
	return ""
}

func extractGroupNameFromRequest(requestJSON string) string {
	var m map[string]interface{}
	if json.Unmarshal([]byte(requestJSON), &m) != nil {
		return ""
	}
	if custom, ok := m["custom"].(map[string]interface{}); ok {
		if v, ok := custom["group_name"].(string); ok {
			return v
		}
	}
	return ""
}

func stringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func validateUniversalClusterType(clusterType string) error {
	switch clusterType {
	case ClusterTypeHA, ClusterTypeMHA, ClusterTypeMGR, ClusterTypePXC:
		return nil
	default:
		return fmt.Errorf("unsupported cluster_type %q", clusterType)
	}
}

func normalizeDeployRole(clusterType, role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		switch clusterType {
		case ClusterTypeHA, ClusterTypeMHA:
			return "replica"
		case ClusterTypeMGR, ClusterTypePXC:
			return "secondary"
		}
	}
	if role == "slave" {
		return "replica"
	}
	if clusterType == ClusterTypePXC && role == "primary" {
		return "secondary"
	}
	return role
}

func validateUniversalRoles(clusterType string, nodes []ClusterDeployNode) error {
	counts := map[string]int{}
	for _, node := range nodes {
		counts[node.Role]++
		if !allowedRole(clusterType, node.Role) {
			return fmt.Errorf("role %q is not valid for %s deployment", node.Role, clusterType)
		}
	}
	switch clusterType {
	case ClusterTypeHA:
		if counts["master"] != 1 || counts["replica"] < 1 {
			return fmt.Errorf("ha deployment requires exactly one master and at least one replica")
		}
	case ClusterTypeMHA:
		if counts["manager"] != 1 || counts["master"] != 1 || counts["replica"] < 1 {
			return fmt.Errorf("mha deployment requires one manager, one master and at least one replica")
		}
	case ClusterTypeMGR:
		if counts["primary"]+counts["bootstrap"] != 1 {
			return fmt.Errorf("mgr deployment requires exactly one primary or bootstrap node")
		}
	case ClusterTypePXC:
		if counts["bootstrap"] != 1 {
			return fmt.Errorf("pxc deployment requires exactly one bootstrap node")
		}
	}
	return nil
}

func allowedRole(clusterType, role string) bool {
	switch clusterType {
	case ClusterTypeHA:
		return role == "master" || role == "replica"
	case ClusterTypeMHA:
		return role == "manager" || role == "master" || role == "replica"
	case ClusterTypeMGR:
		return role == "primary" || role == "bootstrap" || role == "secondary"
	case ClusterTypePXC:
		return role == "bootstrap" || role == "secondary"
	default:
		return false
	}
}

// buildUniversalPlanSteps dispatches to architecture-specific plan builders.
func buildUniversalPlanSteps(clusterType string, nodes []PlanNode, req UniversalClusterDeployRequest) []PlanStep {
	switch clusterType {
	case ClusterTypeHA:
		return buildHAPlanSteps(nodes, req)
	case ClusterTypeMHA:
		return buildMHAPlanSteps(nodes, req)
	case ClusterTypeMGR:
		return buildMGRPlanSteps(nodes, req)
	case ClusterTypePXC:
		return buildPXCPlanSteps(nodes, req)
	default:
		return buildGenericPlanSteps(nodes, clusterType)
	}
}

// buildGenericPlanSteps is a fallback for unknown cluster types.
func buildGenericPlanSteps(nodes []PlanNode, clusterType string) []PlanStep {
	steps := []PlanStep{
		{ID: "validate_input", Name: "校验部署参数", Type: "validate"},
		{ID: "resolve_hosts", Name: "解析主机信息", Type: "validate", DependsOn: []string{"validate_input"}},
		{ID: "check_conflicts", Name: "检查端口和路径冲突", Type: "validate", DependsOn: []string{"resolve_hosts"}},
		{ID: "prepare_credentials", Name: "准备凭据", Type: "configure", DependsOn: []string{"check_conflicts"}},
	}
	last := "prepare_credentials"
	for _, node := range nodes {
		stepType := "join"
		if node.Role == "master" || node.Role == "primary" || node.Role == "bootstrap" {
			stepType = "bootstrap"
		}
		id := fmt.Sprintf("%s_%s", stepType, node.ID)
		steps = append(steps, PlanStep{
			ID:         id,
			Name:       fmt.Sprintf("%s %s", strings.Title(stepType), node.Host),
			Type:       stepType,
			TargetNode: node.ID,
			AgentPath:  "/agent/tasks/deploy",
			DependsOn:  []string{last},
			Config:     map[string]interface{}{"cluster_type": clusterType, "role": node.Role},
		})
		last = id
	}
	steps = append(steps,
		PlanStep{ID: "verify_cluster", Name: "验证集群", Type: "verify", DependsOn: []string{last}},
		PlanStep{ID: "sync_metadata", Name: "同步元数据", Type: "sync", DependsOn: []string{"verify_cluster"}},
		PlanStep{ID: "write_audit", Name: "写入审计日志", Type: "sync", DependsOn: []string{"sync_metadata"}},
	)
	return steps
}

// buildHAPlanSteps generates HA master-replica specific deployment steps.
func buildHAPlanSteps(nodes []PlanNode, req UniversalClusterDeployRequest) []PlanStep {
	steps := preamblePlanSteps()
	var masterNode *PlanNode
	var replicaNodes []PlanNode
	for i := range nodes {
		if nodes[i].Role == "master" {
			masterNode = &nodes[i]
		} else {
			replicaNodes = append(replicaNodes, nodes[i])
		}
	}
	last := "prepare_credentials"

	// Master bootstrap step
	// Note: master_host is "127.0.0.1" because the agent runs locally on the target host.
	if masterNode != nil {
		masterConfig := map[string]interface{}{
			"deploy_mode": "single",
			"host":        masterNode.Host,
			"port":        masterNode.MySQLPort,
			"mysql_user":  req.MySQL.User,
			"mysql_pass":  req.MySQL.Password,
		}
		if masterNode.ServerID != 0 {
			masterConfig["server_id"] = masterNode.ServerID
		}
		if masterNode.DataDir != "" {
			masterConfig["data_dir"] = masterNode.DataDir
			masterConfig["datadir"] = masterNode.DataDir
		}
		if masterNode.Basedir != "" {
			masterConfig["basedir"] = masterNode.Basedir
		}
		mergeCommonDeployConfig(masterConfig, req)
		id := fmt.Sprintf("bootstrap_%s", masterNode.ID)
		steps = append(steps, PlanStep{
			ID: id, Name: fmt.Sprintf("部署主节点 %s", masterNode.Host),
			Type: "bootstrap", TargetNode: masterNode.ID, AgentPath: "/agent/tasks/deploy",
			DependsOn: []string{last}, Config: masterConfig,
		})
		last = id
	}

	// Replica join steps
	// Note: master_host uses master's real IP (for replica to connect to),
	// while slave_host is "127.0.0.1" because the agent runs locally.
	for i := range replicaNodes {
		// Step 1: Initialize MySQL on the replica as a single instance
		initConfig := map[string]interface{}{
			"deploy_mode": "single",
			"host":        replicaNodes[i].Host,
			"port":        replicaNodes[i].MySQLPort,
			"mysql_user":  req.MySQL.User,
			"mysql_pass":  req.MySQL.Password,
		}
		if replicaNodes[i].DataDir != "" {
			initConfig["data_dir"] = replicaNodes[i].DataDir
			initConfig["datadir"] = replicaNodes[i].DataDir
		}
		if replicaNodes[i].Basedir != "" {
			initConfig["basedir"] = replicaNodes[i].Basedir
		}
		if replicaNodes[i].ServerID != 0 {
			initConfig["server_id"] = replicaNodes[i].ServerID
		}
		mergeCommonDeployConfig(initConfig, req)
		initID := fmt.Sprintf("init_%s", replicaNodes[i].ID)
		steps = append(steps, PlanStep{
			ID: initID, Name: fmt.Sprintf("安装从库 MySQL %s", replicaNodes[i].Host),
			Type: "join", TargetNode: replicaNodes[i].ID, AgentPath: "/agent/tasks/deploy",
			DependsOn: []string{last}, Config: initConfig,
		})

		// Step 2: Configure replication on the replica
		replicaConfig := map[string]interface{}{
			"deploy_mode":    "ha-replica",
			"master_host":    masterNode.Host,
			"master_port":    masterNode.MySQLPort,
			"slave_host":     "127.0.0.1",
			"slave_port":     replicaNodes[i].MySQLPort,
			"server_id":      defaultInt(replicaNodes[i].ServerID, i+2),
			"replicate_user": req.Replication.User,
			"replicate_pass": req.Replication.Password,
			"mysql_user":     req.MySQL.User,
			"mysql_password": req.MySQL.Password,
		}
		if replicaNodes[i].DataDir != "" {
			replicaConfig["data_dir"] = replicaNodes[i].DataDir
			replicaConfig["datadir"] = replicaNodes[i].DataDir
		}
		if replicaNodes[i].Basedir != "" {
			replicaConfig["basedir"] = replicaNodes[i].Basedir
		}
		mergeCommonDeployConfig(replicaConfig, req)
		joinID := fmt.Sprintf("join_%s", replicaNodes[i].ID)
		steps = append(steps, PlanStep{
			ID: joinID, Name: fmt.Sprintf("配置从库复制 %s", replicaNodes[i].Host),
			Type: "configure", TargetNode: replicaNodes[i].ID, AgentPath: "/agent/tasks/deploy",
			DependsOn: []string{initID}, Config: replicaConfig,
		})
		last = joinID
	}

	return appendPostambleSteps(steps, last)
}

// buildMHAPlanSteps generates MHA-specific deployment steps.
func buildMHAPlanSteps(nodes []PlanNode, req UniversalClusterDeployRequest) []PlanStep {
	steps := preamblePlanSteps()
	var managerNode, masterNode *PlanNode
	var replicaNodes []PlanNode
	for i := range nodes {
		switch nodes[i].Role {
		case "manager":
			managerNode = &nodes[i]
		case "master":
			masterNode = &nodes[i]
		default:
			replicaNodes = append(replicaNodes, nodes[i])
		}
	}
	last := "prepare_credentials"

	if masterNode != nil {
		masterConfig := map[string]interface{}{
			"deploy_mode": "single",
			"host":        masterNode.Host,
			"port":        masterNode.MySQLPort,
			"mysql_user":  req.MySQL.User,
			"mysql_pass":  req.MySQL.Password,
		}
		if masterNode.ServerID != 0 {
			masterConfig["server_id"] = masterNode.ServerID
		}
		if masterNode.DataDir != "" {
			masterConfig["data_dir"] = masterNode.DataDir
			masterConfig["datadir"] = masterNode.DataDir
		}
		if masterNode.Basedir != "" {
			masterConfig["basedir"] = masterNode.Basedir
		}
		mergeCommonDeployConfig(masterConfig, req)
		id := fmt.Sprintf("bootstrap_%s", masterNode.ID)
		steps = append(steps, PlanStep{
			ID: id, Name: fmt.Sprintf("安装 MHA 主库 MySQL %s", masterNode.Host),
			Type: "bootstrap", TargetNode: masterNode.ID, AgentPath: "/agent/tasks/deploy",
			DependsOn: []string{last}, Config: masterConfig,
		})
		last = id

		masterHAConfig := map[string]interface{}{
			"deploy_mode":    "ha-master",
			"master_host":    masterNode.Host,
			"master_port":    masterNode.MySQLPort,
			"server_id":      defaultInt(masterNode.ServerID, 1),
			"replicate_user": req.Replication.User,
			"replicate_pass": req.Replication.Password,
			"mysql_user":     req.MySQL.User,
			"mysql_password": req.MySQL.Password,
		}
		if masterNode.DataDir != "" {
			masterHAConfig["data_dir"] = masterNode.DataDir
			masterHAConfig["datadir"] = masterNode.DataDir
		}
		if masterNode.Basedir != "" {
			masterHAConfig["basedir"] = masterNode.Basedir
		}
		mergeCommonDeployConfig(masterHAConfig, req)
		id = fmt.Sprintf("configure_master_%s", masterNode.ID)
		steps = append(steps, PlanStep{
			ID: id, Name: fmt.Sprintf("配置 MHA 主库复制 %s", masterNode.Host),
			Type: "configure", TargetNode: masterNode.ID, AgentPath: "/agent/tasks/deploy",
			DependsOn: []string{last}, Config: masterHAConfig,
		})
		last = id
	}

	for i := range replicaNodes {
		replicaConfig := map[string]interface{}{
			"deploy_mode": "single",
			"host":        replicaNodes[i].Host,
			"port":        replicaNodes[i].MySQLPort,
			"mysql_user":  req.MySQL.User,
			"mysql_pass":  req.MySQL.Password,
		}
		if replicaNodes[i].DataDir != "" {
			replicaConfig["data_dir"] = replicaNodes[i].DataDir
			replicaConfig["datadir"] = replicaNodes[i].DataDir
		}
		if replicaNodes[i].Basedir != "" {
			replicaConfig["basedir"] = replicaNodes[i].Basedir
		}
		if replicaNodes[i].ServerID != 0 {
			replicaConfig["server_id"] = replicaNodes[i].ServerID
		}
		mergeCommonDeployConfig(replicaConfig, req)
		installID := fmt.Sprintf("join_%s", replicaNodes[i].ID)
		steps = append(steps, PlanStep{
			ID: installID, Name: fmt.Sprintf("安装 MHA 从库 MySQL %s", replicaNodes[i].Host),
			Type: "join", TargetNode: replicaNodes[i].ID, AgentPath: "/agent/tasks/deploy",
			DependsOn: []string{last}, Config: replicaConfig,
		})
		last = installID

		if masterNode != nil {
			replicaHAConfig := map[string]interface{}{
				"deploy_mode":    "ha-replica",
				"master_host":    masterNode.Host,
				"master_port":    masterNode.MySQLPort,
				"slave_host":     "127.0.0.1",
				"slave_port":     replicaNodes[i].MySQLPort,
				"server_id":      defaultInt(replicaNodes[i].ServerID, i+2),
				"replicate_user": req.Replication.User,
				"replicate_pass": req.Replication.Password,
				"mysql_user":     req.MySQL.User,
				"mysql_password": req.MySQL.Password,
			}
			if replicaNodes[i].DataDir != "" {
				replicaHAConfig["data_dir"] = replicaNodes[i].DataDir
				replicaHAConfig["datadir"] = replicaNodes[i].DataDir
			}
			if replicaNodes[i].Basedir != "" {
				replicaHAConfig["basedir"] = replicaNodes[i].Basedir
			}
			mergeCommonDeployConfig(replicaHAConfig, req)
			configureID := fmt.Sprintf("configure_replica_%s", replicaNodes[i].ID)
			steps = append(steps, PlanStep{
				ID: configureID, Name: fmt.Sprintf("配置 MHA 从库复制 %s", replicaNodes[i].Host),
				Type: "configure", TargetNode: replicaNodes[i].ID, AgentPath: "/agent/tasks/deploy",
				DependsOn: []string{last}, Config: replicaHAConfig,
			})
			last = configureID
		}
	}

	// Prepare SSH credentials step
	sshStepID := "prepare_ssh"
	sshUser := "root"
	if v, ok := stringCustom(req.Custom, "ssh_user"); ok {
		sshUser = v
	}
	steps = append(steps, PlanStep{
		ID: sshStepID, Name: "Prepare SSH credentials for MHA",
		Type: "configure", DependsOn: []string{last},
		Config: map[string]interface{}{"ssh_user": sshUser},
	})
	last = sshStepID

	// Deploy MHA manager
	if managerNode != nil {
		managerConfig := map[string]interface{}{
			"deploy_mode":    "mha",
			"manager_host":   managerNode.Host,
			"master_host":    masterNode.Host,
			"master_port":    masterNode.MySQLPort,
			"repl_user":      req.Replication.User,
			"repl_pass":      req.Replication.Password,
			"mysql_user":     req.MySQL.User,
			"mysql_password": req.MySQL.Password,
			"ssh_user":       sshUser,
		}
		if vip, ok := stringCustom(req.Custom, "vip"); ok {
			managerConfig["vip"] = vip
		}
		if vipInterface, ok := stringCustom(req.Custom, "vip_interface"); ok {
			managerConfig["vip_interface"] = vipInterface
		}
		if sshPrivateKey, ok := stringCustom(req.Custom, "ssh_private_key"); ok {
			managerConfig["ssh_private_key"] = sshPrivateKey
		}
		if pi, ok := stringCustom(req.Custom, "ping_interval"); ok {
			managerConfig["ping_interval"] = pi
		}
		if pr, ok := stringCustom(req.Custom, "ping_retry"); ok {
			managerConfig["ping_retry"] = pr
		}
		mergeCommonDeployConfig(managerConfig, req)

		var slaveHosts []string
		var slavePorts []int
		for _, rn := range replicaNodes {
			slaveHosts = append(slaveHosts, rn.Host)
			slavePorts = append(slavePorts, rn.MySQLPort)
		}
		if len(slaveHosts) > 0 {
			managerConfig["slave_hosts"] = slaveHosts
			managerConfig["slave_ports"] = slavePorts
		}

		id := fmt.Sprintf("deploy_mha_%s", managerNode.ID)
		steps = append(steps, PlanStep{
			ID: id, Name: fmt.Sprintf("部署 MHA Manager %s", managerNode.Host),
			Type: "deploy", TargetNode: managerNode.ID, AgentPath: "/agent/tasks/deploy",
			DependsOn: []string{last}, Config: managerConfig,
		})
		last = id
	}

	return appendPostambleSteps(steps, last)
}

// buildMGRPlanSteps generates MGR-specific deployment steps.
// Uses a single-step-per-node approach with deploy_mode="single" and install_type="mgr".
// - host uses the external IP of each node.
// - The health check initially fails since no root@% user exists yet.
// - Password is then set via Unix socket (creates root@% with caching_sha2_password).
// - Retry health check succeeds because root@% now exists with the correct password.
// - initializeMGR connects via external IP, matching root@% (caching_sha2_password).
// - local_address uses the external IP for group_replication_local_address (MGR inter-node communication).
func buildMGRPlanSteps(nodes []PlanNode, req UniversalClusterDeployRequest) []PlanStep {
	steps := preamblePlanSteps()
	last := "prepare_credentials"

	groupName := ""
	if gn, ok := stringCustom(req.Custom, "group_name"); ok {
		groupName = gn
	} else {
		groupName = randomUniversalGroupName()
	}

	localPorts := make([]int, len(nodes))
	for i := range nodes {
		if lp, ok := nodeIntCustomRaw(nodes[i].Custom, "local_port"); ok {
			localPorts[i] = lp
		} else {
			localPorts[i] = 33061 + i
		}
	}

	// Build group seeds (all nodes for the seeds list)
	var groupSeeds []string
	for i := range nodes {
		groupSeeds = append(groupSeeds, fmt.Sprintf("%s:%d", nodes[i].Host, localPorts[i]))
	}

	// Single consolidated step per node: install MySQL + initialize MGR
	// deploy_mode="single" with install_type="mgr" triggers:
	// 1. MySQL install + start with MGR startup params (group_replication plugin etc.)
	// 2. Root password set via Unix socket (creates root@localhost + root@%)
	// 3. initializeMGR called: creates repl user, starts group replication
	//
	// host uses the external IP of each node. The password setup creates root@%
	// with caching_sha2_password, which allows TCP connections from the external IP
	// to authenticate via RSA public key exchange (handled by the mysql client library
	// with MYSQL_PWD env var).
	for i := range nodes {
		isPrimary := nodes[i].Role == "primary" || nodes[i].Role == "bootstrap"
		nodeConfig := map[string]interface{}{
			"deploy_mode":    "single",
			"host":           nodes[i].Host,
			"port":           nodes[i].MySQLPort,
			"mysql_user":     req.MySQL.User,
			"mysql_pass":     req.MySQL.Password,
			"server_id":      defaultInt(nodes[i].ServerID, i+1),
			"install_type":   "mgr",
			"is_primary":     isPrimary,
			"group_name":     groupName,
			"local_address":  fmt.Sprintf("%s:%d", nodes[i].Host, localPorts[i]),
			"seeds":          strings.Join(groupSeeds, ","),
			"replicate_user": req.Replication.User,
			"replicate_pass": req.Replication.Password,
		}
		if nodes[i].DataDir != "" {
			nodeConfig["data_dir"] = nodes[i].DataDir
			nodeConfig["datadir"] = nodes[i].DataDir
		}
		if nodes[i].Basedir != "" {
			nodeConfig["basedir"] = nodes[i].Basedir
		}
		mergeCommonDeployConfig(nodeConfig, req)

		stepType := "join"
		if isPrimary {
			stepType = "bootstrap"
		}
		id := fmt.Sprintf("deploy_mgr_%s", nodes[i].ID)
		steps = append(steps, PlanStep{
			ID: id, Name: fmt.Sprintf("部署 MGR 节点 %s (%s)", nodes[i].Host, nodes[i].Role),
			Type: stepType, TargetNode: nodes[i].ID, AgentPath: "/agent/tasks/deploy",
			DependsOn: []string{last}, Config: nodeConfig,
		})
		last = id
	}

	return appendPostambleSteps(steps, last)
}

// buildPXCPlanSteps generates PXC-specific deployment steps.
func buildPXCPlanSteps(nodes []PlanNode, req UniversalClusterDeployRequest) []PlanStep {
	steps := preamblePlanSteps()
	last := "prepare_credentials"

	clusterName := ""
	if cn, ok := stringCustom(req.Custom, "cluster_name"); ok {
		clusterName = cn
	} else {
		clusterName = req.Name
	}
	sstMethod := "xtrabackup-v2"
	if sm, ok := stringCustom(req.Custom, "sst_method"); ok {
		sstMethod = sm
	}

	// Collect all node hosts for the nodes list
	nodeHosts := make([]string, len(nodes))
	for i := range nodes {
		nodeHosts[i] = nodes[i].Host
	}

	// Use a fixed default wsrep_port (4567) for PXC cluster communication.
	// Note: wsrep_sst_port is a separate parameter for SST transfers and
	// can be passed via req.Custom["wsrep_sst_port"].
	wsrepPort := 4567
	if customPort, ok := nodeIntCustomRaw(req.Custom, "wsrep_port"); ok {
		wsrepPort = customPort
	}
	wsrepSSTPort, hasWSREPSSTPort := nodeIntCustomRaw(req.Custom, "wsrep_sst_port")
	wsrepSSLEnabled, hasWSREPSSLEnabled := stringCustom(req.Custom, "wsrep_ssl_enabled")

	for i := range nodes {
		isBootstrap := nodes[i].Role == "bootstrap"
		// PXC agent requires datadir matching /data/mysql/pxc-* pattern
		nodeDataDir := nodes[i].DataDir
		if nodeDataDir == "" || !strings.Contains(nodeDataDir, "pxc") {
			nodeDataDir = fmt.Sprintf("/data/mysql/pxc-%d", nodes[i].MySQLPort)
		}
		nodeConfig := map[string]interface{}{
			"deploy_mode":    "pxc",
			"cluster_name":   clusterName,
			"bootstrap":      isBootstrap,
			"nodes":          nodeHosts,
			"node_host":      nodes[i].Host,
			"mysql_port":     nodes[i].MySQLPort,
			"wsrep_port":     wsrepPort,
			"sst_method":     sstMethod,
			"replicate_user": req.Replication.User,
			"replicate_pass": req.Replication.Password,
			"mysql_user":     req.MySQL.User,
			"mysql_password": req.MySQL.Password,
			"data_dir":       nodeDataDir,
		}
		if hasWSREPSSTPort {
			nodeConfig["wsrep_sst_port"] = wsrepSSTPort
		}
		if hasWSREPSSLEnabled {
			nodeConfig["wsrep_ssl_enabled"] = wsrepSSLEnabled
		}
		if force, ok := stringCustom(req.Custom, "force"); ok {
			nodeConfig["force"] = force
		}
		mergeCommonDeployConfig(nodeConfig, req)

		stepType := "join"
		if isBootstrap {
			stepType = "bootstrap"
		}
		id := fmt.Sprintf("%s_%s", stepType, nodes[i].ID)
		steps = append(steps, PlanStep{
			ID: id, Name: fmt.Sprintf("部署 PXC 节点 %s (%s)", nodes[i].Host, nodes[i].Role),
			Type: stepType, TargetNode: nodes[i].ID, AgentPath: "/agent/tasks/deploy",
			DependsOn: []string{last}, Config: nodeConfig,
		})
		last = id
	}

	return appendPostambleSteps(steps, last)
}

// preamblePlanSteps returns the common validation & preparation steps.
func preamblePlanSteps() []PlanStep {
	return []PlanStep{
		{ID: "validate_input", Name: "校验部署参数", Type: "validate"},
		{ID: "resolve_hosts", Name: "解析主机信息", Type: "validate", DependsOn: []string{"validate_input"}},
		{ID: "check_conflicts", Name: "检查端口和路径冲突", Type: "validate", DependsOn: []string{"resolve_hosts"}},
		{ID: "prepare_credentials", Name: "准备凭据", Type: "configure", DependsOn: []string{"check_conflicts"}},
	}
}

// appendPostambleSteps appends the common verification, sync, and audit steps.
func appendPostambleSteps(steps []PlanStep, lastDep string) []PlanStep {
	return append(steps,
		PlanStep{ID: "verify_cluster", Name: "验证集群", Type: "verify", DependsOn: []string{lastDep}},
		PlanStep{ID: "sync_metadata", Name: "同步元数据", Type: "sync", DependsOn: []string{"verify_cluster"}},
		PlanStep{ID: "write_audit", Name: "Write audit log", Type: "sync", DependsOn: []string{"sync_metadata"}},
	)
}

// mergeCommonDeployConfig merges MySQL version, package URL, checksum, and MySQL config
// from the universal request into the node config map.
func mergeCommonDeployConfig(config map[string]interface{}, req UniversalClusterDeployRequest) {
	if req.MySQL.Version != "" {
		config["mysql_version"] = req.MySQL.Version
	}
	if req.MySQL.PackageURL != "" {
		config["package_url"] = req.MySQL.PackageURL
	}
	if req.MySQL.PackageChecksum != "" {
		config["checksum"] = req.MySQL.PackageChecksum
	}
	if len(req.MySQL.Config) > 0 {
		mysqlConfig := make(map[string]string)
		for k, v := range req.MySQL.Config {
			if allowedMySQLDeployOption(k) && v != "" {
				mysqlConfig[k] = v
			}
		}
		if len(mysqlConfig) > 0 {
			config["mysql_config"] = mysqlConfig
		}
	}
	// Semi-sync for HA
	if ss, ok := stringCustom(req.Custom, "semi_sync_enabled"); ok && req.ClusterType == ClusterTypeHA {
		config["semi_sync_enabled"] = ss
	}
	// Relay server URL: when set, construct package_url from relay server.
	// This overrides any package_url from version catalog to ensure packages
	// are downloaded from the relay server instead of dev.mysql.com.
	if relayURL, ok := stringCustom(req.Custom, "relay_url"); ok && relayURL != "" {
		version, _ := config["mysql_version"].(string)
		if version != "" {
			tarball := fmt.Sprintf("mysql-%s-linux-glibc2.17-x86_64.tar.xz", version)
			relayBase := strings.TrimRight(relayURL, "/")
			config["package_url"] = fmt.Sprintf("%s/mysql/%s/%s", relayBase, version, tarball)
		}
	}
	// Relay upload URL: agent will upload downloaded packages back to this endpoint
	// so other hosts can find them on the relay server next time.
	if relayUploadURL, ok := stringCustom(req.Custom, "relay_upload_url"); ok && relayUploadURL != "" {
		config["relay_upload_url"] = relayUploadURL
	}
}

// nodeIntCustomRaw extracts an int value from a node's custom map without the
// ClusterDeployNode type requirement.
func nodeIntCustomRaw(custom map[string]interface{}, key string) (int, bool) {
	if custom == nil {
		return 0, false
	}
	v, ok := custom[key]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case int:
		if val == 0 {
			return 0, false
		}
		return val, true
	case float64:
		iv := int(val)
		if iv == 0 {
			return 0, false
		}
		return iv, true
	case string:
		iv, err := strconv.Atoi(val)
		if err != nil || iv == 0 {
			return 0, false
		}
		return iv, true
	default:
		return 0, false
	}
}

func validateDeployCustomOptions(clusterType string, custom map[string]interface{}, mysqlConfig map[string]string, nodes []ClusterDeployNode) error {
	for key := range mysqlConfig {
		if isForbiddenMySQLDeployOption(key) {
			return fmt.Errorf("mysql config %q is managed by DBOps and cannot be overridden", key)
		}
		if !allowedMySQLDeployOption(key) {
			return fmt.Errorf("mysql config %q is not allowed", key)
		}
	}
	allowedCustom := allowedClusterCustomOptions(clusterType)
	for key := range custom {
		if !allowedCustom[key] {
			return fmt.Errorf("custom option %q is not allowed for %s deployment", key, clusterType)
		}
	}
	for _, node := range nodes {
		for key := range node.Custom {
			if !allowedNodeCustomOption(clusterType, key) {
				return fmt.Errorf("node custom option %q is not allowed for %s deployment", key, clusterType)
			}
		}
	}
	return nil
}

func allowedMySQLDeployOption(key string) bool {
	_, ok := map[string]bool{
		"max_connections":                true,
		"innodb_buffer_pool_size":        true,
		"innodb_log_file_size":           true,
		"sync_binlog":                    true,
		"innodb_flush_log_at_trx_commit": true,
		"character_set_server":           true,
		"collation_server":               true,
		"lower_case_table_names":         true,
		"read_only":                      true,
		"super_read_only":                true,
	}[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

func isForbiddenMySQLDeployOption(key string) bool {
	_, ok := map[string]bool{
		"server_id":                       true,
		"port":                            true,
		"datadir":                         true,
		"data_dir":                        true,
		"socket":                          true,
		"pid_file":                        true,
		"pid-file":                        true,
		"log_error":                       true,
		"log-error":                       true,
		"basedir":                         true,
		"gtid_mode":                       true,
		"enforce_gtid_consistency":        true,
		"wsrep_cluster_address":           true,
		"wsrep_node_address":              true,
		"group_replication_group_name":    true,
		"group_replication_local_address": true,
	}[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

func allowedClusterCustomOptions(clusterType string) map[string]bool {
	common := map[string]bool{
		"package_url":       true,
		"package_checksum":  true,
		"relay_url":         true,
		"relay_upload_url":  true,
	}
	switch clusterType {
	case ClusterTypeHA:
		common["semi_sync_enabled"] = true
	case ClusterTypeMHA:
		common["vip"] = true
		common["vip_interface"] = true
		common["ping_interval"] = true
		common["ping_retry"] = true
		common["ssh_user"] = true
		common["ssh_private_key"] = true
	case ClusterTypeMGR:
		common["group_name"] = true
		common["group_seeds"] = true
		common["local_port"] = true
		common["member_weight"] = true
	case ClusterTypePXC:
		common["cluster_name"] = true
		common["sst_method"] = true
		common["wsrep_port"] = true
		common["wsrep_sst_port"] = true
		common["wsrep_ssl_enabled"] = true
		common["wsrep_provider_options"] = true
		common["pxc_encrypt_cluster_traffic"] = true
		common["force"] = true
	}
	return common
}

func allowedNodeCustomOption(clusterType, key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "local_port":
		return clusterType == ClusterTypeMGR
	case "member_weight":
		return clusterType == ClusterTypeMGR
	default:
		return false
	}
}

func universalToHARequest(req UniversalClusterDeployRequest) DeployHARequest {
	out := DeployHARequest{
		Name:          req.Name,
		ClusterID:     req.ClusterID,
		ReplUser:      req.Replication.User,
		ReplPassword:  req.Replication.Password,
		MySQLUser:     req.MySQL.User,
		MySQLPassword: req.MySQL.Password,
		PseudoMode:    req.Mode == DeployModePseudo,
		ConfigParams:  universalCommonConfigParams(req),
	}
	for _, node := range req.Nodes {
		if node.Role == "master" {
			out.MasterHostID = node.HostID
			out.MasterHost = node.Host
			out.MasterPort = node.MySQLPort
			out.MasterAgentPort = node.AgentPort
			out.MasterServerID = node.ServerID
			out.MasterDataDir = node.DataDir
			out.MasterBasedir = node.Basedir
			continue
		}
		out.ReplicaHosts = append(out.ReplicaHosts, SecondaryNode{
			Host:      node.Host,
			Port:      node.MySQLPort,
			AgentPort: node.AgentPort,
			ServerID:  node.ServerID,
			DataDir:   node.DataDir,
			Basedir:   node.Basedir,
		})
	}
	return out
}

func universalToMHARequest(req UniversalClusterDeployRequest) DeployMHARequest {
	out := DeployMHARequest{
		Name:          req.Name,
		ClusterID:     req.ClusterID,
		ReplUser:      req.Replication.User,
		ReplPassword:  req.Replication.Password,
		MySQLUser:     req.MySQL.User,
		MySQLPassword: req.MySQL.Password,
		PseudoMode:    req.Mode == DeployModePseudo,
		ConfigParams:  universalCommonConfigParams(req),
	}
	if vip, ok := stringCustom(req.Custom, "vip"); ok {
		out.VIP = vip
	}
	for _, key := range []string{"ping_interval", "ping_retry", "vip_interface", "ssh_user", "ssh_private_key"} {
		if v, ok := stringCustom(req.Custom, key); ok {
			out.ConfigParams[key] = v
		}
	}
	for _, node := range req.Nodes {
		switch node.Role {
		case "manager":
			out.ManagerHostID = node.HostID
			out.ManagerHost = node.Host
			out.ManagerAgentPort = node.AgentPort
		case "master":
			out.MasterHostID = node.HostID
			out.MasterHost = node.Host
			out.MasterPort = node.MySQLPort
			out.MasterDataDir = node.DataDir
			out.MasterBasedir = node.Basedir
		case "replica":
			out.SlaveHosts = append(out.SlaveHosts, SlaveNode{
				Host: node.Host, Port: node.MySQLPort,
				DataDir: node.DataDir, Basedir: node.Basedir,
			})
		}
	}
	return out
}

func universalToMGRRequest(req UniversalClusterDeployRequest) DeployMGRRequest {
	out := DeployMGRRequest{
		Name:          req.Name,
		ClusterID:     req.ClusterID,
		GroupMode:     req.Replication.Mode,
		MySQLUser:     req.MySQL.User,
		MySQLPassword: req.MySQL.Password,
		PseudoMode:    req.Mode == DeployModePseudo,
		ConfigParams:  universalCommonConfigParams(req),
	}
	out.ConfigParams["replicate_user"] = req.Replication.User
	out.ConfigParams["replicate_pass"] = req.Replication.Password
	if groupName, ok := stringCustom(req.Custom, "group_name"); ok {
		out.ConfigParams["group_name"] = groupName
	} else {
		out.ConfigParams["group_name"] = randomUniversalGroupName()
	}
	for _, node := range req.Nodes {
		if node.Role == "primary" || node.Role == "bootstrap" {
			out.PrimaryHostID = node.HostID
			out.PrimaryHost = node.Host
			out.PrimaryPort = node.MySQLPort
			out.PrimaryAgentPort = node.AgentPort
			if node.ServerID != 0 {
				out.ConfigParams["primary_server_id"] = fmt.Sprintf("%d", node.ServerID)
			}
			if localPort, ok := nodeIntCustom(node, "local_port"); ok {
				out.ConfigParams["primary_local_port"] = fmt.Sprintf("%d", localPort)
			}
			continue
		}
		secondary := SecondaryNode{Host: node.Host, Port: node.MySQLPort, AgentPort: node.AgentPort, ServerID: node.ServerID}
		if localPort, ok := nodeIntCustom(node, "local_port"); ok {
			secondary.LocalPort = localPort
		}
		out.SecondaryHosts = append(out.SecondaryHosts, secondary)
	}
	return out
}

func universalToPXCRequest(req UniversalClusterDeployRequest) DeployPXCRequest {
	out := DeployPXCRequest{
		Name:          req.Name,
		ClusterID:     req.ClusterID,
		MySQLUser:     req.MySQL.User,
		MySQLPassword: req.MySQL.Password,
		PseudoMode:    req.Mode == DeployModePseudo,
		ConfigParams:  universalCommonConfigParams(req),
	}
	for _, key := range []string{"cluster_name", "sst_method", "wsrep_sst_port", "wsrep_ssl_enabled", "wsrep_provider_options", "pxc_encrypt_cluster_traffic"} {
		if v, ok := stringCustom(req.Custom, key); ok {
			out.ConfigParams[key] = v
		}
	}
	for _, node := range req.Nodes {
		if node.Role == "bootstrap" {
			out.BootstrapHostID = node.HostID
			out.BootstrapNode = BootstrapNode{Host: node.Host, Port: node.MySQLPort, AgentPort: node.AgentPort, DataDir: node.DataDir}
			continue
		}
		out.OtherNodes = append(out.OtherNodes, PXCNode{Host: node.Host, Port: node.MySQLPort, AgentPort: node.AgentPort, DataDir: node.DataDir})
	}
	return out
}

func universalCommonConfigParams(req UniversalClusterDeployRequest) map[string]string {
	out := map[string]string{}
	if req.MySQL.Version != "" {
		out["mysql_version"] = req.MySQL.Version
	}
	if req.MySQL.PackageURL != "" {
		out["package_url"] = req.MySQL.PackageURL
	}
	if req.MySQL.PackageChecksum != "" {
		out["package_checksum"] = req.MySQL.PackageChecksum
	}
	for key, value := range req.MySQL.Config {
		if value != "" {
			out[key] = value
		}
	}
	for _, key := range []string{"package_url", "package_checksum"} {
		if v, ok := stringCustom(req.Custom, key); ok {
			out[key] = v
		}
	}
	return out
}

func nodeIntCustom(node ClusterDeployNode, key string) (int, bool) {
	value, ok := stringCustom(node.Custom, key)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed == 0 {
		return 0, false
	}
	return parsed, true
}

func stringCustom(custom map[string]interface{}, key string) (string, bool) {
	value, ok := custom[key]
	if !ok || value == nil {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return v, v != ""
	case fmt.Stringer:
		return v.String(), true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	case int:
		return fmt.Sprintf("%d", v), true
	case float64:
		return fmt.Sprintf("%.0f", v), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func randomUniversalGroupName() string {
	return uuid.NewString()
}

func copyMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func planNodesToDeployNodes(nodes []PlanNode) []DeployNode {
	out := make([]DeployNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, DeployNode{
			InstanceID: node.ID,
			Host:       node.Host,
			Port:       node.MySQLPort,
			Role:       node.Role,
			Status:     "planned",
		})
	}
	return out
}

func planStepsToDeploySteps(steps []PlanStep) []DeployStep {
	out := make([]DeployStep, 0, len(steps))
	for _, step := range steps {
		out = append(out, DeployStep{Name: step.Name, Status: "planned"})
	}
	return out
}

func TypedMHARequestToUniversal(req DeployMHARequest) UniversalClusterDeployRequest {
	out := UniversalClusterDeployRequest{
		ClusterID:   req.ClusterID,
		Name:        req.Name,
		ClusterType: ClusterTypeMHA,
		Replication: ReplicationOptions{
			User:     req.ReplUser,
			Password: req.ReplPassword,
		},
		MySQL: MySQLDeployOptions{
			User:     req.MySQLUser,
			Password: req.MySQLPassword,
		},
		Custom: map[string]interface{}{},
	}
	if req.PseudoMode {
		out.Mode = DeployModePseudo
	}
	if req.VIP != "" {
		out.Custom["vip"] = req.VIP
	}
	out.Nodes = append(out.Nodes, ClusterDeployNode{HostID: req.ManagerHostID, Host: req.ManagerHost, MySQLPort: req.MasterPort, Role: "manager", AgentPort: req.ManagerAgentPort})
	out.Nodes = append(out.Nodes, ClusterDeployNode{
		HostID: req.MasterHostID, Host: req.MasterHost, MySQLPort: req.MasterPort, Role: "master",
		DataDir: req.MasterDataDir, Basedir: req.MasterBasedir,
	})
	for _, s := range req.SlaveHosts {
		out.Nodes = append(out.Nodes, ClusterDeployNode{
			Host: s.Host, MySQLPort: s.Port, Role: "replica",
			DataDir: s.DataDir, Basedir: s.Basedir,
		})
	}
	return out
}

func TypedMGRRequestToUniversal(req DeployMGRRequest) UniversalClusterDeployRequest {
	out := UniversalClusterDeployRequest{
		ClusterID:   req.ClusterID,
		Name:        req.Name,
		ClusterType: ClusterTypeMGR,
		Replication: ReplicationOptions{Mode: req.GroupMode},
		MySQL: MySQLDeployOptions{
			User:     req.MySQLUser,
			Password: req.MySQLPassword,
		},
	}
	if req.PseudoMode {
		out.Mode = DeployModePseudo
	}
	// Propagate package_url / package_checksum from ConfigParams (same as HA/PXC typed converters)
	if v, ok := req.ConfigParams["package_url"]; ok {
		out.MySQL.PackageURL = v
	}
	if v, ok := req.ConfigParams["package_checksum"]; ok {
		out.MySQL.PackageChecksum = v
	}
	// Propagate replication credentials from ConfigParams
	if v, ok := req.ConfigParams["replicate_user"]; ok && out.Replication.User == "" {
		out.Replication.User = v
	}
	if v, ok := req.ConfigParams["replicate_pass"]; ok && out.Replication.Password == "" {
		out.Replication.Password = v
	}
	role := "primary"
	out.Nodes = append(out.Nodes, ClusterDeployNode{HostID: req.PrimaryHostID, Host: req.PrimaryHost, MySQLPort: req.PrimaryPort, Role: role, AgentPort: req.PrimaryAgentPort})
	// Propagate SecondaryNode.DataDir / Basedir to ClusterDeployNode for mgr secondaries.
	// normalizeUniversalDeployRequest will fill in the default /data/mysql/{port} if empty.
	for _, s := range req.SecondaryHosts {
		out.Nodes = append(out.Nodes, ClusterDeployNode{
			Host: s.Host, MySQLPort: s.Port, Role: "secondary", AgentPort: s.AgentPort,
			ServerID: s.ServerID, DataDir: s.DataDir, Basedir: s.Basedir,
		})
	}
	// Carry config_params as MySQL.Config where allowed
	if len(req.ConfigParams) > 0 {
		out.MySQL.Config = make(map[string]string, len(req.ConfigParams))
		for k, v := range req.ConfigParams {
			if allowedMySQLDeployOption(k) {
				out.MySQL.Config[k] = v
			}
		}
	}
	return out
}

func TypedPXCRequestToUniversal(req DeployPXCRequest) UniversalClusterDeployRequest {
	defaultPort := 0
	if v, ok := req.ConfigParams["mysql_port"]; ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && parsed > 0 {
			defaultPort = parsed
		}
	}
	defaultDataDir := strings.TrimSpace(req.ConfigParams["data_dir"])
	out := UniversalClusterDeployRequest{
		ClusterID:   req.ClusterID,
		Name:        req.Name,
		ClusterType: ClusterTypePXC,
		MySQL: MySQLDeployOptions{
			User:     req.MySQLUser,
			Password: req.MySQLPassword,
		},
	}
	if req.PseudoMode {
		out.Mode = DeployModePseudo
	}
	out.Custom = pxcRequestCustom(req)
	if v, ok := req.ConfigParams["package_url"]; ok {
		out.MySQL.PackageURL = v
	}
	if v, ok := req.ConfigParams["package_checksum"]; ok {
		out.MySQL.PackageChecksum = v
	}
	mysqlConfig := make(map[string]string)
	for k, v := range req.ConfigParams {
		if allowedMySQLDeployOption(k) {
			mysqlConfig[k] = v
		}
	}
	if len(mysqlConfig) > 0 {
		out.MySQL.Config = mysqlConfig
	}
	bootstrapPort := req.BootstrapNode.Port
	if bootstrapPort == 0 {
		bootstrapPort = defaultPort
	}
	bootstrapDataDir := defaultString(req.BootstrapNode.DataDir, defaultDataDir)
	out.Nodes = append(out.Nodes, ClusterDeployNode{HostID: req.BootstrapHostID, Host: req.BootstrapNode.Host, MySQLPort: bootstrapPort, Role: "bootstrap", AgentPort: req.BootstrapNode.AgentPort, DataDir: bootstrapDataDir})
	for _, hostID := range req.OtherHostIDs {
		out.Nodes = append(out.Nodes, ClusterDeployNode{HostID: hostID, MySQLPort: defaultPort, Role: "secondary", DataDir: defaultDataDir})
	}
	for _, n := range req.OtherNodes {
		port := n.Port
		if port == 0 {
			port = defaultPort
		}
		dataDir := defaultString(n.DataDir, defaultDataDir)
		out.Nodes = append(out.Nodes, ClusterDeployNode{Host: n.Host, MySQLPort: port, Role: "secondary", AgentPort: n.AgentPort, DataDir: dataDir})
	}
	if len(req.ConfigParams) > 0 {
		out.MySQL.Config = make(map[string]string, len(req.ConfigParams))
		for k, v := range req.ConfigParams {
			if allowedMySQLDeployOption(k) {
				out.MySQL.Config[k] = v
			}
		}
	}
	return out
}

func pxcRequestCustom(req DeployPXCRequest) map[string]interface{} {
	custom := map[string]interface{}{}
	if req.WSREPPort > 0 {
		custom["wsrep_port"] = req.WSREPPort
	}
	if req.SSLEnabled {
		custom["wsrep_ssl_enabled"] = true
	}
	for _, key := range []string{
		"cluster_name",
		"sst_method",
		"wsrep_port",
		"wsrep_sst_port",
		"wsrep_ssl_enabled",
		"wsrep_provider_options",
		"pxc_encrypt_cluster_traffic",
		"force",
	} {
		if value, ok := req.ConfigParams[key]; ok && strings.TrimSpace(value) != "" {
			custom[key] = value
		}
	}
	if len(custom) == 0 {
		return nil
	}
	return custom
}

func TypedHARequestToUniversal(req DeployHARequest) UniversalClusterDeployRequest {
	out := UniversalClusterDeployRequest{
		ClusterID:   req.ClusterID,
		Name:        req.Name,
		ClusterType: ClusterTypeHA,
		Replication: ReplicationOptions{
			User:     req.ReplUser,
			Password: req.ReplPassword,
		},
		MySQL: MySQLDeployOptions{
			User:     req.MySQLUser,
			Password: req.MySQLPassword,
		},
	}
	if req.PseudoMode {
		out.Mode = DeployModePseudo
	}
	if v, ok := req.ConfigParams["package_url"]; ok {
		out.MySQL.PackageURL = v
	}
	if v, ok := req.ConfigParams["package_checksum"]; ok {
		out.MySQL.PackageChecksum = v
	}
	// Carry allowed MySQL config options
	mysqlConfig := make(map[string]string)
	for k, v := range req.ConfigParams {
		if allowedMySQLDeployOption(k) {
			mysqlConfig[k] = v
		}
	}
	if len(mysqlConfig) > 0 {
		out.MySQL.Config = mysqlConfig
	}
	masterNode := ClusterDeployNode{
		HostID:    req.MasterHostID,
		Host:      req.MasterHost,
		MySQLPort: req.MasterPort,
		Role:      "master",
		AgentPort: req.MasterAgentPort,
		DataDir:   req.MasterDataDir,
		Basedir:   req.MasterBasedir,
		ServerID:  req.MasterServerID,
	}
	if masterNode.AgentPort == 0 {
		masterNode.AgentPort = 9090
	}
	out.Nodes = append(out.Nodes, masterNode)
	// Handle plural ReplicaHosts (new format)
	for _, r := range req.ReplicaHosts {
		out.Nodes = append(out.Nodes, ClusterDeployNode{
			Host:      r.Host,
			MySQLPort: r.Port,
			Role:      "replica",
			AgentPort: r.AgentPort,
			DataDir:   r.DataDir,
			Basedir:   r.Basedir,
			ServerID:  r.ServerID,
		})
	}
	// Handle singular ReplicaHost (legacy format)
	if len(req.ReplicaHosts) == 0 && req.ReplicaHost != "" {
		port := req.ReplicaPort
		if port == 0 {
			port = 3306
		}
		out.Nodes = append(out.Nodes, ClusterDeployNode{
			Host:      req.ReplicaHost,
			MySQLPort: port,
			Role:      "replica",
		})
	}
	return out
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
