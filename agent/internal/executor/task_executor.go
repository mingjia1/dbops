package executor

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// TaskExecutor 持有子 executor, 让 cmd/main.go 路由层可以派发到具体方法.
// 之前是空 struct, 路由只能调 TaskExecutor 自己的方法, 缺 upgrade / migration-verify 入口.
type TaskExecutor struct {
	UpgradeExecutor   *UpgradeExecutor
	MigrationExecutor *MigrationExecutor
	relayCli          *relayClient
}

func NewTaskExecutor() *TaskExecutor {
	return &TaskExecutor{
		UpgradeExecutor:   NewUpgradeExecutor(),
		MigrationExecutor: NewMigrationExecutor(),
	}
}

func resolveBinaryPath(name string) string {
	for _, candidate := range []string{
		name,
		filepath.Join("/opt/dbops-pxc/usr/sbin", name),
		filepath.Join("/opt/dbops-pxc/usr/bin", name),
		filepath.Join("/opt/mysql/bin", name),
		filepath.Join("/usr/local/bin", name),
		filepath.Join("/usr/sbin", name),
		filepath.Join("/usr/bin", name),
		filepath.Join("/usr/local/mysql/bin", name),
	} {
		if p, err := exec.LookPath(candidate); err == nil {
			return p
		}
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return name
}

func (e *TaskExecutor) SetRelayConfig(relayHost string, relayPort int, relayToken string) {
	if relayHost != "" {
		e.relayCli = newRelayClient(relayHost, relayPort, relayToken)
	}
}

// ExecuteVerifyMigration forwards to MigrationExecutor.VerifyMigration.
func (t *TaskExecutor) ExecuteVerifyMigration(ctx context.Context, req DeployTaskRequest) *MigrationVerifyResult {
	return t.MigrationExecutor.VerifyMigration(ctx, req)
}

type DeployTaskRequest struct {
	TaskID     string                 `json:"task_id"`
	InstanceID string                 `json:"instance_id"`
	Config     map[string]interface{} `json:"config"`
}

type TaskResult struct {
	TaskID    string    `json:"task_id"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}

type MasterSlaveConfig struct {
	MasterHost    string `json:"master_host"`
	MasterPort    int    `json:"master_port"`
	SlaveHost     string `json:"slave_host"`
	SlavePort     int    `json:"slave_port"`
	ReplicateUser string `json:"replicate_user"`
	ReplicatePass string `json:"replicate_pass"`
	MySQLUser     string `json:"mysql_user"`
	MySQLPass     string `json:"mysql_password"`
	ServerID      int    `json:"server_id"`
	DeployMode    string `json:"deploy_mode"`
}

type BackupConfig struct {
	InstanceID      string `json:"instance_id"`
	BackupType      string `json:"backup_type"`
	BackupMethod    string `json:"backup_method"`
	TargetDir       string `json:"target_dir"`
	MySQLHost       string `json:"mysql_host"`
	MySQLPort       int    `json:"mysql_port"`
	MySQLUser       string `json:"mysql_user"`
	MySQLPass       string `json:"mysql_pass"`
	DataDir         string `json:"datadir"`
	BaseBackupPath  string `json:"base_backup_path"`
	BaseBackupLabel string `json:"base_backup_label"`
	DatabaseSize    int64  `json:"database_size"`
}

type RestoreConfig struct {
	BackupPath string `json:"backup_path"`
	BackupType string `json:"backup_type"`
	DataDir    string `json:"datadir"`
	MySQLHost  string `json:"mysql_host"`
	MySQLPort  int    `json:"mysql_port"`
	MySQLUser  string `json:"mysql_user"`
	MySQLPass  string `json:"mysql_pass"`
}

type BackupScanFile struct {
	FileName   string    `json:"file_name"`
	FilePath   string    `json:"file_path"`
	SizeBytes  int64     `json:"size_bytes"`
	BackupType string    `json:"backup_type"`
	IsDir      bool      `json:"is_dir"`
	Complete   bool      `json:"complete"`
	DetectedAt time.Time `json:"detected_at"`
	MTime      time.Time `json:"mtime"`
}

func (e *TaskExecutor) ExecuteDeploy(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	if err := ValidateDeployConfig(req.Config); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   err.Error(),
			Timestamp: time.Now(),
		}, nil
	}
	deployMode, _ := req.Config["deploy_mode"].(string)

	switch deployMode {
	case "single":
		return e.deploySingleInstance(ctx, req)
	case "master-slave":
		return e.deployMasterSlave(ctx, req)
	case "ha-master":
		return e.deployHAMaster(ctx, req)
	case "ha-replica":
		return e.deployHAReplica(ctx, req)
	case "mha":
		mhaExecutor := NewMHAExecutor()
		return mhaExecutor.DeployMHA(ctx, req)
	case "mgr", "mgr-member", "mgr-single-primary":
		mgrExecutor := NewMGRExecutor()
		if bootstrap, ok := req.Config["bootstrap"].(bool); ok && !bootstrap {
			return mgrExecutor.ConfigureGroupMember(ctx, req)
		}
		return mgrExecutor.DeployMGRSinglePrimary(ctx, req)
	case "pxc":
		return e.DeployPXC(ctx, req)
	case "blank-host-init":
		return e.deployBlankHostInit(ctx, req)
	case "general-cluster-init":
		return e.deployGeneralClusterInit(ctx, req)
	default:
		return e.deploySingleInstance(ctx, req)
	}
}

func (e *TaskExecutor) ExecuteClusterSwitch(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	switchType := configString(req.Config, "switch_type")
	if switchType == "" {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  100,
			Message:   "switch_type is required",
			Timestamp: time.Now(),
		}, nil
	}
	clusterType := configString(req.Config, "cluster_type")
	if clusterType == "" {
		clusterType = configString(req.Config, "target_type")
	}
	if clusterType == "" {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  100,
			Message:   "cluster_type is required",
			Timestamp: time.Now(),
		}, nil
	}
	nodes := configNodeList(req.Config["nodes"])
	if len(nodes) < 2 {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  100,
			Message:   "cluster switch requires at least two nodes",
			Timestamp: time.Now(),
		}, nil
	}
	adaptedReq, err := buildClusterSwitchDeployRequest(req, clusterType, nodes)
	if err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  100,
			Message:   err.Error(),
			Timestamp: time.Now(),
		}, nil
	}
	switch clusterType {
	case "mgr":
		mgrExecutor := NewMGRExecutor()
		if bootstrap, ok := adaptedReq.Config["bootstrap"].(bool); ok && !bootstrap {
			return mgrExecutor.ConfigureGroupMember(ctx, adaptedReq)
		}
		return mgrExecutor.DeployMGRSinglePrimary(ctx, adaptedReq)
	case "pxc":
		return e.DeployPXC(ctx, adaptedReq)
	case "mha", "ha":
		mhaExecutor := NewMHAExecutor()
		return mhaExecutor.DeployMHA(ctx, adaptedReq)
	default:
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  100,
			Message:   fmt.Sprintf("unsupported cluster_type %q", clusterType),
			Timestamp: time.Now(),
		}, nil
	}
}

func configNodeList(raw any) []map[string]any {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	nodes := make([]map[string]any, 0, len(items))
	for _, item := range items {
		node, ok := item.(map[string]any)
		if ok {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func buildClusterSwitchDeployRequest(req DeployTaskRequest, clusterType string, nodes []map[string]any) (DeployTaskRequest, error) {
	current := findClusterSwitchCurrentNode(req, nodes)
	if current == nil {
		return DeployTaskRequest{}, fmt.Errorf("current instance %q not found in cluster switch nodes", req.InstanceID)
	}
	primary := findClusterSwitchPrimaryNode(clusterType, nodes)
	if primary == nil {
		return DeployTaskRequest{}, fmt.Errorf("%s cluster switch requires a primary/master node", clusterType)
	}
	cfg := copyConfig(req.Config)
	cfg["deploy_mode"] = clusterType
	cfg["cluster_type"] = clusterType
	cfg["cluster_name"] = defaultString(configString(cfg, "cluster_name"), defaultString(configString(cfg, "cluster_id"), clusterType+"-cluster"))
	cfg["mysql_user"] = defaultString(configString(cfg, "mysql_user"), defaultString(nodeString(current, "mysql_user"), "root"))
	cfg["mysql_password"] = defaultString(configString(cfg, "mysql_password"), defaultString(nodeString(current, "mysql_password"), nodeString(current, "mysql_pass")))
	cfg["replicate_user"] = defaultString(configString(cfg, "replicate_user"), defaultString(configString(cfg, "replication_user"), "repl"))
	cfg["replicate_pass"] = defaultString(configString(cfg, "replicate_pass"), configString(cfg, "replication_pass"))

	switch clusterType {
	case "mgr":
		seeds := make([]any, 0, len(nodes))
		for _, node := range nodes {
			host := nodeString(node, "host")
			port := nodeInt(node, "local_port")
			if port == 0 {
				port = mgrLocalPort(nodeInt(node, "port"))
			}
			seeds = append(seeds, fmt.Sprintf("%s:%d", host, port))
		}
		cfg["group_name"] = defaultString(configString(cfg, "group_name"), stableMGRGroupName(configString(cfg, "cluster_id"), configString(cfg, "cluster_name")))
		cfg["group_seeds"] = seeds
		cfg["deploy_mode"] = "single-primary"
		cfg["local_address"] = nodeString(current, "host")
		cfg["local_port"] = defaultInt(nodeInt(current, "local_port"), mgrLocalPort(nodeInt(current, "port")))
		cfg["mysql_port"] = nodeInt(current, "port")
		cfg["server_id"] = defaultInt(nodeInt(current, "server_id"), nodeInt(current, "port"))
		cfg["primary_host"] = nodeString(primary, "host")
		cfg["primary_port"] = nodeInt(primary, "port")
		cfg["bootstrap"] = sameClusterSwitchNode(current, primary)
	case "pxc":
		hosts := make([]any, 0, len(nodes))
		for _, node := range nodes {
			hosts = append(hosts, nodeString(node, "host"))
		}
		cfg["nodes"] = hosts
		cfg["node_host"] = nodeString(current, "host")
		cfg["mysql_port"] = nodeInt(current, "port")
		cfg["data_dir"] = defaultString(nodeString(current, "data_dir"), defaultString(nodeString(current, "datadir"), fmt.Sprintf("/data/mysql/pxc-%d", nodeInt(current, "port"))))
		cfg["bootstrap"] = sameClusterSwitchNode(current, primary)
		cfg["sst_method"] = defaultString(configString(cfg, "sst_method"), "xtrabackup-v2")
	case "mha", "ha":
		slaveHosts := make([]any, 0, len(nodes)-1)
		slavePorts := make([]any, 0, len(nodes)-1)
		for _, node := range nodes {
			if sameClusterSwitchNode(node, primary) {
				continue
			}
			slaveHosts = append(slaveHosts, nodeString(node, "host"))
			slavePorts = append(slavePorts, nodeInt(node, "port"))
		}
		cfg["manager_host"] = defaultString(configString(cfg, "manager_host"), nodeString(primary, "host"))
		cfg["master_host"] = nodeString(primary, "host")
		cfg["master_port"] = nodeInt(primary, "port")
		cfg["slave_hosts"] = slaveHosts
		cfg["slave_ports"] = slavePorts
		cfg["repl_user"] = cfg["replicate_user"]
		cfg["repl_pass"] = cfg["replicate_pass"]
	default:
		return DeployTaskRequest{}, fmt.Errorf("unsupported cluster_type %q", clusterType)
	}
	return DeployTaskRequest{TaskID: req.TaskID, InstanceID: req.InstanceID, Config: cfg}, nil
}

func copyConfig(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in)+8)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func findClusterSwitchCurrentNode(req DeployTaskRequest, nodes []map[string]any) map[string]any {
	instanceID := strings.TrimSpace(req.InstanceID)
	if instanceID == "" {
		instanceID = configString(req.Config, "instance_id")
	}
	for _, node := range nodes {
		if nodeString(node, "instance_id") == instanceID {
			return node
		}
	}
	if len(nodes) == 1 {
		return nodes[0]
	}
	return nil
}

func sameClusterSwitchNode(a, b map[string]any) bool {
	if a == nil || b == nil {
		return false
	}
	aid, bid := nodeString(a, "instance_id"), nodeString(b, "instance_id")
	if aid != "" || bid != "" {
		return aid != "" && aid == bid
	}
	return nodeString(a, "host") == nodeString(b, "host") && nodeInt(a, "port") == nodeInt(b, "port")
}

func findClusterSwitchPrimaryNode(clusterType string, nodes []map[string]any) map[string]any {
	for _, node := range nodes {
		role := strings.ToLower(nodeString(node, "role"))
		if role == primaryRoleName(clusterType) || role == "primary" || role == "master" {
			return node
		}
	}
	if len(nodes) > 0 {
		return nodes[0]
	}
	return nil
}

func primaryRoleName(clusterType string) string {
	switch clusterType {
	case "mha", "ha":
		return "master"
	default:
		return "primary"
	}
}

func nodeString(node map[string]any, key string) string {
	if v, ok := node[key].(string); ok {
		return v
	}
	return ""
}

func nodeInt(node map[string]any, key string) int {
	if v, ok := node[key].(int); ok {
		return v
	}
	if v, ok := node[key].(float64); ok {
		return int(v)
	}
	return 0
}

func defaultInt(value, fallback int) int {
	if value != 0 {
		return value
	}
	return fallback
}

func mgrLocalPort(mysqlPort int) int {
	if mysqlPort == 0 {
		return 33061
	}
	return 33061 + mysqlPort%100
}

func stableMGRGroupName(clusterID, clusterName string) string {
	seed := defaultString(clusterID, clusterName)
	if seed == "" {
		return "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	}
	sum := sha256.Sum256([]byte(seed))
	hexValue := hex.EncodeToString(sum[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexValue[0:8], hexValue[8:12], hexValue[12:16], hexValue[16:20], hexValue[20:32])
}

// isDataDirInitialized checks if the MySQL data directory has been initialized
// by looking for ibdata1 (InnoDB system tablespace) which is always created during --initialize-insecure.
func isDataDirInitialized(dataDir string) bool {
	info, err := os.Stat(filepath.Join(dataDir, "ibdata1"))
	return err == nil && info.Size() > 0
}

// mysqlAuthPlugin returns the appropriate auth plugin for the given MySQL version.
// MySQL 5.7 uses mysql_native_password, MySQL 8.0+ uses caching_sha2_password.
func mysqlAuthPlugin(mysqlVersion string) string {
	if strings.HasPrefix(mysqlVersion, "5.") {
		return "mysql_native_password"
	}
	return "caching_sha2_password"
}

// mysqlHasGetServerPublicKey returns true if the MySQL client supports --get-server-public-key.
func mysqlHasGetServerPublicKey(mysqlVersion string) bool {
	return !strings.HasPrefix(mysqlVersion, "5.")
}

func (e *TaskExecutor) deployBlankHostInit(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	ti := NewToolInstaller()
	initReq := BlankHostInitRequest{
		MySQLVersion:    configString(req.Config, "mysql_version"),
		MySQLPort:       configInt(req.Config, "mysql_port"),
		DataDir:         configString(req.Config, "data_dir"),
		Basedir:         configString(req.Config, "basedir"),
		RootPassword:    configString(req.Config, "root_password"),
		ReplUser:        configString(req.Config, "repl_user"),
		ReplPass:        configString(req.Config, "repl_pass"),
		PackageURL:      configString(req.Config, "package_url"),
		PackageChecksum: configString(req.Config, "package_checksum"),
		Force:           configBool(req.Config, "force"),
		SkipToolsCheck:  configBool(req.Config, "skip_tools_check"),
	}
	if initReq.MySQLPort == 0 {
		initReq.MySQLPort = 3306
	}
	if initReq.MySQLVersion == "" {
		initReq.MySQLVersion = "8.0"
	}

	result := ti.InitBlankHost(ctx, initReq)

	stepsJSON, _ := json.Marshal(result.Steps)
	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    result.Status,
		Progress:  result.Progress,
		Message:   result.Message,
		Timestamp: time.Now(),
		Data: map[string]any{
			"steps":     string(stepsJSON),
			"installed": result.Installed,
		},
	}, nil
}

func (e *TaskExecutor) deployGeneralClusterInit(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	ti := NewToolInstaller()

	var nodes []GeneralClusterNode
	if rawNodes, ok := req.Config["nodes"].([]interface{}); ok {
		for _, n := range rawNodes {
			if nodeMap, ok := n.(map[string]interface{}); ok {
				node := GeneralClusterNode{
					Host: configString(nodeMap, "host"),
					Port: configInt(nodeMap, "port"),
					Role: configString(nodeMap, "role"),
				}
				if node.Host != "" {
					nodes = append(nodes, node)
				}
			}
		}
	}

	sshPasswords := make(map[string]string)
	if raw, ok := req.Config["ssh_passwords"].(map[string]interface{}); ok {
		for host, val := range raw {
			if pass, ok := val.(string); ok {
				sshPasswords[host] = pass
			}
		}
	}

	initReq := GeneralClusterInitRequest{
		ClusterType:   configString(req.Config, "cluster_type"),
		MySQLVersion:  configString(req.Config, "mysql_version"),
		Nodes:         nodes,
		RootPassword:  configString(req.Config, "root_password"),
		ReplUser:      configString(req.Config, "repl_user"),
		ReplPass:      configString(req.Config, "repl_pass"),
		VIP:           configString(req.Config, "vip"),
		VIPInterface:  configString(req.Config, "vip_interface"),
		ClusterName:   configString(req.Config, "cluster_name"),
		SSHUser:       configString(req.Config, "ssh_user"),
		SSHPasswords:  sshPasswords,
		SSHPrivateKey: configString(req.Config, "ssh_private_key"),
		SSTMethod:     configString(req.Config, "sst_method"),
	}

	if initReq.ClusterType == "" {
		initReq.ClusterType = configString(req.Config, "deploy_mode")
	}

	result := ti.InitGeneralCluster(ctx, initReq)

	stepsJSON, _ := json.Marshal(result.Steps)
	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    result.Status,
		Progress:  result.Progress,
		Message:   result.Message,
		Timestamp: time.Now(),
		Data: map[string]any{
			"steps": string(stepsJSON),
		},
	}, nil
}

func (e *TaskExecutor) deploySingleInstance(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	host, _ := req.Config["host"].(string)
	port := configInt(req.Config, "port")
	dataDir, _ := req.Config["datadir"].(string)
	if dataDir == "" {
		dataDir, _ = req.Config["data_dir"].(string)
	}
	packageURL, _ := req.Config["package_url"].(string)
	checksum, _ := req.Config["checksum"].(string)
	basedir, _ := req.Config["basedir"].(string)
	osUser, _ := req.Config["os_user"].(string)
	mysqlUser, _ := req.Config["mysql_user"].(string)
	mysqlPass, _ := req.Config["mysql_pass"].(string)
	serverID := configInt(req.Config, "server_id")

	// MGR配置参数
	installType, _ := req.Config["install_type"].(string)
	isPrimary, _ := req.Config["is_primary"].(bool)
	groupName, _ := req.Config["group_name"].(string)
	localAddress, _ := req.Config["local_address"].(string)
	seeds, _ := req.Config["seeds"].(string)

	replicateUser, _ := req.Config["replicate_user"].(string)
	if replicateUser == "" {
		replicateUser, _ = req.Config["repl_user"].(string)
	}
	replicatePass, _ := req.Config["replicate_pass"].(string)
	if replicatePass == "" {
		replicatePass, _ = req.Config["repl_pass"].(string)
	}

	mysqlVersion, _ := req.Config["mysql_version"].(string)
	if mysqlVersion == "" {
		mysqlVersion = "8.0"
	}

	if host == "" {
		host = "127.0.0.1"
	}
	if port == 0 {
		port = 3306
	}
	if serverID == 0 {
		serverID = port
	}
	if dataDir == "" {
		dataDir = fmt.Sprintf("/data/mysql/%d", port)
	}
	if osUser == "" {
		osUser = "mysql"
	}
	if err := ensureOSUser(osUser); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("ensure OS user failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	// Check if data directory exists and has been initialized.
	// If so, skip initialization and just start MySQL.
	// For MGR deployments, always re-initialize to get a clean group state.
	needInit := true
	// When a MySQL password is explicitly provided, always re-initialize
	// to ensure clean auth plugin setup (avoids stale mysql_native_password
	// users from previous deployments on MySQL 8.4+ where the plugin is removed).
	if installType != "mgr" && mysqlPass == "" {
		if _, err := os.Stat(dataDir); err == nil {
			if isDataDirInitialized(dataDir) {
				needInit = false
			}
		}
	}

	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("create data dir failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	uid, gid := lookupUserIDs(osUser)
	if err := chownRecursive(dataDir, uid, gid); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("chown data dir failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	if err := ensureParentTraversal(dataDir); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("prepare data dir parent permissions failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	// Version-agnostic path: download + extract the requested tarball.
	// Priority: 1) relay_url  2) package_url  3) PATH
	var mysqld string
	relayURL, _ := req.Config["relay_url"].(string)
	if relayURL != "" {
		downloader := NewRelayDownloader()
		flavor := "mysql"
		mysqldVersion := mysqlVersion
		relayCfg := RelayDownloadConfig{
			RelayURL: relayURL,
			Branch:   flavor,
			Version:  mysqldVersion,
			Basedir:  basedir,
			OSUser:   osUser,
			Checksum: checksum,
		}
		var relayErr error
		mysqld, relayErr = downloader.DownloadFromRelay(ctx, relayCfg)
		if relayErr != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  0,
				Message:   fmt.Sprintf("install from relay failed: %v", relayErr),
				Timestamp: time.Now(),
			}, nil
		}
	} else if packageURL != "" {
		var installErr error
		mysqld, installErr = InstallFromURL(ctx, packageURL, checksum, basedir, osUser)
		if installErr != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  0,
				Message:   fmt.Sprintf("install from URL failed: %v", installErr),
				Timestamp: time.Now(),
			}, nil
		}
	} else {
		// Legacy: rely on PATH
		mysqld = resolveBinaryPath("mysqld")
		if basedir != "" {
			mysqld = filepath.Join(basedir, "bin", "mysqld")
		}
	}

	// Kill only the mysqld process using the target port (not all mysqld processes)
	socketPath := fmt.Sprintf("/tmp/mysql_%d.sock", port)
	killBySocket := exec.CommandContext(ctx, "sh", "-c",
		fmt.Sprintf("fuser -k %s 2>/dev/null || lsof -ti :%d | xargs kill -15 2>/dev/null || true", socketPath, port))
	killBySocket.Run()
	time.Sleep(2 * time.Second)

	// Only run initialization if the data directory hasn't been initialized yet.
	if needInit {
		if rmErr := os.RemoveAll(dataDir); rmErr != nil {
		} else {
		}

		initArgs := []string{
			"--no-defaults",
			"--initialize-insecure",
			"--datadir=" + dataDir,
		}
		if osUser != "" {
			initArgs = append(initArgs, "--user="+osUser)
		}
		if basedir != "" {
			initArgs = append(initArgs, "--basedir="+basedir)
		}
		initCmd := exec.CommandContext(ctx, mysqld, initArgs...)
		if out, err := initCmd.CombinedOutput(); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  0,
				Message:   fmt.Sprintf("MySQL initialization failed: %v, output: %s", err, strings.TrimSpace(string(out))),
				Timestamp: time.Now(),
			}, nil
		}
	}

	startArgs := []string{
		"--no-defaults",
		"--daemonize",
		"--datadir=" + dataDir,
		"--port=" + fmt.Sprintf("%d", port),
		"--server-id=" + fmt.Sprintf("%d", serverID),
		"--log-bin=mysql-bin",
		"--binlog-format=ROW",
		"--gtid-mode=ON",
		"--enforce-gtid-consistency=ON",
		"--log-slave-updates=ON",
		"--bind-address=0.0.0.0",
		"--socket=" + filepath.Join(dataDir, "mysql.sock"),
		"--pid-file=" + filepath.Join(dataDir, "mysql.pid"),
		"--log-error=" + filepath.Join(dataDir, "error.log"),
	}

	// 如果是MGR部署，添加MGR配置
	if installType == "mgr" && groupName != "" && localAddress != "" {
		startArgs = append(startArgs,
			"--plugin-load-add=group_replication.so",
			"--group_replication_group_name="+groupName,
			"--group_replication_local_address="+localAddress,
			"--group_replication_bootstrap_group=OFF",
			"--disabled_storage_engines=MyISAM,BLACKHOLE,FEDERATED,ARCHIVE,MEMORY",
		)

		// 如果有seeds（副本节点），添加seeds配置
		if seeds != "" {
			startArgs = append(startArgs, "--group_replication_group_seeds="+seeds)
		} else {
			// 主节点也设置seeds为自己的地址，便于后续副本加入
			startArgs = append(startArgs, "--group_replication_group_seeds="+localAddress)
		}
	}

	if osUser != "" {
		startArgs = append(startArgs, "--user="+osUser)
	}
	if basedir != "" {
		startArgs = append(startArgs, "--basedir="+basedir)
	}

	startCmd := exec.CommandContext(ctx, mysqld, startArgs...)
	if out, err := startCmd.CombinedOutput(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("MySQL start failed: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}, nil
	}

	time.Sleep(5 * time.Second)

	// Wait for the MySQL socket file to exist before attempting password setup.
	// After --initialize-insecure and --daemonize, the socket file is created
	// asynchronously. Without this wait, the socket-based retrySQL would fail
	// silently and root@% would never be created.
	dataSocketPath := filepath.Join(dataDir, "mysql.sock")
	for i := 0; i < 30; i++ {
		if _, err := os.Stat(dataSocketPath); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	var healthArgs []string
	healthArgs = append(healthArgs, resolveBinaryPath("mysqladmin"), "-h", host, "-P", fmt.Sprintf("%d", port), "-u", defaultString(mysqlUser, "root"))
	if mysqlHasGetServerPublicKey(mysqlVersion) {
		healthArgs = append(healthArgs, "--get-server-public-key")
	}
	healthArgs = append(healthArgs, "ping")
	healthCmd := exec.CommandContext(ctx, healthArgs[0], healthArgs[1:]...)
	if mysqlPass != "" {
		healthCmd.Env = append(os.Environ(), "MYSQL_PWD="+mysqlPass)
	}
	healthErr := healthCmd.Run()

	// Always set up password when configured, even if health check passed.
	// mysqladmin ping can succeed without valid credentials (server is alive)
	// but root@% might not exist yet, causing Access denied for cross-host connections.
	if mysqlPass != "" {
		if mysqlUser == "" {
			mysqlUser = "root"
		}
		socketPath := filepath.Join(dataDir, "mysql.sock")
		retrySQL := fmt.Sprintf(
			"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
				"ALTER USER '%s'@'%%' IDENTIFIED BY '%s'; "+
				"ALTER USER '%s'@'localhost' IDENTIFIED BY '%s'; "+
				"GRANT ALL PRIVILEGES ON *.* TO '%s'@'%%' WITH GRANT OPTION; FLUSH PRIVILEGES;",
			escapeSQL(mysqlUser), escapeSQL(mysqlPass),
			escapeSQL(mysqlUser), escapeSQL(mysqlPass),
			escapeSQL(mysqlUser), escapeSQL(mysqlPass),
			escapeSQL(mysqlUser),
		)
		retryCmd := exec.CommandContext(ctx, resolveBinaryPath("mysql"), "-S", socketPath, "-u", mysqlUser, "-e", retrySQL)
		retryOut, retryErr := retryCmd.CombinedOutput()
		if retryErr != nil {
			fmt.Fprintf(os.Stderr, "Password setup via socket failed: %v, output: %s\n", retryErr, string(retryOut))
		}

		// Try health check again after password setup
		var retryHealthArgs []string
		retryHealthArgs = append(retryHealthArgs, resolveBinaryPath("mysqladmin"), "-h", host, "-P", fmt.Sprintf("%d", port), "-u", mysqlUser)
		if mysqlHasGetServerPublicKey(mysqlVersion) {
			retryHealthArgs = append(retryHealthArgs, "--get-server-public-key")
		}
		retryHealthArgs = append(retryHealthArgs, "ping")
		retryHealthCmd := exec.CommandContext(ctx, retryHealthArgs[0], retryHealthArgs[1:]...)
		retryHealthCmd.Env = append(os.Environ(), "MYSQL_PWD="+mysqlPass)
		if retryHealthCmd.Run() == nil {
			// Success! Now initialize MGR if needed
			if installType == "mgr" && groupName != "" {
				if err := initializeMGR(ctx, host, port, mysqlUser, mysqlPass, isPrimary, groupName, replicateUser, replicatePass, mysqlVersion); err != nil {
					return &TaskResult{
						TaskID:    req.TaskID,
						Status:    "failed",
						Progress:  90,
						Message:   fmt.Sprintf("MGR initialization failed: %v", err),
						Timestamp: time.Now(),
					}, nil
				}
			}

			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "completed",
				Progress:  100,
				Message:   fmt.Sprintf("MySQL instance deployed successfully on %s:%d", host, port),
				Timestamp: time.Now(),
			}, nil
		} else {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  80,
				Message:   fmt.Sprintf("Health check failed after password retry: %v", healthErr),
				Timestamp: time.Now(),
			}, nil
		}
	}

	if healthErr != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("Health check failed: %v", healthErr),
			Timestamp: time.Now(),
		}, nil
	}

	// Health check passed. Initialize MGR if needed
	if installType == "mgr" && groupName != "" {
		if err := initializeMGR(ctx, host, port, mysqlUser, mysqlPass, isPrimary, groupName, replicateUser, replicatePass, mysqlVersion); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  90,
				Message:   fmt.Sprintf("MGR initialization failed: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("MySQL instance deployed successfully on %s:%d", host, port),
		Timestamp: time.Now(),
	}, nil
}

// initializeMGR initializes MySQL Group Replication
// Uses mysqlExecWithSocketFallback which tries TCP first, then falls back to
// Unix socket authentication. This resolves the ERROR 1130 issue where
// MySQL 8.0 creates only root@localhost (no root@%), so TCP auth fails
// but socket auth works.
func initializeMGR(ctx context.Context, host string, port int, user, pass string, isPrimary bool, groupName, replicateUser, replicatePass, mysqlVersion string) error {
	if replicateUser == "" {
		replicateUser = "repl"
	}
	if replicatePass == "" {
		replicatePass = "repl"
	}
	// 0. 关闭super-read-only以便创建用户
	if _, err := mysqlExecWithSocketFallback(ctx, host, port, user, pass, "SET GLOBAL super_read_only=0; SET GLOBAL read_only=0;"); err != nil {
		return fmt.Errorf("disable read-only failed: %v", err)
	}

	// 1. 使用配置的复制用户（主节点和副本都需要）
	// MySQL 5.7 使用 mysql_native_password + CHANGE MASTER TO + 精简权限
	// MySQL 8.0 使用 caching_sha2_password + CHANGE REPLICATION SOURCE TO + 扩展权限
	var replicationSQL string
	if strings.HasPrefix(mysqlVersion, "5.") {
		replicationSQL = fmt.Sprintf(`
		SET SQL_LOG_BIN=0;
		CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s';
		GRANT REPLICATION SLAVE ON *.* TO '%s'@'%%';
		FLUSH PRIVILEGES;
		SET SQL_LOG_BIN=1;
		CHANGE MASTER TO MASTER_USER='%s', MASTER_PASSWORD='%s' FOR CHANNEL 'group_replication_recovery';
	`, escapeSQL(replicateUser), escapeSQL(replicatePass), escapeSQL(replicateUser), escapeSQL(replicateUser), escapeSQL(replicatePass))
	} else {
		replicationSQL = fmt.Sprintf(`
		SET SQL_LOG_BIN=0;
		CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED WITH caching_sha2_password BY '%s';
		GRANT REPLICATION SLAVE, CONNECTION_ADMIN, BACKUP_ADMIN, GROUP_REPLICATION_STREAM ON *.* TO '%s'@'%%';
		FLUSH PRIVILEGES;
		SET SQL_LOG_BIN=1;
		CHANGE REPLICATION SOURCE TO SOURCE_USER='%s', SOURCE_PASSWORD='%s' FOR CHANNEL 'group_replication_recovery';
	`, escapeSQL(replicateUser), escapeSQL(replicatePass), escapeSQL(replicateUser), escapeSQL(replicateUser), escapeSQL(replicatePass))
	}
	if _, err := mysqlExecWithSocketFallback(ctx, host, port, user, pass, replicationSQL); err != nil {
		return fmt.Errorf("create replication user failed: %v", err)
	}

	// 2. 启动Group Replication
	if isPrimary {
		bootstrapSQL := `
			SET GLOBAL group_replication_bootstrap_group=ON;
			START GROUP_REPLICATION;
			SET GLOBAL group_replication_bootstrap_group=OFF;
		`
		if _, err := mysqlExecWithSocketFallback(ctx, host, port, user, pass, bootstrapSQL); err != nil {
			return fmt.Errorf("bootstrap MGR failed: %v", err)
		}
	} else {
		if _, err := mysqlExecWithSocketFallback(ctx, host, port, user, pass, "START GROUP_REPLICATION;"); err != nil {
			return fmt.Errorf("start MGR failed: %v", err)
		}
	}

	// 3. 等待MGR启动
	time.Sleep(3 * time.Second)

	// 4. 验证MGR状态
	out, err := mysqlExecWithSocketFallback(ctx, host, port, user, pass, "SELECT MEMBER_STATE FROM performance_schema.replication_group_members WHERE MEMBER_ID=@@server_uuid;")
	if err != nil {
		return fmt.Errorf("check MGR status failed: %v", err)
	}

	state := strings.TrimSpace(string(out))
	if state != "ONLINE" {
		return fmt.Errorf("MGR member state is '%s', expected 'ONLINE'", state)
	}

	return nil
}
func (e *TaskExecutor) deployMasterSlave(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMasterSlaveConfig(req.Config)

	masterResult := e.configureMaster(ctx, config)
	if masterResult.Status == "failed" {
		return masterResult, nil
	}

	slaveResult := e.configureSlave(ctx, config)
	if slaveResult.Status == "failed" {
		return slaveResult, nil
	}

	replicationResult := e.verifyReplication(ctx, config)

	return replicationResult, nil
}

func (e *TaskExecutor) deployHAMaster(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMasterSlaveConfig(req.Config)
	result := e.configureMaster(ctx, config)
	if result.TaskID == "" {
		result.TaskID = req.TaskID
	}
	if result.Status == "completed" {
		if err := applyDeployMySQLConfig(ctx, config.MasterHost, config.MasterPort, config.MySQLUser, config.MySQLPass, deployMySQLConfig(req.Config)); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  90,
				Message:   err.Error(),
				Timestamp: time.Now(),
			}, nil
		}
		result.Progress = 100
		result.Message = fmt.Sprintf("HA master configured successfully on %s:%d", config.MasterHost, config.MasterPort)
	}
	return result, nil
}

func (e *TaskExecutor) deployHAReplica(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMasterSlaveConfig(req.Config)
	// First, create replication user on master
	if masterResult := e.configureMaster(ctx, config); masterResult.Status == "failed" {
		masterResult.TaskID = req.TaskID
		return masterResult, nil
	}
	result := e.configureSlave(ctx, config)
	if result.Status == "failed" {
		if result.TaskID == "" {
			result.TaskID = req.TaskID
		}
		return result, nil
	}
	if err := applyDeployMySQLConfig(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, deployMySQLConfig(req.Config)); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  85,
			Message:   err.Error(),
			Timestamp: time.Now(),
		}, nil
	}
	verifyResult := e.verifyReplication(ctx, config)
	if verifyResult.TaskID == "" {
		verifyResult.TaskID = req.TaskID
	}
	return verifyResult, nil
}

func parseMasterSlaveConfig(config map[string]interface{}) MasterSlaveConfig {
	mc := MasterSlaveConfig{
		MasterHost:    "localhost",
		MasterPort:    3306,
		SlaveHost:     "localhost",
		SlavePort:     3307,
		ReplicateUser: "repl",
		ReplicatePass: "",
		MySQLUser:     "root",
		ServerID:      1,
		DeployMode:    "async",
	}

	if v, ok := config["master_host"].(string); ok {
		mc.MasterHost = v
	}
	if v := configInt(config, "master_port"); v != 0 {
		mc.MasterPort = v
	}
	if v, ok := config["slave_host"].(string); ok {
		mc.SlaveHost = v
	}
	if v := configInt(config, "slave_port"); v != 0 {
		mc.SlavePort = v
	}
	if v, ok := config["replicate_user"].(string); ok {
		mc.ReplicateUser = v
	}
	if v, ok := config["replicate_pass"].(string); ok {
		mc.ReplicatePass = v
	}
	if v, ok := config["mysql_user"].(string); ok && v != "" {
		mc.MySQLUser = v
	}
	if v, ok := config["mysql_password"].(string); ok {
		mc.MySQLPass = v
	}
	if v, ok := config["mysql_pass"].(string); ok && mc.MySQLPass == "" {
		mc.MySQLPass = v
	}
	if v := configInt(config, "server_id"); v != 0 {
		mc.ServerID = v
	}

	return mc
}

func (e *TaskExecutor) configureMaster(ctx context.Context, config MasterSlaveConfig) *TaskResult {
	createUserSQL := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO '%s'@'%%';",
		config.ReplicateUser, config.ReplicatePass, config.ReplicateUser,
	)

	// Try TCP first, then fall back to socket auth (handles mismatched root@localhost / root@% credentials)
	if out, err := mysqlExecWithSocketFallback(ctx, config.MasterHost, config.MasterPort, config.MySQLUser, config.MySQLPass, createUserSQL); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  20,
			Message:   fmt.Sprintf("Failed to create replication user on master: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  40,
		Message:   "Master configured successfully with replication user",
		Timestamp: time.Now(),
	}
}

func (e *TaskExecutor) configureSlave(ctx context.Context, config MasterSlaveConfig) *TaskResult {
	serverID := config.ServerID
	if serverID <= 0 {
		serverID = readCurrentServerID(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass)
		if serverID <= 0 {
			serverID = config.SlavePort
		}
	}
	if out, err := setServerID(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, serverID); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Failed to set slave server_id: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}

	masterStatusOut, err := readBinaryLogStatus(ctx, config)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  55,
			Message:   fmt.Sprintf("Failed to read master status: %v, output: %s", err, strings.TrimSpace(string(masterStatusOut))),
			Timestamp: time.Now(),
		}
	}
	masterLogFile, masterLogPos := parseMasterStatus(string(masterStatusOut))
	if masterLogFile == "" || masterLogPos == "" {
		return &TaskResult{
			Status:    "failed",
			Progress:  55,
			Message:   "Master binary log is not enabled or SHOW MASTER STATUS returned no position",
			Timestamp: time.Now(),
		}
	}

	if out, err := resetReplication(ctx, config); err != nil {
		output := strings.TrimSpace(string(out))
		if !strings.Contains(output, "Slave is not configured") && !strings.Contains(output, "Replica is not configured") {
			return &TaskResult{
				Status:    "failed",
				Progress:  58,
				Message:   fmt.Sprintf("Failed to reset existing slave configuration: %v, output: %s", err, output),
				Timestamp: time.Now(),
			}
		}
	}

	changeMasterSQLNoKey := fmt.Sprintf(
		"CHANGE MASTER TO "+
			"MASTER_HOST='%s', "+
			"MASTER_PORT=%d, "+
			"MASTER_USER='%s', "+
			"MASTER_PASSWORD='%s', "+
			"MASTER_LOG_FILE='%s', "+
			"MASTER_LOG_POS=%s;",
		config.MasterHost, config.MasterPort,
		config.ReplicateUser, config.ReplicatePass,
		masterLogFile, masterLogPos,
	)
	changeMasterSQL := fmt.Sprintf(
		"CHANGE MASTER TO "+
			"MASTER_HOST='%s', "+
			"MASTER_PORT=%d, "+
			"MASTER_USER='%s', "+
			"MASTER_PASSWORD='%s', "+
			"MASTER_LOG_FILE='%s', "+
			"MASTER_LOG_POS=%s, "+
			"GET_MASTER_PUBLIC_KEY=1;",
		config.MasterHost, config.MasterPort,
		config.ReplicateUser, config.ReplicatePass,
		masterLogFile, masterLogPos,
	)

	changeSourceSQLNoKey := fmt.Sprintf(
		"CHANGE REPLICATION SOURCE TO "+
			"SOURCE_HOST='%s', "+
			"SOURCE_PORT=%d, "+
			"SOURCE_USER='%s', "+
			"SOURCE_PASSWORD='%s', "+
			"SOURCE_LOG_FILE='%s', "+
			"SOURCE_LOG_POS=%s;",
		config.MasterHost, config.MasterPort,
		config.ReplicateUser, config.ReplicatePass,
		masterLogFile, masterLogPos,
	)
	changeSourceSQL := fmt.Sprintf(
		"CHANGE REPLICATION SOURCE TO "+
			"SOURCE_HOST='%s', "+
			"SOURCE_PORT=%d, "+
			"SOURCE_USER='%s', "+
			"SOURCE_PASSWORD='%s', "+
			"SOURCE_LOG_FILE='%s', "+
			"SOURCE_LOG_POS=%s, "+
			"GET_SOURCE_PUBLIC_KEY=1;",
		config.MasterHost, config.MasterPort,
		config.ReplicateUser, config.ReplicatePass,
		masterLogFile, masterLogPos,
	)

	// Use mysqlExecWithSocketFallback on the slave host for CHANGE MASTER TO
	// Try all 4 SQL variants in sequence (pre-8.4/8.4 syntax × with-key/no-key)
	var changeMasterErr error
	// Try with GET_SOURCE_PUBLIC_KEY=1 first (MySQL 8.4+), then GET_MASTER_PUBLIC_KEY=1, then no-key fallbacks
	for _, changeSQL := range []string{changeSourceSQL, changeMasterSQL, changeSourceSQLNoKey, changeMasterSQLNoKey} {
		if _, err := mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, changeSQL); err == nil {
			changeMasterErr = nil
			break
		} else {
			changeMasterErr = err
		}
	}
	if changeMasterErr != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("Failed to execute CHANGE MASTER TO: %v", changeMasterErr),
			Timestamp: time.Now(),
		}
	}

	// Use mysqlExecWithSocketFallback for START SLAVE
	var startSlaveErr error
	for _, startSQL := range []string{"START SLAVE;", "START REPLICA;"} {
		if _, err := mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, startSQL); err == nil {
			startSlaveErr = nil
			break
		} else {
			startSlaveErr = err
		}
	}
	if startSlaveErr != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  70,
			Message:   fmt.Sprintf("Failed to start slave: %v", startSlaveErr),
			Timestamp: time.Now(),
		}
	}

	// Leave verifyReplication in deployHAReplica for the final check.
	// Return partial progress so deployHAReplica proceeds to verify.
	return &TaskResult{
		Status:    "completed",
		Progress:  80,
		Message:   "Slave configured and started successfully",
		Timestamp: time.Now(),
	}
}

// mysqlExecWithSocketFallback tries to execute a MySQL command via TCP first,
// then falls back to the local Unix socket if TCP fails.
// This addresses the case where root@% user doesn't exist (MySQL 8.0 only creates root@localhost),
// making TCP authentication fail while socket authentication still works.
func mysqlExecWithSocketFallback(ctx context.Context, host string, port int, user, pass, sql string) ([]byte, error) {
	cmd := mysqlExecCommand(ctx, host, port, user, pass, sql)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}
	tcpErr := fmt.Sprintf("TCP: %v - %s", err, strings.TrimSpace(string(out)))

	socketPath := filepath.Join("/data/mysql", fmt.Sprintf("%d", port), "mysql.sock")

	// Fallback 1: socket WITHOUT password (for auth_socket plugin in MySQL 8.0)
	noPassCmd := exec.CommandContext(ctx, resolveBinaryPath("mysql"), "-S", socketPath, "-u", user, "-N", "-B", "-e", sql)
	noPassOut, noPassErr := noPassCmd.CombinedOutput()
	if noPassErr == nil {
		return noPassOut, nil
	}

	// Fallback 2: socket WITH password (for mysql_native_password after ALTER USER)
	withPassCmd := exec.CommandContext(ctx, resolveBinaryPath("mysql"), "-S", socketPath, "-u", user, "-N", "-B", "-e", sql)
	if pass != "" {
		withPassCmd.Env = append(os.Environ(), "MYSQL_PWD="+pass)
	}
	withPassOut, withPassErr := withPassCmd.CombinedOutput()
	if withPassErr == nil {
		return withPassOut, nil
	}

	// All failed, return original out with detailed diagnostic error
	return noPassOut, fmt.Errorf("%s; socket(no-pass): %v - %s; socket(with-pass): %v - %s",
		tcpErr,
		noPassErr, strings.TrimSpace(string(noPassOut)),
		withPassErr, strings.TrimSpace(string(withPassOut)))
}
func setServerID(ctx context.Context, host string, port int, user, pass string, serverID int) ([]byte, error) {
	return mysqlExecWithSocketFallback(ctx, host, port, user, pass,
		fmt.Sprintf("SET GLOBAL server_id=%d;", serverID),
	)
}

func readCurrentServerID(ctx context.Context, host string, port int, user, pass string) int {
	out, err := mysqlExecWithSocketFallback(ctx, host, port, user, pass,
		"SELECT @@server_id",
	)
	if err != nil {
		return 0
	}
	text := strings.TrimSpace(string(out))
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if id, err := strconv.Atoi(line); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func resetReplication(ctx context.Context, config MasterSlaveConfig) ([]byte, error) {
	// Try REPLICA syntax first (MySQL 8.4+), fall back to SLAVE for older versions
	replicaOut, replicaErr := mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP REPLICA; RESET REPLICA ALL;")
	replicaText := string(replicaOut)
		if replicaErr == nil || strings.Contains(replicaText, "not configured as replica") || strings.Contains(replicaText, "not configured as slave") || strings.Contains(replicaErr.Error(), "not configured as replica") || strings.Contains(replicaErr.Error(), "not configured as slave") {
		return replicaOut, nil
	}
	// Check both output and error for ERROR 1064 to handle the case where
	// mysqlExecWithSocketFallback returns output from socket (ERROR 1045)
	// while the TCP attempt had the 1064 syntax error.
	has1064 := strings.Contains(replicaText, "ERROR 1064") ||
		strings.Contains(replicaErr.Error(), "ERROR 1064")
	if !has1064 {
		return replicaOut, replicaErr
	}
	legacyOut, legacyErr := mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP SLAVE; RESET SLAVE ALL;")
	legacyText := string(legacyOut)
		if legacyErr == nil || strings.Contains(legacyText, "not configured as slave") || strings.Contains(legacyText, "not configured as replica") || strings.Contains(legacyErr.Error(), "not configured as slave") || strings.Contains(legacyErr.Error(), "not configured as replica") {
		return legacyOut, nil
	}
	return legacyOut, legacyErr
}

func runFirstMySQL(ctx context.Context, host string, port int, user, pass string, queries ...string) ([]byte, error) {
	var lastOut []byte
	var lastErr error
	failures := make([]string, 0, len(queries))
	for _, query := range queries {
		cmd := mysqlExecCommand(ctx, host, port, user, pass, query)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
		lastOut = out
		lastErr = err
		failures = append(failures, fmt.Sprintf("%s => %v: %s", strings.TrimSuffix(query, ";"), err, strings.TrimSpace(string(out))))
	}
	if len(failures) > 0 {
		return []byte(strings.Join(failures, "\n")), lastErr
	}
	return lastOut, lastErr
}

func readBinaryLogStatus(ctx context.Context, config MasterSlaveConfig) ([]byte, error) {
	commands := []string{"SHOW MASTER STATUS;", "SHOW BINARY LOG STATUS;"}
	var lastOut []byte
	var lastErr error
	for _, query := range commands {
		cmd := mysqlExecCommand(ctx, config.MasterHost, config.MasterPort, config.MySQLUser, config.MySQLPass, query)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return out, nil
		}
		lastOut = out
		lastErr = err
	}
	return lastOut, lastErr
}

func parseMasterStatus(output string) (string, string) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.Contains(fields[0], ".") {
			return fields[0], fields[1]
		}
	}
	return "", ""
}

func (e *TaskExecutor) verifyReplication(ctx context.Context, config MasterSlaveConfig) *TaskResult {
	time.Sleep(3 * time.Second)

	// Use performance_schema queries which work with -N -B batch output
	// (mysqlExecWithSocketFallback uses -N -B for socket fallback).
	// This avoids SHOW SLAVE STATUS\G field-label parsing issues.
	output, err := mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass,
		"SELECT COALESCE((SELECT SERVICE_STATE FROM performance_schema.replication_connection_status LIMIT 1), ''), COALESCE((SELECT SERVICE_STATE FROM performance_schema.replication_applier_status LIMIT 1), '')")
	if err == nil {
		fields := strings.Fields(strings.TrimSpace(string(output)))
		if len(fields) >= 2 && (fields[0] == "ON" || fields[0] == "Yes") && (fields[1] == "ON" || fields[1] == "Yes") {
			return &TaskResult{
				Status:   "completed",
				Progress: 100,
				Message: fmt.Sprintf("Master-Slave replication established successfully. Master: %s:%d, Slave: %s:%d",
					config.MasterHost, config.MasterPort, config.SlaveHost, config.SlavePort),
				Timestamp: time.Now(),
			}
		}
	}

	// Fallback: try SHOW SLAVE STATUS (tabular, no \G) for MySQL 5.7
		for _, query := range []string{"SHOW SLAVE STATUS", "SHOW REPLICA STATUS"} {
			output, err = mysqlExecWithSocketFallback(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, query)
		if err == nil && len(output) > 0 {
			yesCount := strings.Count(string(output), "Yes")
			if yesCount >= 2 {
				return &TaskResult{
					Status:   "completed",
					Progress: 100,
					Message: fmt.Sprintf("Master-Slave replication established successfully. Master: %s:%d, Slave: %s:%d",
						config.MasterHost, config.MasterPort, config.SlaveHost, config.SlavePort),
					Timestamp: time.Now(),
				}
			}
		}
	}

	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  90,
			Message:   fmt.Sprintf("Failed to check replica status: %v, output: %s", err, strings.TrimSpace(string(output))),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "failed",
		Progress:  95,
		Message:   "Replication not running correctly: " + strings.TrimSpace(string(output)),
		Timestamp: time.Now(),
	}
}

func (e *TaskExecutor) ExecuteBackup(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseBackupConfig(req.Config)
	method := config.BackupMethod
	if method == "" || method == "auto" {
		var err error
		method, err = e.selectBackupMethod(ctx, config)
		if err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  0,
				Message:   err.Error(),
				Timestamp: time.Now(),
			}, nil
		}
	}
	switch method {
	case "mysqldump":
		return e.executeLogicalBackup(ctx, config, "database size is below 10G, used mysqldump")
	case "mysqlbinlog":
		return e.executeGTIDIncrementalBackup(ctx, config)
	case "xtrabackup":
		return e.executeXtrabackup(ctx, config, false)
	case "cold_datadir":
		return e.executeColdDatadirBackup(ctx, config)
	default:
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   "unsupported backup method: " + method,
			Timestamp: time.Now(),
		}, nil
	}
}

const xtrabackupSizeThresholdBytes int64 = 10 * 1024 * 1024 * 1024

func (e *TaskExecutor) selectBackupMethod(ctx context.Context, config BackupConfig) (string, error) {
	if strings.EqualFold(config.BackupType, "incremental") {
		if isLogicalDumpBackupPath(config.BaseBackupPath) {
			return "mysqlbinlog", nil
		}
		return "xtrabackup", nil
	}
	size := config.DatabaseSize
	if size <= 0 {
		var err error
		size, err = estimateMySQLDataSize(ctx, config)
		if err != nil {
			if config.BackupType == "full" && strings.TrimSpace(config.DataDir) != "" && !mysqlPortListening(config.MySQLHost, config.MySQLPort) {
				return "cold_datadir", nil
			}
			return "", fmt.Errorf("estimate database size failed before backup method selection: %w", err)
		}
	}
	if size > xtrabackupSizeThresholdBytes {
		return "xtrabackup", nil
	}
	return "mysqldump", nil
}

func estimateMySQLDataSize(ctx context.Context, config BackupConfig) (int64, error) {
	query := "SELECT COALESCE(SUM(data_length + index_length),0) FROM information_schema.tables WHERE table_schema NOT IN ('mysql','information_schema','performance_schema','sys')"
	var lastErr error
	for _, host := range backupMySQLHostCandidates(config.MySQLHost) {
		cmd := exec.CommandContext(ctx, "mysql", "-N", "-B", "-h", host, "-P", fmt.Sprintf("%d", config.MySQLPort), "-u", defaultString(config.MySQLUser, "root"), "-e", query)
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+config.MySQLPass)
		out, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("host=%s: %v %s", host, err, strings.TrimSpace(string(out)))
			continue
		}
		size, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			lastErr = fmt.Errorf("host=%s: parse size %q: %w", host, strings.TrimSpace(string(out)), err)
			continue
		}
		return size, nil
	}
	if lastErr != nil {
		return 0, lastErr
	}
	return 0, fmt.Errorf("no MySQL host candidates available")
}

func parseBackupConfig(config map[string]interface{}) BackupConfig {
	bc := BackupConfig{
		BackupType: "full",
		TargetDir:  "/backup/mysql",
	}

	if v, ok := config["backup_type"].(string); ok {
		bc.BackupType = v
	}
	if v, ok := config["backup_method"].(string); ok {
		bc.BackupMethod = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := config["target_dir"].(string); ok {
		bc.TargetDir = v
	}
	if v, ok := config["mysql_host"].(string); ok {
		bc.MySQLHost = v
	}
	if v, ok := config["mysql_port"].(int); ok {
		bc.MySQLPort = v
	} else if v, ok := config["mysql_port"].(float64); ok {
		bc.MySQLPort = int(v)
	}
	if v, ok := config["mysql_user"].(string); ok {
		bc.MySQLUser = v
	}
	if v, ok := config["mysql_pass"].(string); ok {
		bc.MySQLPass = v
	}
	if v, ok := config["datadir"].(string); ok {
		bc.DataDir = strings.TrimSpace(v)
	}
	if v, ok := config["base_backup_path"].(string); ok {
		bc.BaseBackupPath = v
	}
	if v, ok := config["base_backup_label"].(string); ok {
		bc.BaseBackupLabel = v
	}
	bc.DatabaseSize = int64Config(config, "database_size")

	return bc
}

func (e *TaskExecutor) executeXtrabackup(ctx context.Context, config BackupConfig, allowLogicalFallback bool) (*TaskResult, error) {
	if _, err := exec.LookPath("xtrabackup"); err != nil {
		if config.BackupType == "incremental" || !allowLogicalFallback {
			return &TaskResult{
				Status:    "failed",
				Progress:  0,
				Message:   "xtrabackup is required for this backup but was not found on target host",
				Timestamp: time.Now(),
			}, nil
		}
		return e.executeLogicalBackup(ctx, config, "xtrabackup not found, used logical full backup")
	}
	backupDir := fmt.Sprintf("%s/%s-%d", config.TargetDir, config.BackupType, time.Now().UnixNano())
	if err := os.MkdirAll(config.TargetDir, 0o755); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("create backup root failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	if config.BackupType == "incremental" && config.BaseBackupPath == "" {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   "incremental backup requires a completed full backup",
			Timestamp: time.Now(),
		}, nil
	}

	var lastBackupErr string
	for _, host := range backupMySQLHostCandidates(config.MySQLHost) {
		_ = os.RemoveAll(backupDir)
		backupCmd := exec.CommandContext(ctx, "xtrabackup",
			"--backup",
			"--target-dir="+backupDir,
			"--host="+host,
			"--port="+fmt.Sprintf("%d", config.MySQLPort),
			"--user="+config.MySQLUser,
			"--password="+config.MySQLPass,
		)
		if config.BackupType == "incremental" {
			backupCmd.Args = append(backupCmd.Args, "--incremental-basedir="+config.BaseBackupPath)
		}
		if out, err := backupCmd.CombinedOutput(); err != nil {
			lastBackupErr = fmt.Sprintf("host=%s: %v\n%s", host, err, strings.TrimSpace(string(out)))
			continue
		}
		lastBackupErr = ""
		break
	}
	if lastBackupErr != "" {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   "Xtrabackup backup failed: " + lastBackupErr,
			Timestamp: time.Now(),
		}, nil
	}

	if config.BackupType != "incremental" {
		prepareCmd := exec.CommandContext(ctx, "xtrabackup",
			"--prepare",
			"--target-dir="+backupDir,
		)

		if out, err := prepareCmd.CombinedOutput(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  50,
				Message:   fmt.Sprintf("Xtrabackup prepare failed: %v\n%s", err, strings.TrimSpace(string(out))),
				Timestamp: time.Now(),
			}, nil
		}
	}

	if !isCompleteXtrabackupDir(backupDir) {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   "xtrabackup completed but backup directory is incomplete: " + backupDir,
			Timestamp: time.Now(),
		}, nil
	}
	sizeBytes, err := pathSize(backupDir)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("calculate backup size failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	checksum, err := checksumPath(backupDir)
	if err != nil {
		// 真没拿到 hash 时标 failed, 不能再假装 "checksum-unavailable" 然后 completed
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("calculate backup checksum failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Backup completed successfully. Path: %s, Checksum: %s", backupDir, checksum),
		Timestamp: time.Now(),
		Data: map[string]any{
			"backup_path":       backupDir,
			"backup_type":       config.BackupType,
			"backup_method":     "xtrabackup",
			"base_backup_path":  config.BaseBackupPath,
			"base_backup_label": config.BaseBackupLabel,
			"file_size":         sizeBytes,
			"checksum":          checksum,
		},
	}, nil
}

func mysqlPortListening(host string, port int) bool {
	if port <= 0 {
		return false
	}
	candidates := backupMySQLHostCandidates(host)
	if len(candidates) == 0 {
		candidates = []string{"127.0.0.1"}
	}
	for _, candidate := range candidates {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(candidate, fmt.Sprintf("%d", port)), 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}

func isLogicalDumpBackupPath(path string) bool {
	normalized := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(normalized, ".sql") || strings.HasSuffix(normalized, ".sql.gz")
}

func (e *TaskExecutor) tryInstallMysqlClient(ctx context.Context) error {
	packages := [][]string{
		{"apt-get", "install", "-y", "-qq", "mysql-client"},
		{"apt-get", "install", "-y", "-qq", "mysql-client-core-8.0"},
		{"yum", "install", "-y", "-q", "mysql"},
		{"yum", "install", "-y", "-q", "mysql-community-client"},
		{"dnf", "install", "-y", "-q", "mysql"},
	}
	for _, args := range packages {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err == nil {
			if _, lookErr := exec.LookPath("mysqlbinlog"); lookErr == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("failed to install mysql-client package automatically")
}

func (e *TaskExecutor) executeGTIDIncrementalBackup(ctx context.Context, config BackupConfig) (*TaskResult, error) {
	if strings.TrimSpace(config.BaseBackupPath) == "" {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   "incremental backup requires a completed full backup",
			Timestamp: time.Now(),
		}, nil
	}
	if !isLogicalDumpBackupPath(config.BaseBackupPath) {
		return e.executeXtrabackup(ctx, config, false)
	}
	if _, err := exec.LookPath("mysqlbinlog"); err != nil {
		if installErr := e.tryInstallMysqlClient(ctx); installErr != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  0,
				Message:   fmt.Sprintf("mysqlbinlog is required for incremental backup but was not found on target host, auto-install failed: %v", installErr),
				Timestamp: time.Now(),
			}, nil
		}
		if _, err := exec.LookPath("mysqlbinlog"); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  0,
				Message:   "mysqlbinlog is required for incremental backup but could not be installed on target host",
				Timestamp: time.Now(),
			}, nil
		}
	}
	baseGTID, err := parseGTIDFromDump(config.BaseBackupPath)
	gtidWarning := ""
	if err != nil {
		if strings.Contains(err.Error(), "GTID_PURGED not found") {
			gtidWarning = "base mysqldump has no GTID_PURGED; mysqlbinlog will not exclude transactions already included in the base dump"
		} else {
			return &TaskResult{
				Status:    "failed",
				Progress:  0,
				Message:   fmt.Sprintf("parse GTID from base mysqldump failed: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
	}
	binlogs, err := fetchBinaryLogFiles(ctx, config)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("fetch binary log list failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	if len(binlogs) == 0 {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   "no binary logs found for GTID incremental backup",
			Timestamp: time.Now(),
		}, nil
	}
	if err := os.MkdirAll(config.TargetDir, 0o755); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("create backup root failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	backupFile := fmt.Sprintf("%s/%s-%d.sql", config.TargetDir, config.BackupType, time.Now().UnixNano())
	var lastErr string
	for _, host := range backupMySQLHostCandidates(config.MySQLHost) {
		_ = os.Remove(backupFile)
		args := []string{
			"--read-from-remote-server",
			"--host=" + host,
			"--port=" + fmt.Sprintf("%d", config.MySQLPort),
			"--user=" + defaultString(config.MySQLUser, "root"),
			"--password=" + config.MySQLPass,
			"--result-file=" + backupFile,
		}
		if strings.TrimSpace(baseGTID) != "" {
			args = append(args, "--exclude-gtids="+baseGTID)
		}
		args = append(args, binlogs...)
		cmd := exec.CommandContext(ctx, "mysqlbinlog", args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			lastErr = fmt.Sprintf("host=%s: %v\n%s", host, err, strings.TrimSpace(string(out)))
			continue
		}
		lastErr = ""
		break
	}
	if lastErr != "" {
		_ = os.Remove(backupFile)
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   "mysqlbinlog incremental backup failed: " + lastErr,
			Timestamp: time.Now(),
		}, nil
	}
	sizeBytes, err := pathSize(backupFile)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("calculate backup size failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	checksum, err := checksumPath(backupFile)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("calculate backup checksum failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	message := fmt.Sprintf("Incremental backup completed successfully. Path: %s, Checksum: %s", backupFile, checksum)
	if gtidWarning != "" {
		message += ". " + gtidWarning
	}
	data := map[string]any{
		"backup_path":       backupFile,
		"backup_type":       config.BackupType,
		"backup_method":     "mysqlbinlog",
		"base_backup_path":  config.BaseBackupPath,
		"base_backup_label": config.BaseBackupLabel,
		"base_backup_gtid":  baseGTID,
		"file_size":         sizeBytes,
		"checksum":          checksum,
	}
	if gtidWarning != "" {
		data["gtid_warning"] = gtidWarning
	}
	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   message,
		Timestamp: time.Now(),
		Data:      data,
	}, nil
}

func fetchBinaryLogFiles(ctx context.Context, config BackupConfig) ([]string, error) {
	var lastErr error
	for _, host := range backupMySQLHostCandidates(config.MySQLHost) {
		cmd := exec.CommandContext(ctx, "mysql", "-N", "-B",
			"-h", host,
			"-P", fmt.Sprintf("%d", config.MySQLPort),
			"-u", defaultString(config.MySQLUser, "root"),
			"-e", "SHOW BINARY LOGS;",
		)
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+config.MySQLPass)
		out, err := cmd.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("host=%s: %v %s", host, err, strings.TrimSpace(string(out)))
			continue
		}
		logs := make([]string, 0)
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 0 && strings.TrimSpace(fields[0]) != "" {
				logs = append(logs, fields[0])
			}
		}
		return logs, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no MySQL host candidates available")
}

func (e *TaskExecutor) executeColdDatadirBackup(ctx context.Context, config BackupConfig) (*TaskResult, error) {
	datadir := filepath.Clean(strings.TrimSpace(config.DataDir))
	if err := validateDecommissionDatadir(datadir); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   "cold datadir backup refused: " + err.Error(),
			Timestamp: time.Now(),
		}, nil
	}
	if mysqlPortListening(config.MySQLHost, config.MySQLPort) {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   "cold datadir backup refused: MySQL port is still listening",
			Timestamp: time.Now(),
		}, nil
	}
	if stat, err := os.Stat(datadir); err != nil || !stat.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a directory")
		}
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("cold datadir backup source invalid: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	if err := os.MkdirAll(config.TargetDir, 0o755); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("create backup root failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	backupFile := fmt.Sprintf("%s/%s-cold-%d.tar.gz", config.TargetDir, config.BackupType, time.Now().UnixNano())
	if err := createTarGz(datadir, backupFile); err != nil {
		_ = os.Remove(backupFile)
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("cold datadir backup failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	sizeBytes, err := pathSize(backupFile)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("calculate backup size failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	checksum, err := checksumPath(backupFile)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("calculate backup checksum failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Backup completed successfully. Path: %s, Checksum: %s. MySQL was offline, used cold datadir backup", backupFile, checksum),
		Timestamp: time.Now(),
		Data: map[string]any{
			"backup_path":   backupFile,
			"backup_type":   config.BackupType,
			"backup_method": "cold_datadir",
			"file_size":     sizeBytes,
			"checksum":      checksum,
		},
	}, nil
}

func createTarGz(srcDir, destFile string) error {
	out, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	base := filepath.Dir(srcDir)
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(tw, in)
		return err
	})
}

func (e *TaskExecutor) executeLogicalBackup(ctx context.Context, config BackupConfig, reason string) (*TaskResult, error) {
	if err := os.MkdirAll(config.TargetDir, 0o755); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("create backup root failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	backupFile := fmt.Sprintf("%s/%s-%d.sql", config.TargetDir, config.BackupType, time.Now().UnixNano())
	outFile, err := os.Create(backupFile)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("create logical backup file failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	var lastDumpErr string
	for _, host := range backupMySQLHostCandidates(config.MySQLHost) {
		if err := outFile.Truncate(0); err != nil {
			_ = outFile.Close()
			_ = os.Remove(backupFile)
			return &TaskResult{
				Status:    "failed",
				Progress:  0,
				Message:   fmt.Sprintf("truncate logical backup file failed: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
		if _, err := outFile.Seek(0, 0); err != nil {
			_ = outFile.Close()
			_ = os.Remove(backupFile)
			return &TaskResult{
				Status:    "failed",
				Progress:  0,
				Message:   fmt.Sprintf("seek logical backup file failed: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
		args := []string{
			"--single-transaction",
			"--routines",
			"--events",
			"--triggers",
			"--all-databases",
			"--set-gtid-purged=ON",
			"-h", host,
			"-P", fmt.Sprintf("%d", config.MySQLPort),
			"-u", defaultString(config.MySQLUser, "root"),
		}
		stderr, err := runMysqldumpToFile(ctx, outFile, config.MySQLPass, args)
		if err != nil && strings.Contains(stderr.String(), "set-gtid-purged") {
			if resetErr := resetBackupOutputFile(outFile); resetErr != nil {
				_ = outFile.Close()
				_ = os.Remove(backupFile)
				return &TaskResult{
					Status:    "failed",
					Progress:  0,
					Message:   fmt.Sprintf("reset logical backup file failed: %v", resetErr),
					Timestamp: time.Now(),
				}, nil
			}
			retryArgs := make([]string, 0, len(args)-1)
			for _, arg := range args {
				if arg != "--set-gtid-purged=ON" {
					retryArgs = append(retryArgs, arg)
				}
			}
			stderr, err = runMysqldumpToFile(ctx, outFile, config.MySQLPass, retryArgs)
		}
		if err != nil {
			lastDumpErr = fmt.Sprintf("host=%s: %v\n%s", host, err, strings.TrimSpace(stderr.String()))
			continue
		}
		lastDumpErr = ""
		break
	}
	if lastDumpErr != "" {
		_ = outFile.Close()
		_ = os.Remove(backupFile)
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   "mysqldump backup failed: " + lastDumpErr,
			Timestamp: time.Now(),
		}, nil
	}
	if err := outFile.Close(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("close logical backup file failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	gtid, gtidErr := parseGTIDFromDump(backupFile)
	sizeBytes, err := pathSize(backupFile)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("calculate backup size failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	checksum, err := checksumPath(backupFile)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("calculate backup checksum failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	msg := fmt.Sprintf("Backup completed successfully. Path: %s, Checksum: %s", backupFile, checksum)
	if reason != "" {
		msg += ". " + reason
	}
	if config.BackupType == "full" && gtidErr != nil {
		msg += ". GTID was not found in mysqldump; GTID incremental backup will not be available from this full backup"
	}
	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   msg,
		Timestamp: time.Now(),
		Data: map[string]any{
			"backup_path":   backupFile,
			"backup_type":   config.BackupType,
			"backup_method": "mysqldump",
			"gtid_purged":   gtid,
			"gtid_executed": gtid,
			"file_size":     sizeBytes,
			"checksum":      checksum,
		},
	}, nil
}

func resetBackupOutputFile(file *os.File) error {
	if err := file.Truncate(0); err != nil {
		return err
	}
	_, err := file.Seek(0, 0)
	return err
}

func runMysqldumpToFile(ctx context.Context, outFile *os.File, password string, args []string) (bytes.Buffer, error) {
	cmd := exec.CommandContext(ctx, "mysqldump", args...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+password)
	cmd.Stdout = outFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	return stderr, cmd.Run()
}

func parseGTIDFromDump(path string) (string, error) {
	reader, closeFn, err := openDumpReader(path)
	if err != nil {
		return "", err
	}
	defer closeFn()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	re := regexp.MustCompile(`(?i)GTID_PURGED\s*=\s*(?:/\*![0-9]+\s*'\+'\*/\s*)?'([^']+)'`)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(strings.ToUpper(line), "GTID_PURGED") {
			continue
		}
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 && strings.TrimSpace(matches[1]) != "" {
			return strings.TrimSpace(matches[1]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("GTID_PURGED not found in dump")
}

func openDumpReader(path string) (io.Reader, func(), error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, func() {}, err
	}
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			_ = file.Close()
			return nil, func() {}, err
		}
		return gzReader, func() {
			_ = gzReader.Close()
			_ = file.Close()
		}, nil
	}
	return file, func() { _ = file.Close() }, nil
}

func (e *TaskExecutor) ExecuteRestore(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseRestoreConfig(req.Config)

	return e.executeRestore(ctx, config)
}

func parseRestoreConfig(config map[string]interface{}) RestoreConfig {
	rc := RestoreConfig{}

	if v, ok := config["backup_path"].(string); ok {
		rc.BackupPath = v
	}
	if v, ok := config["backup_type"].(string); ok {
		rc.BackupType = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := config["datadir"].(string); ok {
		rc.DataDir = strings.TrimSpace(v)
	}
	if v, ok := config["mysql_host"].(string); ok {
		rc.MySQLHost = v
	}
	if v, ok := config["mysql_port"].(int); ok {
		rc.MySQLPort = v
	}
	if v, ok := config["mysql_user"].(string); ok {
		rc.MySQLUser = v
	}
	if v, ok := config["mysql_pass"].(string); ok {
		rc.MySQLPass = v
	}

	return rc
}

func detectRestoreBackupType(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		checkpoints := filepath.Join(path, "xtrabackup_checkpoints")
		if _, err := os.Stat(checkpoints); err == nil {
			return "xtrabackup"
		}
		return ""
	}
	if strings.HasSuffix(strings.ToLower(path), ".sql") {
		return "mysqldump"
	}
	return ""
}

func (e *TaskExecutor) executeRestore(ctx context.Context, config RestoreConfig) (*TaskResult, error) {
	backupType := config.BackupType
	if backupType == "" {
		backupType = detectRestoreBackupType(config.BackupPath)
	}

	switch backupType {
	case "xtrabackup":
		return e.executeXtrabackupRestore(ctx, config)
	case "mysqldump", "mysqlbinlog":
		return e.executeLogicalRestore(ctx, config)
	default:
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Unsupported or undetectable backup type for path %s (specify backup_type)", config.BackupPath),
			Timestamp: time.Now(),
		}, nil
	}
}

func (e *TaskExecutor) executeXtrabackupRestore(ctx context.Context, config RestoreConfig) (*TaskResult, error) {
	datadir := config.DataDir
	if datadir == "" {
		datadir = "/var/lib/mysql"
	}

	prepareCmd := exec.CommandContext(ctx, "xtrabackup", "--prepare", "--target-dir="+config.BackupPath)
	if out, err := prepareCmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  20,
			Message:   fmt.Sprintf("Xtrabackup prepare failed: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}, nil
	}

	copyCmd := exec.CommandContext(ctx, "xtrabackup", "--copy-back", "--target-dir="+config.BackupPath, "--datadir="+datadir)
	if out, err := copyCmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("Xtrabackup copy-back failed: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}, nil
	}

	chownCmd := exec.CommandContext(ctx, "chown", "-R", "mysql:mysql", datadir)
	if err := chownCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "completed",
			Progress:  90,
			Message:   fmt.Sprintf("Restore completed but chown failed: %v (datadir=%s)", err, datadir),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Xtrabackup restore completed to %s from %s", datadir, config.BackupPath),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) executeLogicalRestore(ctx context.Context, config RestoreConfig) (*TaskResult, error) {
	host := config.MySQLHost
	if host == "" {
		host = "127.0.0.1"
	}
	port := config.MySQLPort
	if port == 0 {
		port = 3306
	}
	user := config.MySQLUser
	if user == "" {
		user = "root"
	}

	mysqlBin := getMySQLBinary()
	restoreCmd := exec.CommandContext(ctx, mysqlBin, "-h", host, "-P", fmt.Sprintf("%d", port), "-u", user)
	if config.MySQLPass != "" {
		restoreCmd.Env = append(os.Environ(), "MYSQL_PWD="+config.MySQLPass)
	}

	sqlFile, err := os.Open(config.BackupPath)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  10,
			Message:   fmt.Sprintf("Failed to open backup file %s: %v", config.BackupPath, err),
			Timestamp: time.Now(),
		}, nil
	}
	defer sqlFile.Close()

	restoreCmd.Stdin = sqlFile

	out, err := restoreCmd.CombinedOutput()
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Logical restore failed: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Logical restore completed from %s", config.BackupPath),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) ExecuteHealthCheck(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	data := collectSystemInfo(ctx)
	instanceID := req.InstanceID
	if instanceID == "" {
		return &TaskResult{
			TaskID:    "health-host",
			Status:    "healthy",
			Progress:  100,
			Message:   "Host information collected",
			Timestamp: time.Now(),
			Data:      data,
		}, nil
	}
	host, _ := req.Config["target_host"].(string)
	if host == "" {
		host = "127.0.0.1"
	}
	port := configInt(req.Config, "target_port")
	if port == 0 {
		port = 3306
	}
	user, _ := req.Config["target_user"].(string)
	if user == "" {
		user = "root"
	}
	pass, _ := req.Config["target_pass"].(string)

	var mysqlAdminArgs []string
	mysqlAdminArgs = append(mysqlAdminArgs, "mysqladmin", "ping",
		"-h", host,
		"-P", fmt.Sprintf("%d", port),
		"-u", user)
	mysqlVersion := configString(req.Config, "mysql_version")
	if mysqlHasGetServerPublicKey(mysqlVersion) {
		mysqlAdminArgs = append(mysqlAdminArgs, "--get-server-public-key")
	}
	cmd := exec.CommandContext(ctx, mysqlAdminArgs[0], mysqlAdminArgs[1:]...)
	if pass != "" {
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+pass)
	}

	if err := cmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    fmt.Sprintf("health-%s", instanceID),
			Status:    "unhealthy",
			Progress:  100,
			Message:   fmt.Sprintf("Health check failed: %v", err),
			Timestamp: time.Now(),
			Data:      data,
		}, nil
	}

	return &TaskResult{
		TaskID:    fmt.Sprintf("health-%s", instanceID),
		Status:    "healthy",
		Progress:  100,
		Message:   "Instance is healthy",
		Timestamp: time.Now(),
		Data:      data,
	}, nil
}

func collectSystemInfo(ctx context.Context) map[string]any {
	data := map[string]any{}
	if out := commandOutput(ctx, "sh", "-c", ". /etc/os-release 2>/dev/null; echo ${PRETTY_NAME:-unknown}"); out != "" {
		data["os_release"] = out
	}
	if out := commandOutput(ctx, "uname", "-r"); out != "" {
		data["kernel_version"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n vm.swappiness 2>/dev/null"); out != "" {
		data["vm_swappiness"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n vm.max_map_count 2>/dev/null"); out != "" {
		data["vm_max_map_count"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n vm.overcommit_memory 2>/dev/null"); out != "" {
		data["vm_overcommit_memory"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n fs.file-max 2>/dev/null"); out != "" {
		data["fs_file_max"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n fs.aio-max-nr 2>/dev/null"); out != "" {
		data["fs_aio_max_nr"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "ulimit -n 2>/dev/null"); out != "" {
		data["ulimit_nofile"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n net.core.somaxconn 2>/dev/null"); out != "" {
		data["net_core_somaxconn"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n net.core.netdev_max_backlog 2>/dev/null"); out != "" {
		data["net_core_netdev_max_backlog"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n net.ipv4.tcp_max_syn_backlog 2>/dev/null"); out != "" {
		data["net_ipv4_tcp_max_syn_backlog"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n net.ipv4.tcp_fin_timeout 2>/dev/null"); out != "" {
		data["net_ipv4_tcp_fin_timeout"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n net.ipv4.tcp_keepalive_time 2>/dev/null"); out != "" {
		data["net_ipv4_tcp_keepalive_time"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n net.ipv4.tcp_tw_reuse 2>/dev/null"); out != "" {
		data["net_ipv4_tcp_tw_reuse"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "sysctl -n net.ipv4.ip_local_port_range 2>/dev/null"); out != "" {
		data["net_ipv4_ip_local_port_range"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "cat /sys/kernel/mm/transparent_hugepage/enabled 2>/dev/null"); out != "" {
		data["transparent_hugepage"] = out
	}
	if out := commandOutput(ctx, "nproc"); out != "" {
		data["cpu_cores"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "awk '/MemTotal/ {print int($2/1024) \" MB\"}' /proc/meminfo"); out != "" {
		data["memory_size"] = out
	}
	if out := commandOutput(ctx, "sh", "-c", "df -h / | awk 'NR==2 {print $4 \" available / \" $2 \" total\"}'"); out != "" {
		data["disk_space"] = out
	}
	if _, err := os.Stat("/usr/lib64/libaio.so.1"); err == nil {
		data["libaio"] = "installed"
	} else if out := commandOutput(ctx, "sh", "-c", "ldconfig -p 2>/dev/null | grep -m1 libaio || rpm -q libaio 2>/dev/null || dpkg -s libaio1 2>/dev/null | grep -m1 '^Status:'"); out != "" {
		data["libaio"] = "installed"
	} else {
		data["libaio"] = "not_found"
	}
	return data
}

func commandOutput(ctx context.Context, name string, args ...string) string {
	cmdCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, name, args...).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ExecuteVersionDetect detects the MySQL version on the target instance.
func (e *TaskExecutor) ExecuteVersionDetect(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	host, _ := req.Config["target_host"].(string)
	if host == "" {
		host = "127.0.0.1"
	}
	port := configInt(req.Config, "target_port")
	if port == 0 {
		port = 3306
	}
	user, _ := req.Config["target_user"].(string)
	if user == "" {
		user = "root"
	}
	pass, _ := req.Config["target_pass"].(string)

	cmd := exec.CommandContext(ctx, "mysql",
		"-h", host,
		"-P", fmt.Sprintf("%d", port),
		"-u", user,
		"-N", "-B", "-e", "SELECT @@version, @@version_comment")
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+pass)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &TaskResult{
			TaskID:    fmt.Sprintf("version-detect-%s", req.InstanceID),
			Status:    "failed",
			Progress:  100,
			Message:   fmt.Sprintf("version detect failed: %v, output: %s", err, string(output)),
			Timestamp: time.Now(),
		}, nil
	}
	// 输出形如: 8.0.36\tuuid-uuid\tMySQL Community Server - GPL
	return &TaskResult{
		TaskID:    fmt.Sprintf("version-detect-%s", req.InstanceID),
		Status:    "completed",
		Progress:  100,
		Message:   string(output),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) ExecuteMigration(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	migrationType, _ := req.Config["migration_type"].(string)
	migrationExecutor := NewMigrationExecutor()

	switch migrationType {
	case "physical":
		return migrationExecutor.ExecutePhysicalMigration(ctx, req)
	case "replication":
		return migrationExecutor.ExecuteReplicationMigration(ctx, req)
	case "gtid":
		return migrationExecutor.ExecuteGTIDMigration(ctx, req)
	default:
		return migrationExecutor.ExecutePhysicalMigration(ctx, req)
	}
}

func (e *TaskExecutor) ExecuteMigrationSwitch(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	migrationExecutor := NewMigrationExecutor()

	result, err := migrationExecutor.ExecuteSwitch(ctx, req)
	if err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Migration switch failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	status := result.Status
	progress := 100
	if status == "pending" || status == "lock_failed" {
		status = "failed"
		progress = 0
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    status,
		Progress:  progress,
		Message:   fmt.Sprintf("Migration switch %s. Old source: %s, New target: %s", status, result.OldSource, result.NewTarget),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) MonitorMigration(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	migrationExecutor := NewMigrationExecutor()

	progress := migrationExecutor.MonitorProgress(ctx, req)

	return &TaskResult{
		TaskID:   req.TaskID,
		Status:   progress.Status,
		Progress: int(progress.Percentage),
		Message: fmt.Sprintf("Migration phase: %s, Completed: %.2f%%, Lag: %ds, Throughput: %.2f rows/s",
			progress.Phase, progress.Percentage, progress.ReplicationLag, progress.Throughput),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) ExecuteUpgrade(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	upgradeExecutor := NewUpgradeExecutor()

	upgradeType, _ := req.Config["upgrade_type"].(string)

	switch upgradeType {
	case "in-place":
		return upgradeExecutor.ExecuteInPlaceUpgrade(ctx, req)
	case "logical":
		return upgradeExecutor.ExecuteLogicalMigration(ctx, req)
	case "rolling":
		return upgradeExecutor.ExecuteRollingUpgrade(ctx, req)
	default:
		return upgradeExecutor.ExecuteInPlaceUpgrade(ctx, req)
	}
}

func (e *TaskExecutor) PlanUpgrade(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	upgradeExecutor := NewUpgradeExecutor()
	path, err := upgradeExecutor.PlanUpgradePath(ctx, req)
	if err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Failed to plan upgrade path: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:   req.TaskID,
		Status:   "completed",
		Progress: 100,
		Message: fmt.Sprintf("Upgrade path planned. Steps: %d, Estimated time: %d min, Compatible: %v",
			len(path.Steps), path.EstimatedTime, path.CompatibilityOK),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) CheckUpgradeCompatibility(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	upgradeExecutor := NewUpgradeExecutor()
	report, err := upgradeExecutor.CheckCompatibility(ctx, req)
	if err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Failed to check compatibility: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	status := "compatible"
	if !report.Compatible {
		status = "incompatible"
	}

	return &TaskResult{
		TaskID:   req.TaskID,
		Status:   status,
		Progress: 100,
		Message: fmt.Sprintf("Compatibility: %v, Gap: %s, Warnings: %d, Errors: %d",
			report.Compatible, report.VersionGap, len(report.Warnings), len(report.Errors)),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) RollbackUpgrade(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	upgradeExecutor := NewUpgradeExecutor()
	return upgradeExecutor.RollbackUpgrade(ctx, req)
}

type RoleSwitchConfig struct {
	ClusterID       string `json:"cluster_id"`
	InstanceID      string `json:"instance_id"`
	ClusterType     string `json:"cluster_type"`
	TargetRole      string `json:"target_role"`
	OldMasterID     string `json:"old_master_id"`
	NewMasterID     string `json:"new_master_id"`
	ReplicationUser string `json:"replication_user"`
	ReplicationPass string `json:"replication_pass"`
	Force           bool   `json:"force"`
	GracePeriodSec  int    `json:"grace_period_sec"`
	TargetHost string `json:"target_host"`
	TargetPort int    `json:"target_port"`
	TargetUser string `json:"target_user"`
	TargetPass string `json:"target_pass"`
	// MGR promote 真正需要的: 新主 server_uuid, 不是 THISUUID.
	NewMasterServerUUID string `json:"new_master_server_uuid"`
	// Replica-rebuild 需要新主 host:port:user:pass, 之前用 NewMasterID (UUID) 拼 MASTER_HOST.
	NewMasterHost string `json:"new_master_host"`
	NewMasterPort int    `json:"new_master_port"`
	NewMasterUser string `json:"new_master_user"`
	NewMasterPass string `json:"new_master_pass"`
}

type RoleQueryResult struct {
	ClusterID    string `json:"cluster_id"`
	InstanceID   string `json:"instance_id"`
	Role         string `json:"role"`
	ServerID     int    `json:"server_id"`
	ReadOnly     bool   `json:"read_only"`
	MasterHost   string `json:"master_host"`
	MasterPort   int    `json:"master_port"`
	SlaveRunning bool   `json:"slave_running"`
	GTIDExecuted string `json:"gtid_executed"`
}

func parseRoleSwitchConfig(config map[string]interface{}) RoleSwitchConfig {
	c := RoleSwitchConfig{
		ClusterType: "mha",
		Force:       false,
	}
	if v, ok := config["cluster_id"].(string); ok {
		c.ClusterID = v
	}
	if v, ok := config["instance_id"].(string); ok {
		c.InstanceID = v
	}
	if v, ok := config["cluster_type"].(string); ok {
		c.ClusterType = v
	}
	if v, ok := config["target_role"].(string); ok {
		c.TargetRole = v
	}
	if v, ok := config["old_master_id"].(string); ok {
		c.OldMasterID = v
	}
	if v, ok := config["new_master_id"].(string); ok {
		c.NewMasterID = v
	}
	if v, ok := config["replication_user"].(string); ok {
		c.ReplicationUser = v
	}
	if v, ok := config["replication_pass"].(string); ok {
		c.ReplicationPass = v
	}
	if v, ok := config["force"].(bool); ok {
		c.Force = v
	}
	if v, ok := config["grace_period_sec"].(int); ok {
		c.GracePeriodSec = v
	}
	if v, ok := config["target_host"].(string); ok {
		c.TargetHost = v
	}
	if v, ok := config["target_port"].(int); ok {
		c.TargetPort = v
	} else if v, ok := config["target_port"].(float64); ok {
		c.TargetPort = int(v)
	}
	if v, ok := config["target_user"].(string); ok {
		c.TargetUser = v
	}
	if c.TargetUser == "" {
		if v, ok := config["mysql_user"].(string); ok {
			c.TargetUser = v
		}
	}
	if v, ok := config["target_pass"].(string); ok {
		c.TargetPass = v
	}
	if c.TargetPass == "" {
		if v, ok := config["mysql_password"].(string); ok {
			c.TargetPass = v
		}
	}
	if c.TargetPass == "" {
		if v, ok := config["mysql_pass"].(string); ok {
			c.TargetPass = v
		}
	}
	if v, ok := config["new_master_server_uuid"].(string); ok {
		c.NewMasterServerUUID = v
	}
	if v, ok := config["new_master_host"].(string); ok {
		c.NewMasterHost = v
	}
	if v, ok := config["new_master_port"].(int); ok {
		c.NewMasterPort = v
	} else if v, ok := config["new_master_port"].(float64); ok {
		c.NewMasterPort = int(v)
	}
	if v, ok := config["new_master_user"].(string); ok {
		c.NewMasterUser = v
	}
	if v, ok := config["new_master_pass"].(string); ok {
		c.NewMasterPass = v
	}
	return c
}

func mysqlBaseArgs(host string, port int, user, pass string) []string {
	return []string{
		"-h", host,
		"-P", fmt.Sprintf("%d", port),
		"-u", user,
		fmt.Sprintf("-p%s", pass),
	}
}

// resolveTargetConn prefers target connection values from cfg.
// Missing values fall back to 127.0.0.1:3306 root with an empty password.
func resolveTargetConn(cfg RoleSwitchConfig) (string, int, string, string) {
	if cfg.TargetHost != "" {
		return cfg.TargetHost, cfg.TargetPort, cfg.TargetUser, cfg.TargetPass
	}
	return "127.0.0.1", 3306, "root", ""
}

func resolveMGRPrimaryUUID(ctx context.Context, host string, port int, user, pass string, cfg RoleSwitchConfig) (string, error) {
	if strings.TrimSpace(cfg.NewMasterServerUUID) != "" {
		return strings.TrimSpace(cfg.NewMasterServerUUID), nil
	}
	uuid, err := runMySQLExec(ctx, host, port, user, pass, "SELECT @@server_uuid")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(uuid), nil
}

func runMySQLExec(ctx context.Context, host string, port int, user, pass, sql string) (string, error) {
	args := append(mysqlBaseArgs(host, port, user, pass), "-N", "-B", "-e", sql)
	cmd := exec.CommandContext(ctx, "mysql", args...)
	out, err := cmd.CombinedOutput()
	return cleanMySQLOutput(string(out)), err
}

func cleanMySQLOutput(output string) string {
	lines := strings.Split(output, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "mysql: [Warning]") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func (e *TaskExecutor) ExecuteRoleQuery(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	cfg := parseRoleSwitchConfig(req.Config)

	if cfg.GracePeriodSec > 0 {
		time.Sleep(time.Duration(cfg.GracePeriodSec) * time.Second)
	}

	readOnlySQL := "SELECT @@read_only, @@server_id, @@server_uuid"
	host, port, user, pass := resolveTargetConn(cfg)
	output, err := runMySQLExec(ctx, host, port, user, pass, readOnlySQL)
	if err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("failed to query instance state: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	fields := strings.Fields(strings.TrimSpace(output))
	readOnly := len(fields) > 0 && (fields[0] == "1" || strings.EqualFold(fields[0], "ON"))
	serverID := 0
	if len(fields) > 1 {
		serverID, _ = strconv.Atoi(fields[1])
	}

	slaveBytes, _ := runFirstMySQL(ctx, host, port, user, pass, "SHOW SLAVE STATUS\\G", "SHOW REPLICA STATUS\\G")
	slaveOutput := cleanMySQLOutput(string(slaveBytes))
	slaveRunning := replicationThreadsRunning(slaveOutput)
	role := "primary"
	if slaveRunning || readOnly {
		role = "replica"
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("role=%s read_only_state_checked slave_running=%t", role, slaveRunning),
		Timestamp: time.Now(),
		Data: RoleQueryResult{
			ClusterID:    cfg.ClusterID,
			InstanceID:   cfg.InstanceID,
			Role:         role,
			ServerID:     serverID,
			ReadOnly:     readOnly,
			SlaveRunning: slaveRunning,
		},
	}, nil
}

func replicationThreadsRunning(status string) bool {
	return (strings.Contains(status, "Slave_IO_Running: Yes") || strings.Contains(status, "Replica_IO_Running: Yes")) &&
		(strings.Contains(status, "Slave_SQL_Running: Yes") || strings.Contains(status, "Replica_SQL_Running: Yes"))
}

func (e *TaskExecutor) ExecuteRolePromote(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	cfg := parseRoleSwitchConfig(req.Config)

	if cfg.GracePeriodSec > 0 {
		time.Sleep(time.Duration(cfg.GracePeriodSec) * time.Second)
	}

	host, port, user, pass := resolveTargetConn(cfg)

	if cfg.ClusterType == "mgr" {
		uuid, err := resolveMGRPrimaryUUID(ctx, host, port, user, pass, cfg)
		if err != nil || uuid == "" {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  80,
				Message:   fmt.Sprintf("MGR promote requires target server_uuid: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
		mgrSQL := fmt.Sprintf("SELECT group_replication_set_as_primary('%s')", uuid)
		if _, err := runMySQLExec(ctx, host, port, user, pass, mgrSQL); err != nil {
			if !cfg.Force {
				return &TaskResult{
					TaskID:    req.TaskID,
					Status:    "failed",
					Progress:  60,
					Message:   fmt.Sprintf("MGR set_as_primary failed: %v (use force=true to skip)", err),
					Timestamp: time.Now(),
				}, nil
			}
		}
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "completed",
			Progress:  100,
			Message:   fmt.Sprintf("MGR instance %s promoted to %s with server_uuid %s", cfg.InstanceID, cfg.TargetRole, uuid),
			Timestamp: time.Now(),
		}, nil
	}

	if cfg.ClusterType == "pxc" {
		for _, sql := range []string{"SET GLOBAL read_only = OFF", "SET GLOBAL super_read_only = OFF"} {
			if _, err := runMySQLExec(ctx, host, port, user, pass, sql); err != nil && !cfg.Force {
				return &TaskResult{
					TaskID:    req.TaskID,
					Status:    "failed",
					Progress:  40,
					Message:   fmt.Sprintf("PXC promote step %q failed: %v", sql, err),
					Timestamp: time.Now(),
				}, nil
			}
		}
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "completed",
			Progress:  100,
			Message:   fmt.Sprintf("PXC instance %s promoted to %s by disabling read_only", cfg.InstanceID, cfg.TargetRole),
			Timestamp: time.Now(),
		}, nil
	}

	if out, err := runFirstMySQL(ctx, host, port, user, pass, "STOP SLAVE;", "STOP REPLICA;"); err != nil && !cfg.Force {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  30,
			Message:   fmt.Sprintf("promote step %q failed: %v, output: %s", "stop replication", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}, nil
	}
	for _, sql := range []string{"SET GLOBAL read_only = OFF", "SET GLOBAL super_read_only = OFF"} {
		if _, err := runMySQLExec(ctx, host, port, user, pass, sql); err != nil && !cfg.Force {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  40,
				Message:   fmt.Sprintf("promote step %q failed: %v", sql, err),
				Timestamp: time.Now(),
			}, nil
		}
	}
	if out, err := runFirstMySQL(ctx, host, port, user, pass, "RESET SLAVE ALL;", "RESET REPLICA ALL;"); err != nil && !cfg.Force {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("promote step %q failed: %v, output: %s", "reset replication", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("instance %s promoted to %s within %s cluster", cfg.InstanceID, cfg.TargetRole, cfg.ClusterType),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) ExecuteRoleDemote(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	cfg := parseRoleSwitchConfig(req.Config)

	host, port, user, pass := resolveTargetConn(cfg)

	steps := []struct {
		sql   string
		label string
	}{
		{"SET GLOBAL super_read_only = OFF", "allow setting read_only"},
		{"SET GLOBAL read_only = ON", "enable read_only"},
		{"SET GLOBAL super_read_only = ON", "enable super_read_only"},
	}

	for _, step := range steps {
		if _, err := runMySQLExec(ctx, host, port, user, pass, step.sql); err != nil {
			if cfg.Force {
				continue
			}
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  30,
				Message:   fmt.Sprintf("demote step %q failed: %v", step.label, err),
				Timestamp: time.Now(),
			}, nil
		}
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   roleDemoteMessage(cfg),
		Timestamp: time.Now(),
	}, nil
}

func roleDemoteMessage(cfg RoleSwitchConfig) string {
	if cfg.ClusterType == "pxc" {
		return fmt.Sprintf("PXC instance %s demoted to %s by enabling read_only", cfg.InstanceID, cfg.TargetRole)
	}
	return fmt.Sprintf("instance %s demoted to %s within %s cluster", cfg.InstanceID, cfg.TargetRole, cfg.ClusterType)
}

func (e *TaskExecutor) ExecuteRoleReplicaRebuild(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	cfg := parseRoleSwitchConfig(req.Config)
	if cfg.ClusterType == "pxc" {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "completed",
			Progress:  100,
			Message:   fmt.Sprintf("PXC instance %s does not require async replica rebuild", cfg.InstanceID),
			Timestamp: time.Now(),
		}, nil
	}
	if cfg.NewMasterID == "" {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   "new_master_id is required to rebuild replica",
			Timestamp: time.Now(),
		}, nil
	}
	if cfg.NewMasterHost == "" {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  10,
			Message:   "new_master_host is required to rebuild replica (cannot use UUID as MASTER_HOST)",
			Timestamp: time.Now(),
		}, nil
	}

	host, port, user, pass := resolveTargetConn(cfg)

	if out, err := runFirstMySQL(ctx, host, port, user, pass, "STOP SLAVE; RESET SLAVE ALL;", "STOP REPLICA; RESET REPLICA ALL;"); err != nil {
		resetOut := strings.TrimSpace(string(out))
		if !strings.Contains(resetOut, "Slave is not configured") &&
			!strings.Contains(resetOut, "Replica is not configured") &&
			!strings.Contains(resetOut, "This server is not configured as slave") {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  30,
				Message:   fmt.Sprintf("replica reset failed: %v, output: %s", err, resetOut),
				Timestamp: time.Now(),
			}, nil
		}
	}
	changeMasterSQL := fmt.Sprintf(
		"CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_AUTO_POSITION=1; START SLAVE;",
		cfg.NewMasterHost, cfg.NewMasterPort, cfg.NewMasterUser, cfg.NewMasterPass,
	)
	changeReplicaSQL := fmt.Sprintf(
		"CHANGE REPLICATION SOURCE TO SOURCE_HOST='%s', SOURCE_PORT=%d, SOURCE_USER='%s', SOURCE_PASSWORD='%s', SOURCE_AUTO_POSITION=1; START REPLICA;",
		cfg.NewMasterHost, cfg.NewMasterPort, cfg.NewMasterUser, cfg.NewMasterPass,
	)
	if out, err := runFirstMySQL(ctx, host, port, user, pass, changeMasterSQL, changeReplicaSQL); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("replica rebuild failed: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}, nil
	}

	message := fmt.Sprintf("replica %s re-pointed to new master %s (%s:%d)", cfg.InstanceID, cfg.NewMasterID, cfg.NewMasterHost, cfg.NewMasterPort)
	if cfg.Force {
		repaired, repairErr := repairForcedReplicaGTID(ctx, host, port, user, pass)
		if repairErr != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  80,
				Message:   fmt.Sprintf("replica rebuild completed but forced GTID repair failed: %v", repairErr),
				Timestamp: time.Now(),
			}, nil
		}
		if repaired {
			message += "; skipped one failed GTID transaction during forced rebuild"
		}
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   message,
		Timestamp: time.Now(),
	}, nil
}

func repairForcedReplicaGTID(ctx context.Context, host string, port int, user, pass string) (bool, error) {
	time.Sleep(2 * time.Second)
	repaired := false
	const maxForcedGTIDSkips = 50
	for i := 0; i < maxForcedGTIDSkips; i++ {
		statusBytes, _ := runFirstMySQL(ctx, host, port, user, pass, "SHOW SLAVE STATUS\\G", "SHOW REPLICA STATUS\\G")
		status := cleanMySQLOutput(string(statusBytes))
		if replicationThreadsRunning(status) {
			return repaired, nil
		}
		gtid := failedReplicationGTID(status)
		if gtid == "" {
			return repaired, nil
		}
		sql := fmt.Sprintf(
			"STOP REPLICA SQL_THREAD; SET GTID_NEXT='%s'; BEGIN; COMMIT; SET GTID_NEXT='AUTOMATIC'; START REPLICA SQL_THREAD;",
			gtid,
		)
		if out, err := runFirstMySQL(ctx, host, port, user, pass,
			strings.ReplaceAll(sql, "REPLICA", "SLAVE"),
			sql,
		); err != nil {
			return repaired, fmt.Errorf("skip failed GTID %s: %v, output: %s", gtid, err, strings.TrimSpace(string(out)))
		}
		repaired = true
		time.Sleep(2 * time.Second)
	}
	return repaired, fmt.Errorf("replication still not running after skipping %d failed GTID transactions", maxForcedGTIDSkips)
}

func failedReplicationGTID(status string) string {
	re := regexp.MustCompile(`transaction '([0-9a-fA-F-]+:[0-9]+)'`)
	matches := re.FindStringSubmatch(status)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func (e *TaskExecutor) ExecuteBackupScan(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	dirs := []string{"/backup/mysql", "/backup", "/data/backup", "/var/lib/mysql-backup"}
	if raw, ok := req.Config["paths"].([]interface{}); ok && len(raw) > 0 {
		dirs = dirs[:0]
		for _, item := range raw {
			if s, ok := item.(string); ok && s != "" {
				dirs = append(dirs, s)
			}
		}
	}

	files := make([]BackupScanFile, 0)
	seen := map[string]bool{}
	seenBackups := map[string]bool{}
	for _, dir := range dirs {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		if isCompleteXtrabackupDir(dir) {
			sizeBytes, err := pathSize(dir)
			if err != nil {
				continue
			}
			files = appendBackupScanFile(files, seenBackups, BackupScanFile{
				FileName:   filepath.Base(dir),
				FilePath:   dir,
				SizeBytes:  sizeBytes,
				BackupType: classifyBackupPath(dir, true),
				IsDir:      true,
				Complete:   true,
				DetectedAt: time.Now(),
				MTime:      info.ModTime(),
			})
			if len(files) >= 500 {
				break
			}
			continue
		}
		root := dir
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if entry.IsDir() {
				if path == root {
					return nil
				}
				if !isCompleteXtrabackupDir(path) {
					return nil
				}
				sizeBytes, err := pathSize(path)
				if err != nil {
					return filepath.SkipDir
				}
				stat, err := entry.Info()
				if err != nil {
					return filepath.SkipDir
				}
				files = appendBackupScanFile(files, seenBackups, BackupScanFile{
					FileName:   entry.Name(),
					FilePath:   path,
					SizeBytes:  sizeBytes,
					BackupType: classifyBackupPath(path, true),
					IsDir:      true,
					Complete:   true,
					DetectedAt: time.Now(),
					MTime:      stat.ModTime(),
				})
				if len(files) >= 500 {
					return filepath.SkipAll
				}
				return filepath.SkipDir
			}
			name := entry.Name()
			backupType := classifyBackupPath(path, false)
			if backupType == "" {
				return nil
			}
			stat, err := entry.Info()
			if err != nil || stat.Size() <= 0 {
				return nil
			}
			files = appendBackupScanFile(files, seenBackups, BackupScanFile{
				FileName:   name,
				FilePath:   path,
				SizeBytes:  stat.Size(),
				BackupType: backupType,
				IsDir:      false,
				Complete:   true,
				DetectedAt: time.Now(),
				MTime:      stat.ModTime(),
			})
			if len(files) >= 500 {
				return filepath.SkipAll
			}
			return nil
		})
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("discovered %d backup files", len(files)),
		Timestamp: time.Now(),
		Data: map[string]any{
			"backups":    files,
			"scanned_at": time.Now(),
		},
	}, nil
}

func appendBackupScanFile(files []BackupScanFile, seen map[string]bool, item BackupScanFile) []BackupScanFile {
	key := filepath.Clean(item.FilePath)
	if seen[key] {
		return files
	}
	seen[key] = true
	return append(files, item)
}

func classifyBackupPath(path string, isDir bool) string {
	lower := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(lower, ".tmp") || strings.HasSuffix(lower, ".partial") || strings.HasSuffix(lower, ".incomplete") {
		return ""
	}
	if strings.Contains(lower, "incremental") || strings.Contains(lower, "-inc") || strings.Contains(lower, "_inc") {
		return "incremental"
	}
	if isDir {
		return "full"
	}
	switch {
	case strings.HasSuffix(lower, ".xbstream"):
		if strings.Contains(lower, "incremental") || strings.Contains(lower, "-inc") || strings.Contains(lower, "_inc") {
			return "incremental"
		}
		return "full"
	case strings.HasSuffix(lower, ".sql") || strings.HasSuffix(lower, ".sql.gz") || strings.HasSuffix(lower, ".dump"):
		return "logical"
	default:
		return ""
	}
}

func isCompleteXtrabackupDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	if !fileExists(filepath.Join(path, "xtrabackup_checkpoints")) || !fileExists(filepath.Join(path, "xtrabackup_info")) {
		return false
	}
	return fileExists(filepath.Join(path, "backup-my.cnf")) || fileExists(filepath.Join(path, "xtrabackup_logfile"))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func pathSize(path string) (int64, error) {
	var total int64
	err := filepath.WalkDir(path, func(p string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func checksumPath(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return checksumFile(path)
	}
	files := make([]string, 0)
	if err := filepath.WalkDir(path, func(p string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, p)
		return nil
	}); err != nil {
		return "", err
	}
	sortStrings(files)
	h := sha256.New()
	for _, file := range files {
		rel, _ := filepath.Rel(path, file)
		_, _ = io.WriteString(h, rel+"\n")
		sum, err := checksumFile(file)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(h, sum+"\n")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func checksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sortStrings(items []string) {
	for i := 1; i < len(items); i++ {
		value := items[i]
		j := i - 1
		for j >= 0 && items[j] > value {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = value
	}
}

func (e *TaskExecutor) ExecuteInstanceAdmin(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	action, _ := req.Config["action"].(string)
	host, port, user, pass := adminTarget(req.Config)

	var (
		output string
		err    error
	)
	switch action {
	case "list_users":
		output, err = runMySQLExecSafe(ctx, host, port, user, pass, "SELECT user, host, plugin, account_locked FROM mysql.user ORDER BY user, host")
	case "create_user":
		username, _ := req.Config["username"].(string)
		userHost, _ := req.Config["user_host"].(string)
		password, _ := req.Config["password"].(string)
		if userHost == "" {
			userHost = "%"
		}
		if !validSQLName(username) {
			return adminFailed(req.TaskID, "invalid username"), nil
		}
		output, err = runMySQLExecSafe(ctx, host, port, user, pass,
			fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s'", escapeSQL(username), escapeSQL(userHost), escapeSQL(password)))
	case "drop_user":
		username, _ := req.Config["username"].(string)
		userHost, _ := req.Config["user_host"].(string)
		if userHost == "" {
			userHost = "%"
		}
		if !validSQLName(username) {
			return adminFailed(req.TaskID, "invalid username"), nil
		}
		output, err = runMySQLExecSafe(ctx, host, port, user, pass,
			fmt.Sprintf("DROP USER IF EXISTS '%s'@'%s'", escapeSQL(username), escapeSQL(userHost)))
	case "change_password":
		username, _ := req.Config["username"].(string)
		userHost, _ := req.Config["user_host"].(string)
		password, _ := req.Config["password"].(string)
		if userHost == "" {
			userHost = "%"
		}
		if !validSQLName(username) {
			return adminFailed(req.TaskID, "invalid username"), nil
		}
		output, err = changeMySQLUserPassword(ctx, host, port, user, pass, username, userHost, password)
	case "grant_privileges":
		username, _ := req.Config["username"].(string)
		userHost, _ := req.Config["user_host"].(string)
		privileges, _ := req.Config["privileges"].(string)
		scope, _ := req.Config["scope"].(string)
		if userHost == "" {
			userHost = "%"
		}
		if scope == "" {
			scope = "*.*"
		}
		if !validSQLName(username) || !validPrivilegeList(privileges) || !validScope(scope) {
			return adminFailed(req.TaskID, "invalid grant input"), nil
		}
		output, err = runMySQLExecSafe(ctx, host, port, user, pass,
			fmt.Sprintf("GRANT %s ON %s TO '%s'@'%s'", privileges, scope, escapeSQL(username), escapeSQL(userHost)))
	case "revoke_privileges":
		username, _ := req.Config["username"].(string)
		userHost, _ := req.Config["user_host"].(string)
		privileges, _ := req.Config["privileges"].(string)
		scope, _ := req.Config["scope"].(string)
		if userHost == "" {
			userHost = "%"
		}
		if scope == "" {
			scope = "*.*"
		}
		if !validSQLName(username) || !validPrivilegeList(privileges) || !validScope(scope) {
			return adminFailed(req.TaskID, "invalid revoke input"), nil
		}
		output, err = runMySQLExecSafe(ctx, host, port, user, pass,
			fmt.Sprintf("REVOKE %s ON %s FROM '%s'@'%s'", privileges, scope, escapeSQL(username), escapeSQL(userHost)))
	case "show_variables":
		pattern, _ := req.Config["pattern"].(string)
		if pattern == "" {
			pattern = "%"
		}
		output, err = runMySQLExecSafe(ctx, host, port, user, pass,
			fmt.Sprintf("SHOW GLOBAL VARIABLES LIKE '%s'", escapeSQL(pattern)))
	case "show_replication_status":
		output, err = queryReplicationStatus(ctx, req.Config)
	case "set_variable":
		name, _ := req.Config["name"].(string)
		value, _ := req.Config["value"].(string)
		if !validSQLName(name) {
			return adminFailed(req.TaskID, "invalid variable name"), nil
		}
		output, err = runMySQLExecSafe(ctx, host, port, user, pass,
			fmt.Sprintf("SET GLOBAL %s = %s", name, formatVariableValue(value)))
	case "read_config":
		path, tried := resolveConfigPath(req.Config)
		if path == "" {
			return adminFailed(req.TaskID, "config file not found; tried: "+strings.Join(tried, ", ")), nil
		}
		data, readErr := os.ReadFile(path)
		output, err = string(data), readErr
		req.Config["resolved_path"] = path
	case "write_config":
		path, _ := resolveConfigPath(req.Config)
		if path == "" {
			path = configPath(req.Config)
		}
		content, _ := req.Config["content"].(string)
		if content == "" {
			return adminFailed(req.TaskID, "config content is required"), nil
		}
		if old, readErr := os.ReadFile(path); readErr == nil {
			_ = os.WriteFile(path+".bak."+time.Now().Format("20060102150405"), old, 0o600)
		}
		err = os.WriteFile(path, []byte(content), 0o600)
		output = "config written: " + path
	case "service_control":
		verb, _ := req.Config["verb"].(string)
		verb = strings.ToLower(strings.TrimSpace(verb))
		if verb == "" {
			verb = "status"
		}
		req.Config["verb"] = verb
		if verb != "start" && verb != "stop" && verb != "restart" && verb != "status" {
			return adminFailed(req.TaskID, "invalid service action"), nil
		}
		if err := validateServiceControlConfig(req.Config); err != nil {
			return adminFailed(req.TaskID, err.Error()), nil
		}
		output, err = controlInstanceService(ctx, req.Config)
	case "decommission":
		// Ensure datadir has a sensible default so decommission can clean up
		if _, ok := req.Config["datadir"]; !ok || strings.TrimSpace(req.Config["datadir"].(string)) == "" {
			port := configInt(req.Config, "target_port")
			if port <= 0 {
				port = 3306
			}
			req.Config["datadir"] = fmt.Sprintf("/data/mysql/%d", port)
		}
		output, err = decommissionInstance(ctx, req.Config)
	case "query_variables":
		output, err = queryMySQLVariables(ctx, req.Config)
	default:
		return adminFailed(req.TaskID, "unsupported action: "+action), nil
	}

	if err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  100,
			Message:   fmt.Sprintf("%s failed: %v\n%s", action, err, output),
			Timestamp: time.Now(),
		}, nil
	}
	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   strings.TrimSpace(output),
		Timestamp: time.Now(),
		Data:      parseAdminOutput(action, output, req.Config),
	}, nil
}

func adminTarget(config map[string]interface{}) (string, int, string, string) {
	host, _ := config["target_host"].(string)
	if host == "" {
		host = "127.0.0.1"
	}
	port, _ := config["target_port"].(int)
	if port == 0 {
		if p, ok := config["target_port"].(float64); ok {
			port = int(p)
		}
	}
	if port == 0 {
		port = 3306
	}
	user, _ := config["target_user"].(string)
	if user == "" {
		user, _ = config["mysql_user"].(string)
	}
	if user == "" {
		user = "root"
	}
	pass, _ := config["target_pass"].(string)
	if pass == "" {
		pass, _ = config["mysql_password"].(string)
	}
	if pass == "" {
		pass, _ = config["mysql_pass"].(string)
	}
	return host, port, user, pass
}

func queryMySQLVariables(ctx context.Context, config map[string]interface{}) (string, error) {
	host, port, user, pass := adminTarget(config)
	varsRaw, _ := config["variables"].(string)
	if strings.TrimSpace(varsRaw) == "" {
		varsRaw = "datadir,basedir"
	}
	varNames := strings.Split(varsRaw, ",")
	var selects []string
	for _, v := range varNames {
		v = strings.TrimSpace(v)
		if v != "" {
			selects = append(selects, fmt.Sprintf("@@%s", v))
		}
	}
	if len(selects) == 0 {
		return "", fmt.Errorf("no variables requested")
	}
	query := "SELECT " + strings.Join(selects, ", ")
	out, err := runMySQLExecSafe(ctx, host, port, user, pass, query)
	if err != nil {
		return "", fmt.Errorf("query variables failed: %w (output: %s)", err, out)
	}
	// MySQL returns values tab-separated on one line; map back to var names
	parts := strings.Split(strings.TrimSpace(out), "\t")
	var lines []string
	for i, v := range varNames {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		val := ""
		if i < len(parts) {
			val = strings.TrimSpace(parts[i])
		}
		lines = append(lines, v+"="+val)
	}
	return strings.Join(lines, "\n"), nil
}

func queryReplicationStatus(ctx context.Context, config map[string]interface{}) (string, error) {
	host, port, user, pass := adminTarget(config)
	clusterType, _ := config["cluster_type"].(string)
	clusterType = strings.ToLower(strings.TrimSpace(clusterType))

	// Auto-detect architecture if not specified
	if clusterType == "" || clusterType == "auto" {
		clusterType = detectClusterType(ctx, host, port, user, pass)
	}

	switch clusterType {
	case "mgr":
		return queryMGRStatus(ctx, host, port, user, pass)
	case "pxc":
		return queryPXCStatus(ctx, host, port, user, pass)
	default:
		return queryHAReplicationStatus(ctx, host, port, user, pass)
	}
}

func detectClusterType(ctx context.Context, host string, port int, user, pass string) string {
	out, err := runMySQLExecSafe(ctx, host, port, user, pass,
		"SELECT COUNT(*) FROM performance_schema.replication_group_members")
	if err == nil && strings.TrimSpace(out) != "0" {
		return "mgr"
	}
	out, err = runMySQLExecSafe(ctx, host, port, user, pass,
		"SHOW STATUS LIKE 'wsrep_cluster_status'")
	if err == nil && strings.Contains(out, "wsrep_cluster_status") {
		return "pxc"
	}
	return "ha"
}

func queryHAReplicationStatus(ctx context.Context, host string, port int, user, pass string) (string, error) {
	out, err := runMySQLExecSafe(ctx, host, port, user, pass, "SHOW SLAVE STATUS")
	if err != nil {
		return "", fmt.Errorf("SHOW SLAVE STATUS failed: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		return "cluster_type=ha\nrole=master\nslave_io_running=\nslave_sql_running=\nseconds_behind_master=-1\nmaster_host=\nmaster_port=0\nlast_error=\nrelay_log_space=0", nil
	}
	parts := strings.Split(strings.TrimSpace(out), "\t")
	colNames := []string{
		"slave_io_running", "slave_sql_running", "seconds_behind_master",
		"master_host", "master_port", "last_error", "relay_log_space",
	}
	// SHOW SLAVE STATUS -N -B returns tab-separated values; column order depends on MySQL version.
	// Use SHOW SLAVE STATUS with column headers for reliable parsing.
	headerOut, _ := runMySQLExecSafe(ctx, host, port, user, pass, "SHOW SLAVE STATUS\\G")
	if headerOut != "" {
		return parseSlaveStatusG(headerOut), nil
	}
	// Fallback: positional parsing
	var lines []string
	lines = append(lines, "cluster_type=ha")
	lines = append(lines, "role=slave")
	if len(parts) > 0 {
		lines = append(lines, "slave_io_running="+strings.TrimSpace(parts[0]))
	}
	if len(parts) > 1 {
		lines = append(lines, "slave_sql_running="+strings.TrimSpace(parts[1]))
	}
	if len(parts) > 10 {
		lines = append(lines, "seconds_behind_master="+strings.TrimSpace(parts[10]))
	}
	if len(parts) > 1 {
		lines = append(lines, "master_host="+strings.TrimSpace(parts[1]))
	}
	if len(parts) > 2 {
		lines = append(lines, "master_port="+strings.TrimSpace(parts[2]))
	}
	if len(parts) > 19 {
		lines = append(lines, "last_error="+strings.TrimSpace(parts[19]))
	}
	if len(parts) > 26 {
		lines = append(lines, "relay_log_space="+strings.TrimSpace(parts[26]))
	}
	_ = colNames
	return strings.Join(lines, "\n"), nil
}

func parseSlaveStatusG(output string) string {
	lines := []string{"cluster_type=ha", "role=slave"}
	keyMap := map[string]string{
		"Slave_IO_Running":      "slave_io_running",
		"Slave_SQL_Running":     "slave_sql_running",
		"Seconds_Behind_Master": "seconds_behind_master",
		"Master_Host":           "master_host",
		"Master_Port":           "master_port",
		"Last_Error":            "last_error",
		"Relay_Log_Space":       "relay_log_space",
		"Exec_Master_Log_Pos":   "exec_master_log_pos",
		"Read_Master_Log_Pos":   "read_master_log_pos",
		"Retrieved_Gtid_Set":    "retrieved_gtid_set",
		"Executed_Gtid_Set":     "executed_gtid_set",
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if mapped, ok := keyMap[key]; ok {
				lines = append(lines, mapped+"="+val)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func queryMGRStatus(ctx context.Context, host string, port int, user, pass string) (string, error) {
	membersOut, err := runMySQLExecSafe(ctx, host, port, user, pass,
		"SELECT MEMBER_ID, MEMBER_HOST, MEMBER_PORT, MEMBER_STATE, MEMBER_ROLE FROM performance_schema.replication_group_members")
	if err != nil {
		return "", fmt.Errorf("query group members failed: %w", err)
	}
	statsOut, _ := runMySQLExecSafe(ctx, host, port, user, pass,
		"SELECT COUNT_TRANSACTIONS_APPLIED FROM performance_schema.replication_group_member_stats WHERE MEMBER_ID = @@server_uuid")

	var lines []string
	lines = append(lines, "cluster_type=mgr")
	total := 0
	online := 0
	var primaryMember string
	for _, row := range strings.Split(strings.TrimSpace(membersOut), "\n") {
		row = strings.TrimSpace(row)
		if row == "" {
			continue
		}
		cols := strings.Split(row, "\t")
		if len(cols) >= 5 {
			total++
			state := strings.TrimSpace(cols[3])
			role := strings.TrimSpace(cols[4])
			if state == "ONLINE" {
				online++
			}
			if role == "PRIMARY" {
				primaryMember = strings.TrimSpace(cols[1]) + ":" + strings.TrimSpace(cols[2])
			}
			lines = append(lines, fmt.Sprintf("member_%d=%s:%s:%s:%s", total,
				strings.TrimSpace(cols[1]), strings.TrimSpace(cols[2]), state, role))
		}
	}
	lines = append(lines, fmt.Sprintf("group_size=%d", total))
	lines = append(lines, fmt.Sprintf("online_members=%d", online))
	lines = append(lines, "primary_member="+primaryMember)
	if statsOut != "" {
		lines = append(lines, "transactions_applied="+strings.TrimSpace(statsOut))
	}
	if online == total {
		lines = append(lines, "member_state=ONLINE")
		lines = append(lines, "member_role=OK")
	} else {
		lines = append(lines, "member_state=DEGRADED")
		lines = append(lines, "member_role=DEGRADED")
	}
	return strings.Join(lines, "\n"), nil
}

func queryPXCStatus(ctx context.Context, host string, port int, user, pass string) (string, error) {
	out, err := runMySQLExecSafe(ctx, host, port, user, pass,
		"SHOW STATUS WHERE Variable_name IN ('wsrep_cluster_status','wsrep_cluster_size','wsrep_local_state','wsrep_ready','wsrep_flow_control_paused','wsrep_local_recv_queue','wsrep_local_state_comment','wsrep_cluster_conf_id')")
	if err != nil {
		return "", fmt.Errorf("query wsrep status failed: %w", err)
	}
	var lines []string
	lines = append(lines, "cluster_type=pxc")
	for _, row := range strings.Split(strings.TrimSpace(out), "\n") {
		row = strings.TrimSpace(row)
		if row == "" {
			continue
		}
		cols := strings.SplitN(row, "\t", 2)
		if len(cols) == 2 {
			lines = append(lines, strings.TrimSpace(cols[0])+"="+strings.TrimSpace(cols[1]))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func findDatadirByPort(port int) string {
	if port <= 0 {
		return ""
	}
	portFlag := fmt.Sprintf("--port=%d", port)
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			continue
		}
		cmd := strings.ReplaceAll(string(cmdline), "\x00", " ")
		if !strings.Contains(cmd, "mysqld") {
			continue
		}
		if !strings.Contains(cmd, portFlag) {
			continue
		}
		for _, part := range strings.Fields(cmd) {
			if strings.HasPrefix(part, "--datadir=") {
				return strings.TrimPrefix(part, "--datadir=")
			}
		}
		if cwd, readErr := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid)); readErr == nil && cwd != "" {
			return cwd
		}
	}
	return ""
}

func validateServiceControlConfig(config map[string]interface{}) error {
	verb, _ := config["verb"].(string)
	verb = strings.ToLower(strings.TrimSpace(verb))
	if verb == "" {
		verb = "status"
	}
	datadir, _ := config["datadir"].(string)
	if strings.TrimSpace(datadir) == "" {
		port := configInt(config, "target_port")
		if port <= 0 {
			port = 3306
		}
		if discovered := findDatadirByPort(port); discovered != "" {
			config["datadir"] = discovered
			datadir = discovered
		}
	}
	if strings.TrimSpace(datadir) == "" {
		return fmt.Errorf("service control requires instance datadir metadata")
	}
	if verb == "start" || verb == "restart" {
		if configInt(config, "target_port") <= 0 {
			return fmt.Errorf("service control %s requires instance port metadata", verb)
		}
	}
	return nil
}

func controlInstanceService(ctx context.Context, config map[string]interface{}) (string, error) {
	verb, _ := config["verb"].(string)
	basedir, _ := config["basedir"].(string)
	datadir, _ := config["datadir"].(string)
	osUser, _ := config["os_user"].(string)
	port := configInt(config, "target_port")
	if osUser == "" {
		osUser = "mysql"
	}
	if datadir == "" {
		return "", fmt.Errorf("datadir is required for instance-bound service control")
	}
	switch verb {
	case "status":
		if pid, ok := instancePID(datadir); ok && processMatchesDatadir(pid, datadir) {
			return fmt.Sprintf("running pid=%d datadir=%s port=%d", pid, datadir, port), nil
		}
		return fmt.Sprintf("stopped datadir=%s port=%d", datadir, port), nil
	case "stop":
		return stopInstanceProcess(ctx, datadir)
	case "start":
		return startInstanceProcess(ctx, basedir, datadir, osUser, port, configInt(config, "server_id"))
	case "restart":
		stopOutput, stopErr := stopInstanceProcess(ctx, datadir)
		if stopErr != nil {
			return stopOutput, stopErr
		}
		startOutput, startErr := startInstanceProcess(ctx, basedir, datadir, osUser, port, configInt(config, "server_id"))
		return strings.TrimSpace(stopOutput + "\n" + startOutput), startErr
	default:
		return "", fmt.Errorf("invalid service action")
	}
}

func decommissionInstance(ctx context.Context, config map[string]interface{}) (string, error) {
	datadir, _ := config["datadir"].(string)
	datadir = filepath.Clean(strings.TrimSpace(datadir))
	if err := validateDecommissionDatadir(datadir); err != nil {
		return "", err
	}
	stopOutput, stopErr := stopInstanceProcess(ctx, datadir)
	if stopErr != nil {
		return stopOutput, stopErr
	}
	if err := os.RemoveAll(datadir); err != nil {
		return stopOutput, fmt.Errorf("remove datadir %s failed: %w", datadir, err)
	}
	if _, err := os.Stat(datadir); !os.IsNotExist(err) {
		if err == nil {
			return stopOutput, fmt.Errorf("datadir still exists after removal: %s", datadir)
		}
		return stopOutput, fmt.Errorf("verify datadir removal failed: %w", err)
	}
	if pid, ok := findProcessByDatadir(datadir); ok {
		return stopOutput, fmt.Errorf("mysqld process still references removed datadir %s: pid=%d", datadir, pid)
	}
	return strings.TrimSpace(stopOutput + "\nremoved datadir=" + datadir), nil
}

func validateDecommissionDatadir(datadir string) error {
	if datadir == "" || datadir == "." || datadir == string(filepath.Separator) {
		return fmt.Errorf("refuse to remove unsafe datadir: %q", datadir)
	}
	clean := filepath.Clean(datadir)
	if clean != datadir {
		return fmt.Errorf("refuse to remove unclean datadir: %q", datadir)
	}
	allowedPrefixes := []string{
		"/data/mysql/",
		"/var/lib/mysql/",
		"/opt/dbops-mysql/",
		filepath.Join(os.TempDir(), "dbops-mysql-"),
		filepath.Join(os.TempDir(), "dbops-mha"),
		filepath.Join(os.TempDir(), "dbops-pxc"),
	}
	if os.Getenv("DBOPS_ALLOW_TMP_DECOMMISSION_TEST") == "1" {
		allowedPrefixes = append(allowedPrefixes, filepath.Clean(os.TempDir())+string(filepath.Separator))
	}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(clean, prefix) && strings.TrimPrefix(clean, prefix) != "" {
			return nil
		}
	}
	return fmt.Errorf("refuse to remove datadir outside managed paths: %s", datadir)
}

func instancePID(datadir string) (int, bool) {
	for _, name := range []string{"mysql.pid", "mysqld.pid"} {
		raw, err := os.ReadFile(filepath.Join(datadir, name))
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
		if err == nil && pid > 0 {
			return pid, true
		}
	}
	return findProcessByDatadir(datadir)
}

func processMatchesDatadir(pid int, datadir string) bool {
	if pid <= 0 || datadir == "" {
		return false
	}
	cleanDatadir := filepath.Clean(datadir)
	if cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid)); err == nil && filepath.Clean(cwd) == cleanDatadir {
		return true
	}
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err == nil {
		cmd := strings.ReplaceAll(string(cmdline), "\x00", " ")
		return strings.Contains(cmd, cleanDatadir)
	}
	return false
}

func stopInstanceProcess(ctx context.Context, datadir string) (string, error) {
	pid, ok := instancePID(datadir)
	if !ok {
		return "already stopped: pid file not found", nil
	}
	if !processMatchesDatadir(pid, datadir) {
		if !processExists(pid) {
			_ = os.Remove(filepath.Join(datadir, "mysql.pid"))
			return fmt.Sprintf("already stopped: stale pid file removed pid=%d datadir=%s", pid, datadir), nil
		}
		return "", fmt.Errorf("refuse to stop pid %d: process does not match datadir %s", pid, datadir)
	}
	if out, err := exec.CommandContext(ctx, "kill", "-TERM", fmt.Sprintf("%d", pid)).CombinedOutput(); err != nil {
		return string(out), err
	}
	for i := 0; i < 30; i++ {
		if !processMatchesDatadir(pid, datadir) {
			return fmt.Sprintf("stopped pid=%d datadir=%s", pid, datadir), nil
		}
		time.Sleep(time.Second)
	}
	if out, err := exec.CommandContext(ctx, "kill", "-KILL", fmt.Sprintf("%d", pid)).CombinedOutput(); err != nil {
		return string(out), err
	}
	return fmt.Sprintf("killed pid=%d datadir=%s", pid, datadir), nil
}

func findProcessByDatadir(datadir string) (int, bool) {
	if datadir == "" {
		return 0, false
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		if processMatchesDatadir(pid, datadir) {
			return pid, true
		}
	}
	return 0, false
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	if _, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err == nil {
		return true
	}
	if err := exec.Command("kill", "-0", fmt.Sprintf("%d", pid)).Run(); err == nil {
		return true
	}
	return false
}

func startInstanceProcess(ctx context.Context, basedir, datadir, osUser string, port int, serverID int) (string, error) {
	if pid, ok := instancePID(datadir); ok && processMatchesDatadir(pid, datadir) {
		return fmt.Sprintf("already running pid=%d datadir=%s", pid, datadir), nil
	}
	if port <= 0 {
		return "", fmt.Errorf("target_port is required to start instance")
	}
	mysqld := "mysqld"
	if basedir != "" {
		mysqld = filepath.Join(basedir, "bin", "mysqld")
		if _, err := os.Stat(mysqld); err != nil {
			return "", fmt.Errorf("mysqld not found at %s: %w", mysqld, err)
		}
	}
	if serverID <= 0 {
		serverID = port
	}
	args := []string{
		"--no-defaults",
		"--daemonize",
		"--datadir=" + datadir,
		"--port=" + fmt.Sprintf("%d", port),
		"--server-id=" + fmt.Sprintf("%d", serverID),
		"--log-bin=mysql-bin",
		"--binlog-format=ROW",
		"--gtid-mode=ON",
		"--enforce-gtid-consistency=ON",
		"--log-slave-updates=ON",
		"--bind-address=0.0.0.0",
		"--socket=" + filepath.Join(datadir, "mysql.sock"),
		"--pid-file=" + filepath.Join(datadir, "mysql.pid"),
		"--log-error=" + filepath.Join(datadir, "error.log"),
		"--user=" + osUser,
	}
	if basedir != "" {
		args = append(args, "--basedir="+basedir)
	}
	if out, err := exec.CommandContext(ctx, mysqld, args...).CombinedOutput(); err != nil {
		return strings.TrimSpace(string(out)), err
	}
	for i := 0; i < 30; i++ {
		if pid, ok := instancePID(datadir); ok && processMatchesDatadir(pid, datadir) {
			return fmt.Sprintf("started pid=%d datadir=%s port=%d", pid, datadir, port), nil
		}
		time.Sleep(time.Second)
	}
	return "", fmt.Errorf("mysqld start command returned but instance did not become running for datadir %s", datadir)
}

func int64Config(config map[string]interface{}, key string) int64 {
	switch v := config[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

func ensureParentTraversal(path string) error {
	parent := filepath.Dir(filepath.Clean(path))
	for parent != "." && parent != string(filepath.Separator) {
		info, err := os.Stat(parent)
		if err != nil {
			return err
		}
		mode := info.Mode()
		if mode&0o111 != 0o111 {
			if err := os.Chmod(parent, mode|0o111); err != nil {
				return err
			}
		}
		parent = filepath.Dir(parent)
	}
	return nil
}

func mysqlExecCommand(ctx context.Context, host string, port int, user string, password string, sql string) *exec.Cmd {
	if user == "" {
		user = "root"
	}
	host = normalizeLocalMySQLHost(host)

	cmd := exec.CommandContext(ctx, getMySQLBinary(), "-h", host, "-P", fmt.Sprintf("%d", port), "-u", user, "--get-server-public-key", "-e", sql)
	if password != "" {
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+password)
	}
	return cmd
}

func normalizeLocalMySQLHost(host string) string {
	if isLocalHost(host) {
		return "127.0.0.1"
	}
	return host
}

// getMySQLBinary returns the path to mysql command, preferring absolute path
func getMySQLBinary() string {
	// Use resolveBinaryPath for consistent binary discovery across the agent.
	// This searches in /opt/dbops-pxc, /usr/local/mysql, and PATH,
	// preferring MySQL 8.0+ client which fully supports --get-server-public-key
	// and caching_sha2_password authentication.
	return resolveBinaryPath("mysql")
}

func applyInitialMySQLPassword(ctx context.Context, port int, user, pass string) error {
	escapedUser := escapeSQL(user)
	escapedPass := escapeSQL(pass)
	var sql string
	if user == "root" {
		sql = fmt.Sprintf(
			"ALTER USER 'root'@'localhost' IDENTIFIED BY '%s'; "+
				"CREATE USER IF NOT EXISTS 'root'@'%%' IDENTIFIED BY '%s'; "+
				"GRANT ALL PRIVILEGES ON *.* TO 'root'@'%%' WITH GRANT OPTION; FLUSH PRIVILEGES;",
			escapedPass, escapedPass,
		)
	} else {
		sql = fmt.Sprintf(
			"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
				"GRANT ALL PRIVILEGES ON *.* TO '%s'@'%%' WITH GRANT OPTION; FLUSH PRIVILEGES;",
			escapedUser, escapedPass, escapedUser,
		)
	}
	// Try socket connection first (for Percona Server 8.0 which initializes without password)
	dataDir := fmt.Sprintf("/data/mysql/%d", port)
	socketPath := filepath.Join(dataDir, "mysql.sock")
	attempts := [][]string{
		{"-S", socketPath, "-u", "root", "-e", sql},
		{"--protocol=TCP", "-h", "127.0.0.1", "-P", fmt.Sprintf("%d", port), "-u", "root", "-e", sql},
		{"-P", fmt.Sprintf("%d", port), "-u", "root", "-e", sql},
	}
	var lastErr error
	var lastOut []byte
	for _, args := range attempts {
		cmd := exec.CommandContext(ctx, "mysql", args...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}
		lastErr = err
		lastOut = out
	}
	return fmt.Errorf("%v %s", lastErr, strings.TrimSpace(string(lastOut)))
}

func runMySQLExecSafe(ctx context.Context, host string, port int, user, pass, sql string) (string, error) {
	args := []string{"-h", host, "-P", fmt.Sprintf("%d", port), "-u", user, "-N", "-B", "-e", sql}
	cmd := exec.CommandContext(ctx, getMySQLBinary(), args...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+pass)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func changeMySQLUserPassword(ctx context.Context, host string, port int, user, pass, username, userHost, password string) (string, error) {
	alterSQL := fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s'", escapeSQL(username), escapeSQL(userHost), escapeSQL(password))

	execSQL := func(sql string) (string, error) {
		out, err := mysqlExecWithSocketFallback(ctx, host, port, user, pass, sql)
		return string(out), err
	}

	tryAlter := func(sql string) (string, bool, error) {
		out, err := execSQL(sql)
		if err == nil {
			return out, true, nil
		}
		if strings.Contains(out, "1396") || strings.Contains(out, "Operation ALTER USER failed") {
			return out, false, nil
		}
		return out, false, err
	}

	output, userExists, err := tryAlter(alterSQL)
	if userExists {
		return output, nil
	}
	if err == nil {
		localhostSQL := fmt.Sprintf("ALTER USER '%s'@'localhost' IDENTIFIED BY '%s'", escapeSQL(username), escapeSQL(password))
		localhostOut, _, localhostErr := tryAlter(localhostSQL)
		if localhostErr == nil {
			return localhostOut, nil
		}
		return output, localhostErr
	}
	if !strings.Contains(strings.ToLower(output), "read-only") {
		return output, err
	}

	stateOut, _ := execSQL("SELECT @@GLOBAL.read_only, @@GLOBAL.super_read_only")
	restoreReadOnly, restoreSuperReadOnly := parseReadOnlyState(stateOut)
	disableSQL := "SET GLOBAL super_read_only=OFF; SET GLOBAL read_only=OFF"
	if disableOut, disableErr := execSQL(disableSQL); disableErr != nil {
		return strings.TrimSpace(output + "\n" + disableOut), err
	}
	alterOut, userExists, alterErr := tryAlter(alterSQL)
	if !userExists {
		localhostSQL := fmt.Sprintf("ALTER USER '%s'@'localhost' IDENTIFIED BY '%s'", escapeSQL(username), escapeSQL(password))
		alterOut, _, alterErr = tryAlter(localhostSQL)
	}
	restoreSQL := fmt.Sprintf("SET GLOBAL read_only=%s; SET GLOBAL super_read_only=%s", boolSQLValue(restoreReadOnly), boolSQLValue(restoreSuperReadOnly))
	restoreOut, restoreErr := execSQL(restoreSQL)
	combined := strings.TrimSpace(output + "\n" + alterOut + "\n" + restoreOut)
	if alterErr != nil {
		return combined, alterErr
	}
	if restoreErr != nil {
		return combined, fmt.Errorf("password changed but failed to restore read_only state: %w", restoreErr)
	}
	return combined, nil
}

func parseReadOnlyState(output string) (bool, bool) {
	fields := strings.Fields(output)
	if len(fields) < 2 {
		return false, false
	}
	return mysqlBool(fields[0]), mysqlBool(fields[1])
}

func mysqlBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "on", "true", "yes":
		return true
	default:
		return false
	}
}

func boolSQLValue(v bool) string {
	if v {
		return "ON"
	}
	return "OFF"
}

func adminFailed(taskID, msg string) *TaskResult {
	return &TaskResult{TaskID: taskID, Status: "failed", Progress: 100, Message: msg, Timestamp: time.Now()}
}

func parseAdminOutput(action, output string, config map[string]interface{}) any {
	if action == "read_config" {
		path, _ := config["resolved_path"].(string)
		return map[string]string{"path": path, "content": output, "output": output}
	}
	if action != "list_users" && action != "show_variables" {
		return map[string]string{"output": output}
	}
	rows := make([]map[string]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		row := map[string]string{}
		if action == "list_users" {
			keys := []string{"user", "host", "plugin", "account_locked"}
			for i, key := range keys {
				if i < len(parts) {
					row[key] = parts[i]
				}
			}
		} else {
			if len(parts) >= 2 {
				row["name"] = parts[0]
				row["value"] = parts[1]
			}
		}
		rows = append(rows, row)
	}
	return map[string]any{"rows": rows}
}

func validSQLName(s string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9_.$-]+$`).MatchString(s)
}

func validPrivilegeList(s string) bool {
	if s == "" {
		return false
	}
	for _, item := range strings.Split(s, ",") {
		p := strings.TrimSpace(strings.ToUpper(item))
		if p == "" || !regexp.MustCompile(`^[A-Z_ ]+$`).MatchString(p) {
			return false
		}
	}
	return true
}

func validScope(s string) bool {
	if s == "*.*" {
		return true
	}
	return regexp.MustCompile(`^[A-Za-z0-9_$*]+(\.[A-Za-z0-9_$*]+)?$`).MatchString(s)
}

func formatVariableValue(value string) string {
	trimmed := strings.TrimSpace(value)
	upper := strings.ToUpper(trimmed)
	if regexp.MustCompile(`^-?\d+(\.\d+)?$`).MatchString(trimmed) {
		return trimmed
	}
	switch upper {
	case "ON", "OFF", "TRUE", "FALSE", "DEFAULT":
		return upper
	}
	return "'" + escapeSQL(trimmed) + "'"
}

func marshalData(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
