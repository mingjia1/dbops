package executor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	TargetDir       string `json:"target_dir"`
	MySQLHost       string `json:"mysql_host"`
	MySQLPort       int    `json:"mysql_port"`
	MySQLUser       string `json:"mysql_user"`
	MySQLPass       string `json:"mysql_pass"`
	BaseBackupPath  string `json:"base_backup_path"`
	BaseBackupLabel string `json:"base_backup_label"`
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
	case "mha":
		mhaExecutor := NewMHAExecutor()
		return mhaExecutor.DeployMHA(ctx, req)
	case "mgr":
		mgrExecutor := NewMGRExecutor()
		if bootstrap, ok := req.Config["bootstrap"].(bool); ok && !bootstrap {
			return mgrExecutor.ConfigureGroupMember(ctx, req)
		}
		return mgrExecutor.DeployMGRSinglePrimary(ctx, req)
	case "pxc":
		return e.DeployPXC(ctx, req)
	default:
		return e.deploySingleInstance(ctx, req)
	}
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
	if err := initCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("MySQL initialization failed: %v", err),
			Timestamp: time.Now(),
		}, nil
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
	if err := startCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("MySQL start failed: %v", err),
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

	serverIDCmd := mysqlExecCommand(ctx, config.MasterHost, config.MasterPort, config.MySQLUser, config.MySQLPass, "SET GLOBAL server_id=1;")

	if out, err := serverIDCmd.CombinedOutput(); err != nil {
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
	serverIDCmd := mysqlExecCommand(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, fmt.Sprintf("SET GLOBAL server_id=%d;", config.ServerID+2))

	if out, err := serverIDCmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Failed to set slave server_id: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}

	masterStatusCmd := mysqlExecCommand(ctx, config.MasterHost, config.MasterPort, config.MySQLUser, config.MySQLPass, "SHOW MASTER STATUS;")
	masterStatusOut, err := masterStatusCmd.CombinedOutput()
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

	resetCmd := mysqlExecCommand(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "STOP SLAVE; RESET SLAVE ALL;")
	if out, err := resetCmd.CombinedOutput(); err != nil {
		output := strings.TrimSpace(string(out))
		if !strings.Contains(output, "Slave is not configured") {
			return &TaskResult{
				Status:    "failed",
				Progress:  58,
				Message:   fmt.Sprintf("Failed to reset existing slave configuration: %v, output: %s", err, output),
				Timestamp: time.Now(),
			}
		}
	}

	changeMasterSQL := fmt.Sprintf(
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

	changeCmd := mysqlExecCommand(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, changeMasterSQL)

	if out, err := changeCmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("Failed to execute CHANGE MASTER TO: %v, output: %s", err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}
	}

	startCmd := mysqlExecCommand(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "START SLAVE;")

	if out, err := startCmd.CombinedOutput(); err != nil {
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

	cmd := mysqlExecCommand(ctx, config.SlaveHost, config.SlavePort, config.MySQLUser, config.MySQLPass, "SHOW SLAVE STATUS\\G")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  90,
			Message:   fmt.Sprintf("Failed to check slave status: %v, output: %s", err, strings.TrimSpace(string(output))),
			Timestamp: time.Now(),
		}
	}

	if !strings.Contains(string(output), "Slave_IO_Running: Yes") ||
		!strings.Contains(string(output), "Slave_SQL_Running: Yes") {
		return &TaskResult{
			Status:    "failed",
			Progress:  95,
			Message:   "Replication not running correctly",
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

	return e.executeXtrabackup(ctx, config)
}

func parseBackupConfig(config map[string]interface{}) BackupConfig {
	bc := BackupConfig{
		BackupType: "full",
		TargetDir:  "/backup/mysql",
	}

	if v, ok := config["backup_type"].(string); ok {
		bc.BackupType = v
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
	if v, ok := config["base_backup_path"].(string); ok {
		bc.BaseBackupPath = v
	}
	if v, ok := config["base_backup_label"].(string); ok {
		bc.BaseBackupLabel = v
	}

	return bc
}

func (e *TaskExecutor) executeXtrabackup(ctx context.Context, config BackupConfig) (*TaskResult, error) {
	// P0: 之前用 time.Now().Unix() 1 秒精度, 同秒并发会写同 dir 互相覆盖.
	// 改 UnixNano 纳秒精度, 并发 backup 不可能撞.
	if _, err := exec.LookPath("xtrabackup"); err != nil {
		if config.BackupType == "incremental" {
			return &TaskResult{
				Status:    "failed",
				Progress:  0,
				Message:   "incremental backup requires xtrabackup on target host",
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

	backupCmd := exec.CommandContext(ctx, "xtrabackup",
		"--backup",
		"--target-dir="+backupDir,
		"--host="+config.MySQLHost,
		"--port="+fmt.Sprintf("%d", config.MySQLPort),
		"--user="+config.MySQLUser,
		"--password="+config.MySQLPass,
	)
	if config.BackupType == "incremental" {
		if config.BaseBackupPath == "" {
			return &TaskResult{
				Status:    "failed",
				Progress:  0,
				Message:   "incremental backup requires a completed full backup",
				Timestamp: time.Now(),
			}, nil
		}
		backupCmd.Args = append(backupCmd.Args, "--incremental-basedir="+config.BaseBackupPath)
	}

	// A3 part 1: --stream=xbstream 写到 stdout, 之前 Stdout=nil 把流扔了,
	// sha256sum 必然拿不到文件, 然后假装 "checksum-unavailable" + 报 completed.
	// 修: 把 xbstream 流到磁盘, sha256sum 才能算出真 hash.
	if out, err := backupCmd.CombinedOutput(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Xtrabackup backup failed: %v\n%s", err, strings.TrimSpace(string(out))),
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
			"base_backup_path":  config.BaseBackupPath,
			"base_backup_label": config.BaseBackupLabel,
			"file_size":         sizeBytes,
			"checksum":          checksum,
		},
	}, nil
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

	args := []string{
		"--single-transaction",
		"--routines",
		"--events",
		"--triggers",
		"--all-databases",
		"-h", defaultString(config.MySQLHost, "127.0.0.1"),
		"-P", fmt.Sprintf("%d", config.MySQLPort),
		"-u", defaultString(config.MySQLUser, "root"),
	}
	cmd := exec.CommandContext(ctx, "mysqldump", args...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+config.MySQLPass)
	cmd.Stdout = outFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = outFile.Close()
		_ = os.Remove(backupFile)
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("mysqldump backup failed: %v\n%s", err, strings.TrimSpace(stderr.String())),
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
			"backup_path": backupFile,
			"backup_type": "logical",
			"file_size":   sizeBytes,
			"checksum":    checksum,
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

func (e *TaskExecutor) ExecuteHealthCheck(ctx context.Context, instanceID string) (*TaskResult, error) {
	data := collectSystemInfo(ctx)
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
	cmd := exec.CommandContext(ctx, "mysqladmin", "ping", "-h", "localhost", "-u", "root")

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
	if v, ok := config["target_pass"].(string); ok {
		c.TargetPass = v
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

func runMySQLExec(ctx context.Context, host string, port int, user, pass, sql string) (string, error) {
	args := append(mysqlBaseArgs(host, port, user, pass), "-N", "-B", "-e", sql)
	cmd := exec.CommandContext(ctx, "mysql", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
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

	role := "replica"
	if !strings.Contains(output, "1") {
		role = "primary"
	}

	slaveOutput, _ := runMySQLExec(ctx, host, port, user, pass, "SHOW SLAVE STATUS\\G")
	slaveRunning := strings.Contains(slaveOutput, "Slave_IO_Running: Yes") &&
		strings.Contains(slaveOutput, "Slave_SQL_Running: Yes")

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("role=%s read_only_state_checked slave_running=%t", role, slaveRunning),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) ExecuteRolePromote(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	cfg := parseRoleSwitchConfig(req.Config)

	if cfg.GracePeriodSec > 0 {
		time.Sleep(time.Duration(cfg.GracePeriodSec) * time.Second)
	}

	host, port, user, pass := resolveTargetConn(cfg)

	steps := []struct {
		sql   string
		label string
	}{
		{"STOP SLAVE", "stop slave"},
		{"STOP SLAVE IO_THREAD", "stop slave IO thread"},
		{"SET GLOBAL read_only = OFF", "disable read_only"},
		{"SET GLOBAL super_read_only = OFF", "disable super_read_only"},
		{"RESET SLAVE ALL", "reset slave"},
	}

	for _, step := range steps {
		if _, err := runMySQLExec(ctx, host, port, user, pass, step.sql); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  30,
				Message:   fmt.Sprintf("promote step %q failed: %v", step.label, err),
				Timestamp: time.Now(),
			}, nil
		}
	}

	// A3 part 3: MGR promote 之前发 'THISUUID' 永远失败 (或 force=true 时静默成功).
	// 修: 用 cfg.NewMasterServerUUID, 由 backend 解析新 primary 的 @@server_uuid 后下发.
	if cfg.ClusterType == "mgr" {
		if cfg.NewMasterServerUUID == "" {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  80,
				Message:   "MGR promote requires new_master_server_uuid in request config",
				Timestamp: time.Now(),
			}, nil
		}
		mgrSQL := fmt.Sprintf("SELECT group_replication_set_as_primary('%s')", cfg.NewMasterServerUUID)
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
		Message:   fmt.Sprintf("instance %s demoted to %s within %s cluster", cfg.InstanceID, cfg.TargetRole, cfg.ClusterType),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) ExecuteRoleReplicaRebuild(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	cfg := parseRoleSwitchConfig(req.Config)
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

	changeSQL := fmt.Sprintf(
		"STOP SLAVE; CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_AUTO_POSITION=1; START SLAVE;",
		cfg.NewMasterHost, cfg.NewMasterPort, cfg.NewMasterUser, cfg.NewMasterPass,
	)
	if _, err := runMySQLExec(ctx, host, port, user, pass, changeSQL); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("replica rebuild failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("replica %s re-pointed to new master %s (%s:%d)", cfg.InstanceID, cfg.NewMasterID, cfg.NewMasterHost, cfg.NewMasterPort),
		Timestamp: time.Now(),
	}, nil
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
		if verb != "start" && verb != "stop" && verb != "restart" && verb != "status" {
			return adminFailed(req.TaskID, "invalid service action"), nil
		}
		output, err = controlInstanceService(ctx, req.Config)
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
		user = "root"
	}
	pass, _ := config["target_pass"].(string)
	return host, port, user, pass
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
		return startInstanceProcess(ctx, basedir, datadir, osUser, port)
	case "restart":
		stopOutput, stopErr := stopInstanceProcess(ctx, datadir)
		if stopErr != nil {
			return stopOutput, stopErr
		}
		startOutput, startErr := startInstanceProcess(ctx, basedir, datadir, osUser, port)
		return strings.TrimSpace(stopOutput + "\n" + startOutput), startErr
	default:
		return "", fmt.Errorf("invalid service action")
	}
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
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	cmd := strings.ReplaceAll(string(cmdline), "\x00", " ")
	return strings.Contains(cmd, datadir)
}

func stopInstanceProcess(ctx context.Context, datadir string) (string, error) {
	pid, ok := instancePID(datadir)
	if !ok {
		return "already stopped: pid file not found", nil
	}
	if !processMatchesDatadir(pid, datadir) {
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

func startInstanceProcess(ctx context.Context, basedir, datadir, osUser string, port int) (string, error) {
	if pid, ok := instancePID(datadir); ok && processMatchesDatadir(pid, datadir) {
		return fmt.Sprintf("already running pid=%d datadir=%s", pid, datadir), nil
	}
	if basedir == "" {
		return "", fmt.Errorf("basedir is required to start instance")
	}
	if port <= 0 {
		return "", fmt.Errorf("target_port is required to start instance")
	}
	mysqld := filepath.Join(basedir, "bin", "mysqld")
	if _, err := os.Stat(mysqld); err != nil {
		return "", fmt.Errorf("mysqld not found at %s: %w", mysqld, err)
	}
	args := []string{
		"--no-defaults",
		"--daemonize",
		"--basedir=" + basedir,
		"--datadir=" + datadir,
		"--port=" + fmt.Sprintf("%d", port),
		"--bind-address=0.0.0.0",
		"--socket=" + filepath.Join(datadir, "mysql.sock"),
		"--pid-file=" + filepath.Join(datadir, "mysql.pid"),
		"--log-error=" + filepath.Join(datadir, "error.log"),
		"--user=" + osUser,
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

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
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
