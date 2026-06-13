package executor

import (
	"archive/tar"
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
}

func NewTaskExecutor() *TaskExecutor {
	return &TaskExecutor{
		UpgradeExecutor:   NewUpgradeExecutor(),
		MigrationExecutor: NewMigrationExecutor(),
	}
}

// ExecuteVerifyMigration A1: 之前 /agent/tasks/migration-verify 路由不存在,
// backend 调过去 404, 现在转发到 MigrationExecutor.VerifyMigration.
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

// isDataDirInitialized checks if the MySQL data directory has been initialized
// by looking for ibdata1 (InnoDB system tablespace) which is always created during --initialize-insecure.
func isDataDirInitialized(dataDir string) bool {
	info, err := os.Stat(filepath.Join(dataDir, "ibdata1"))
	return err == nil && info.Size() > 0
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

	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 3306
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
	needInit := true
	if _, err := os.Stat(dataDir); err == nil {
		if isDataDirInitialized(dataDir) {
			needInit = false
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
	// Falls back to "whatever is on PATH" if package_url is absent (legacy).
	var mysqld string
	if packageURL != "" {
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
		mysqld = "mysqld"
		if basedir != "" {
			mysqld = filepath.Join(basedir, "bin", "mysqld")
		}
	}

	// Only run initialization if the data directory hasn't been initialized yet.
	if needInit {
		// Percona/Percona XtraDB Cluster mysqld requires the data directory to NOT
		// exist when running --initialize-insecure. Remove it if we just created it.
		os.RemoveAll(dataDir)

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
		"--server-id=" + fmt.Sprintf("%d", port),
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

	if mysqlPass != "" {
		if mysqlUser == "" {
			mysqlUser = "root"
		}
		if err := applyInitialMySQLPassword(ctx, port, mysqlUser, mysqlPass); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  70,
				Message:   fmt.Sprintf("apply configured MySQL password failed: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
	}

	healthCmd := exec.CommandContext(ctx, "mysqladmin", "-h", host, "-P", fmt.Sprintf("%d", port), "-u", defaultString(mysqlUser, "root"), "ping")
	if mysqlPass != "" {
		healthCmd.Env = append(os.Environ(), "MYSQL_PWD="+mysqlPass)
	}
	if err := healthCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("Health check failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("MySQL instance deployed successfully on %s:%d", host, port),
		Timestamp: time.Now(),
	}, nil
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
		result.Progress = 100
		result.Message = fmt.Sprintf("HA master configured successfully on %s:%d", config.MasterHost, config.MasterPort)
	}
	return result, nil
}

func (e *TaskExecutor) deployHAReplica(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMasterSlaveConfig(req.Config)
	result := e.configureSlave(ctx, config)
	if result.Status == "failed" {
		if result.TaskID == "" {
			result.TaskID = req.TaskID
		}
		return result, nil
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
		ReplicatePass: "repl123",
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

	cmd := mysqlExecCommand(ctx, config.MasterHost, config.MasterPort, config.MySQLUser, config.MySQLPass, createUserSQL)

	if out, err := cmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  20,
			Message:   fmt.Sprintf("Failed to create replication user on master: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}
	_ = mysqlExecCommand(ctx, config.MasterHost, config.MasterPort, config.MySQLUser, config.MySQLPass,
		fmt.Sprintf("ALTER USER '%s'@'%%' IDENTIFIED WITH mysql_native_password BY '%s';", escapeSQL(config.ReplicateUser), escapeSQL(config.ReplicatePass))).Run()

	if out, err := setServerID(ctx, config.MasterHost, config.MasterPort, config.MySQLUser, config.MySQLPass, 1); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  30,
			Message:   fmt.Sprintf("Failed to configure master server_id: %v, output: %s", err, strings.TrimSpace(string(out))),
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
	if out, err := setServerID(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, config.ServerID+2); err != nil {
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

	if out, err := runFirstMySQL(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, changeMasterSQLNoKey, changeMasterSQL, changeSourceSQLNoKey, changeSourceSQL); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("Failed to execute CHANGE MASTER TO: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}

	if out, err := runFirstMySQL(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "START SLAVE;", "START REPLICA;"); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  70,
			Message:   fmt.Sprintf("Failed to start slave: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  80,
		Message:   "Slave configured and started successfully",
		Timestamp: time.Now(),
	}
}

func setServerID(ctx context.Context, host string, port int, user, pass string, serverID int) ([]byte, error) {
	return runFirstMySQL(ctx, host, port, user, pass,
		fmt.Sprintf("SET PERSIST server_id=%d; SET GLOBAL server_id=%d;", serverID, serverID),
		fmt.Sprintf("SET GLOBAL server_id=%d;", serverID),
	)
}

func resetReplication(ctx context.Context, config MasterSlaveConfig) ([]byte, error) {
	legacyOut, legacyErr := mysqlExecCommand(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP SLAVE; RESET SLAVE ALL;").CombinedOutput()
	legacyText := string(legacyOut)
	if legacyErr == nil || strings.Contains(legacyText, "Slave is not configured") || strings.Contains(legacyText, "This server is not configured as slave") {
		return legacyOut, nil
	}
	if !strings.Contains(legacyText, "ERROR 1064") {
		return legacyOut, legacyErr
	}
	replicaOut, replicaErr := mysqlExecCommand(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP REPLICA; RESET REPLICA ALL;").CombinedOutput()
	replicaText := string(replicaOut)
	if replicaErr == nil || strings.Contains(replicaText, "Replica is not configured") {
		return replicaOut, nil
	}
	return replicaOut, replicaErr
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

	output, err := runFirstMySQL(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "SHOW SLAVE STATUS\\G", "SHOW REPLICA STATUS\\G")
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  90,
			Message:   fmt.Sprintf("Failed to check slave status: %v, output: %s", err, strings.TrimSpace(string(output))),
			Timestamp: time.Now(),
		}
	}

	outputText := string(output)
	if !(strings.Contains(outputText, "Slave_IO_Running: Yes") || strings.Contains(outputText, "Replica_IO_Running: Yes")) ||
		!(strings.Contains(outputText, "Slave_SQL_Running: Yes") || strings.Contains(outputText, "Replica_SQL_Running: Yes")) {
		return &TaskResult{
			Status:    "failed",
			Progress:  95,
			Message:   "Replication not running correctly: " + strings.TrimSpace(outputText),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:   "completed",
		Progress: 100,
		Message: fmt.Sprintf("Master-Slave replication established successfully. Master: %s:%d, Slave: %s:%d",
			config.MasterHost, config.MasterPort, config.SlaveHost, config.SlavePort),
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
	// P0: 之前用 time.Now().Unix() 1 秒精度, 同秒并发会写同 dir 互相覆盖.
	// 改 UnixNano 纳秒精度, 并发 backup 不可能撞.
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
			"-h", host,
			"-P", fmt.Sprintf("%d", config.MySQLPort),
			"-u", defaultString(config.MySQLUser, "root"),
		}
		cmd := exec.CommandContext(ctx, "mysqldump", args...)
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+config.MySQLPass)
		cmd.Stdout = outFile
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
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
	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   msg,
		Timestamp: time.Now(),
		Data: map[string]any{
			"backup_path":   backupFile,
			"backup_type":   "logical",
			"backup_method": "mysqldump",
			"file_size":     sizeBytes,
			"checksum":      checksum,
		},
	}, nil
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

func (e *TaskExecutor) executeRestore(ctx context.Context, config RestoreConfig) (*TaskResult, error) {
	prepareCmd := exec.CommandContext(ctx, "xtrabackup",
		"--prepare",
		"--target-dir="+config.BackupPath,
	)

	if err := prepareCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  20,
			Message:   fmt.Sprintf("Restore prepare failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	copyCmd := exec.CommandContext(ctx, "xtrabackup",
		"--copy-back",
		"--target-dir="+config.BackupPath,
		"--datadir=/var/lib/mysql",
	)

	if err := copyCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("Restore copy-back failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	chownCmd := exec.CommandContext(ctx, "chown", "-R", "mysql:mysql", "/var/lib/mysql")
	if err := chownCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("Restore chown failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Restore completed successfully from %s", config.BackupPath),
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

	cmd := exec.CommandContext(ctx, "mysqladmin", "ping",
		"-h", host,
		"-P", fmt.Sprintf("%d", port),
		"-u", user)
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

// ExecuteVersionDetect P0: 之前 backend DetectVersion 直接返 error, 阻断所有
// 升级链路 (PlanUpgradePath / CheckCompatibility 都需要 source version).
// 修: agent 调 mysql -e "SELECT @@version, @@version_comment", 真实探测.
// 走 MYSQL_PWD env 传密码避免 ps 泄露.
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
	// A3 part 2/3: 之前 executor 硬编 127.0.0.1:3306 root ''.
	// 改成 backend 通过 req.Config 传入 target host/port/user/pass.
	// (即 "本实例" 的连接信息 - 同一个 agent 跑多实例时要按 instance_id 区分)
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

// resolveTargetConn A3 part 2 prefers target connection values from cfg.
// Missing values fall back to 127.0.0.1:3306 root with an empty password, with a warning.
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
	// A3 part 3: 之前直接用 cfg.NewMasterID (UUID) 当 MASTER_HOST, 复制永远起不来.
	// 修: 优先用 cfg.NewMasterHost/Port/User/Pass, 缺失时 fail 让人补, 不再猜.
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
		output, err = runMySQLExecSafe(ctx, host, port, user, pass,
			fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s'", escapeSQL(username), escapeSQL(userHost), escapeSQL(password)))
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
		out, runErr := runFirstMySQL(ctx, host, port, user, pass, "SHOW SLAVE STATUS\\G", "SHOW REPLICA STATUS\\G")
		output, err = string(out), runErr
	case "set_variable":
		name, _ := req.Config["name"].(string)
		value, _ := req.Config["value"].(string)
		if !validSQLName(name) {
			return adminFailed(req.TaskID, "invalid variable name"), nil
		}
		output, err = runMySQLExecSafe(ctx, host, port, user, pass,
			fmt.Sprintf("SET GLOBAL %s = %s", name, formatVariableValue(value)))
	case "read_config":
		path := configPath(req.Config)
		data, readErr := os.ReadFile(path)
		output, err = string(data), readErr
	case "write_config":
		path := configPath(req.Config)
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
		if err := validateServiceControlConfig(req.Config); err != nil {
			return adminFailed(req.TaskID, err.Error()), nil
		}
		output, err = decommissionInstance(ctx, req.Config)
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
		Data:      parseAdminOutput(action, output),
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

func validateServiceControlConfig(config map[string]interface{}) error {
	verb, _ := config["verb"].(string)
	verb = strings.ToLower(strings.TrimSpace(verb))
	if verb == "" {
		verb = "status"
	}
	datadir, _ := config["datadir"].(string)
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
	raw, err := os.ReadFile(filepath.Join(datadir, "mysql.pid"))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
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

func configInt(config map[string]interface{}, key string) int {
	if v, ok := config[key].(int); ok {
		return v
	}
	if v, ok := config[key].(float64); ok {
		return int(v)
	}
	return 0
}

func configString(config map[string]interface{}, key string) string {
	if v, ok := config[key].(string); ok {
		return v
	}
	return ""
}

func configBool(config map[string]interface{}, key string) bool {
	if v, ok := config[key].(bool); ok {
		return v
	}
	return false
}

func backupMySQLHostCandidates(host string) []string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "127.0.0.1"
	}
	candidates := []string{host}
	if isLocalHost(host) {
		candidates = append(candidates, "localhost", "127.0.0.1")
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" && !seen[candidate] {
			seen[candidate] = true
			out = append(out, candidate)
		}
	}
	return out
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

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
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
	cmd := exec.CommandContext(ctx, "mysql", "-h", host, "-P", fmt.Sprintf("%d", port), "-u", user, "-e", sql)
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
	attempts := [][]string{
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
	cmd := exec.CommandContext(ctx, "mysql", args...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+pass)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func adminFailed(taskID, msg string) *TaskResult {
	return &TaskResult{TaskID: taskID, Status: "failed", Progress: 100, Message: msg, Timestamp: time.Now()}
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func configPath(config map[string]interface{}) string {
	path, _ := config["path"].(string)
	if path == "" {
		return "/etc/my.cnf"
	}
	return path
}

func parseAdminOutput(action, output string) any {
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
