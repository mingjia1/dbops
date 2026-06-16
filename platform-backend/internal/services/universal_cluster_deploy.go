package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
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

	switch normalized.ClusterType {
	case ClusterTypeHA:
		return s.DeployHA(ctx, universalToHARequest(normalized))
	case ClusterTypeMHA:
		return s.DeployMHA(ctx, universalToMHARequest(normalized))
	case ClusterTypeMGR:
		return s.DeployMGR(ctx, universalToMGRRequest(normalized))
	case ClusterTypePXC:
		return s.DeployPXC(ctx, universalToPXCRequest(normalized))
	default:
		return nil, fmt.Errorf("unsupported cluster_type %q", normalized.ClusterType)
	}
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

	plan := &ClusterDeployPlan{
		DeploymentID: normalized.ClusterID,
		ClusterType:  normalized.ClusterType,
		Mode:         normalized.Mode,
		Nodes:        nodes,
		Parameters: map[string]interface{}{
			"mysql_version":    normalized.MySQL.Version,
			"replication_mode": normalized.Replication.Mode,
		},
	}
	plan.Steps = buildUniversalPlanSteps(normalized.ClusterType, nodes)
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
	return req, nil
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

func buildUniversalPlanSteps(clusterType string, nodes []PlanNode) []PlanStep {
	steps := []PlanStep{
		{ID: "validate_input", Name: "Validate deployment input", Type: "validate"},
		{ID: "resolve_hosts", Name: "Resolve hosts", Type: "validate", DependsOn: []string{"validate_input"}},
		{ID: "check_conflicts", Name: "Check port and path conflicts", Type: "validate", DependsOn: []string{"resolve_hosts"}},
		{ID: "prepare_credentials", Name: "Prepare credentials", Type: "configure", DependsOn: []string{"check_conflicts"}},
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
		PlanStep{ID: "verify_cluster", Name: "Verify cluster", Type: "verify", DependsOn: []string{last}},
		PlanStep{ID: "sync_metadata", Name: "Sync metadata", Type: "sync", DependsOn: []string{"verify_cluster"}},
		PlanStep{ID: "write_audit", Name: "Write audit log", Type: "sync", DependsOn: []string{"sync_metadata"}},
	)
	return steps
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
		"package_url":      true,
		"package_checksum": true,
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
		common["wsrep_sst_port"] = true
		common["wsrep_ssl_enabled"] = true
		common["wsrep_provider_options"] = true
		common["pxc_encrypt_cluster_traffic"] = true
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
		case "replica":
			out.SlaveHosts = append(out.SlaveHosts, SlaveNode{Host: node.Host, Port: node.MySQLPort})
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
		out.ConfigParams["group_name"] = stableUniversalGroupName(req.ClusterID, req.Name)
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

func stableUniversalGroupName(clusterID, name string) string {
	seed := defaultString(clusterID, name)
	if seed == "" {
		seed = "dbops-mgr"
	}
	sum := sha256.Sum256([]byte(seed))
	hexValue := hex.EncodeToString(sum[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexValue[0:8], hexValue[8:12], hexValue[12:16], hexValue[16:20], hexValue[20:32])
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

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
