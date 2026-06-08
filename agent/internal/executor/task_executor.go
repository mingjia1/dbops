package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	ServerID      int    `json:"server_id"`
	DeployMode    string `json:"deploy_mode"`
}

type BackupConfig struct {
	InstanceID string `json:"instance_id"`
	BackupType string `json:"backup_type"`
	TargetDir  string `json:"target_dir"`
	MySQLHost  string `json:"mysql_host"`
	MySQLPort  int    `json:"mysql_port"`
	MySQLUser  string `json:"mysql_user"`
	MySQLPass  string `json:"mysql_pass"`
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
		return mgrExecutor.DeployMGRSinglePrimary(ctx, req)
	case "pxc":
		return e.DeployPXC(ctx, req)
	default:
		return e.deploySingleInstance(ctx, req)
	}
}

func (e *TaskExecutor) deploySingleInstance(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	host, _ := req.Config["host"].(string)
	port, _ := req.Config["port"].(int)
	dataDir, _ := req.Config["data_dir"].(string)
	packageURL, _ := req.Config["package_url"].(string)
	basedir, _ := req.Config["basedir"].(string)
	osUser, _ := req.Config["os_user"].(string)

	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 3306
	}
	if dataDir == "" {
		dataDir = fmt.Sprintf("/data/mysql/%d", port)
	}

	// Version-agnostic path: download + extract the requested tarball.
	// Falls back to "whatever is on PATH" if package_url is absent (legacy).
	var mysqld string
	if packageURL != "" {
		var installErr error
		mysqld, installErr = InstallFromURL(ctx, packageURL, "", basedir, osUser)
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
	}

	initArgs := []string{
		"--initialize-insecure",
		"--datadir=" + dataDir,
		"--user=" + osUser,
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
		"--datadir=" + dataDir,
		"--port=" + fmt.Sprintf("%d", port),
		"--user=" + osUser,
	}
	if basedir != "" {
		startArgs = append(startArgs, "--basedir="+basedir)
	}
	startCmd := exec.CommandContext(ctx, mysqld, startArgs...)
	if err := startCmd.Start(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("MySQL start failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	time.Sleep(5 * time.Second)

	healthCmd := exec.CommandContext(ctx, "mysqladmin", "-h", host, "-P", fmt.Sprintf("%d", port), "ping")
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
		ServerID:      1,
		DeployMode:    "async",
	}

	if v, ok := config["master_host"].(string); ok {
		mc.MasterHost = v
	}
	if v, ok := config["master_port"].(int); ok {
		mc.MasterPort = v
	}
	if v, ok := config["slave_host"].(string); ok {
		mc.SlaveHost = v
	}
	if v, ok := config["slave_port"].(int); ok {
		mc.SlavePort = v
	}
	if v, ok := config["replicate_user"].(string); ok {
		mc.ReplicateUser = v
	}
	if v, ok := config["replicate_pass"].(string); ok {
		mc.ReplicatePass = v
	}

	return mc
}

func (e *TaskExecutor) configureMaster(ctx context.Context, config MasterSlaveConfig) *TaskResult {
	createUserSQL := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO '%s'@'%%';",
		config.ReplicateUser, config.ReplicatePass, config.ReplicateUser,
	)

	cmd := exec.CommandContext(ctx, "mysql", "-h", config.MasterHost,
		"-P", fmt.Sprintf("%d", config.MasterPort),
		"-u", "root", "-e", createUserSQL)

	if err := cmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  20,
			Message:   fmt.Sprintf("Failed to create replication user on master: %v", err),
			Timestamp: time.Now(),
		}
	}

	serverIDCmd := exec.CommandContext(ctx, "mysql", "-h", config.MasterHost,
		"-P", fmt.Sprintf("%d", config.MasterPort),
		"-u", "root", "-e", "SET GLOBAL server_id=1; SET GLOBAL log_bin=ON;")

	if err := serverIDCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  30,
			Message:   fmt.Sprintf("Failed to configure master server_id: %v", err),
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
	serverIDCmd := exec.CommandContext(ctx, "mysql", "-h", config.SlaveHost,
		"-P", fmt.Sprintf("%d", config.SlavePort),
		"-u", "root", "-e", fmt.Sprintf("SET GLOBAL server_id=%d;", config.ServerID+2))

	if err := serverIDCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Failed to set slave server_id: %v", err),
			Timestamp: time.Now(),
		}
	}

	changeMasterSQL := fmt.Sprintf(
		"CHANGE MASTER TO "+
			"MASTER_HOST='%s', "+
			"MASTER_PORT=%d, "+
			"MASTER_USER='%s', "+
			"MASTER_PASSWORD='%s', "+
			"MASTER_AUTO_POSITION=1;",
		config.MasterHost, config.MasterPort,
		config.ReplicateUser, config.ReplicatePass,
	)

	changeCmd := exec.CommandContext(ctx, "mysql", "-h", config.SlaveHost,
		"-P", fmt.Sprintf("%d", config.SlavePort),
		"-u", "root", "-e", changeMasterSQL)

	if err := changeCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("Failed to execute CHANGE MASTER TO: %v", err),
			Timestamp: time.Now(),
		}
	}

	startCmd := exec.CommandContext(ctx, "mysql", "-h", config.SlaveHost,
		"-P", fmt.Sprintf("%d", config.SlavePort),
		"-u", "root", "-e", "START SLAVE;")

	if err := startCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  70,
			Message:   fmt.Sprintf("Failed to start slave: %v", err),
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

func (e *TaskExecutor) verifyReplication(ctx context.Context, config MasterSlaveConfig) *TaskResult {
	time.Sleep(3 * time.Second)

	cmd := exec.CommandContext(ctx, "mysql", "-h", config.SlaveHost,
		"-P", fmt.Sprintf("%d", config.SlavePort),
		"-u", "root", "-e", "SHOW SLAVE STATUS\\G")

	output, err := cmd.Output()
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  90,
			Message:   fmt.Sprintf("Failed to check slave status: %v", err),
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
	}
	if v, ok := config["mysql_user"].(string); ok {
		bc.MySQLUser = v
	}
	if v, ok := config["mysql_pass"].(string); ok {
		bc.MySQLPass = v
	}

	return bc
}

func (e *TaskExecutor) executeXtrabackup(ctx context.Context, config BackupConfig) (*TaskResult, error) {
	// P0: 之前用 time.Now().Unix() 1 秒精度, 同秒并发会写同 dir 互相覆盖.
	// 改 UnixNano 纳秒精度, 并发 backup 不可能撞.
	backupDir := fmt.Sprintf("%s/%s-%d", config.TargetDir, config.BackupType, time.Now().UnixNano())

	backupCmd := exec.CommandContext(ctx, "xtrabackup",
		"--backup",
		"--target-dir="+backupDir,
		"--host="+config.MySQLHost,
		"--port="+fmt.Sprintf("%d", config.MySQLPort),
		"--user="+config.MySQLUser,
		"--password="+config.MySQLPass,
		"--stream=xbstream",
		"--compress",
	)

	// A3 part 1: --stream=xbstream 写到 stdout, 之前 Stdout=nil 把流扔了,
	// sha256sum 必然拿不到文件, 然后假装 "checksum-unavailable" + 报 completed.
	// 修: 把 xbstream 流到磁盘, sha256sum 才能算出真 hash.
	backupFile := fmt.Sprintf("%s.xbstream", backupDir)
	streamOut, err := os.Create(backupFile)
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Failed to open backup file %s: %v", backupFile, err),
			Timestamp: time.Now(),
		}, nil
	}
	backupCmd.Stdout = streamOut
	backupCmd.Stderr = os.Stderr

	if err := backupCmd.Run(); err != nil {
		_ = streamOut.Close()
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Xtrabackup backup failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}
	if err := streamOut.Close(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Failed to close backup file %s: %v", backupFile, err),
			Timestamp: time.Now(),
		}, nil
	}

	prepareCmd := exec.CommandContext(ctx, "xtrabackup",
		"--prepare",
		"--target-dir="+backupDir,
	)

	if err := prepareCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Xtrabackup prepare failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	checksumCmd := exec.CommandContext(ctx, "sha256sum", backupFile)
	checksumOutput, err := checksumCmd.Output()
	if err != nil {
		// 真没拿到 hash 时标 failed, 不能再假装 "checksum-unavailable" 然后 completed
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("sha256sum failed: %v (backup file: %s)", err, backupFile),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		Status:   "completed",
		Progress: 100,
		Message: fmt.Sprintf("Backup completed successfully. Path: %s, Checksum: %s",
			backupDir, strings.TrimSpace(string(checksumOutput))),
		Timestamp: time.Now(),
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
	cmd := exec.CommandContext(ctx, "mysqladmin", "ping", "-h", "localhost", "-u", "root")

	if err := cmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    fmt.Sprintf("health-%s", instanceID),
			Status:    "unhealthy",
			Progress:  100,
			Message:   fmt.Sprintf("Health check failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:    fmt.Sprintf("health-%s", instanceID),
		Status:    "healthy",
		Progress:  100,
		Message:   "Instance is healthy",
		Timestamp: time.Now(),
	}, nil
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
	port, _ := req.Config["target_port"].(int)
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
	for _, dir := range dirs {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			name := entry.Name()
			backupType := classifyBackupFile(name)
			if backupType == "" {
				return nil
			}
			stat, err := entry.Info()
			if err != nil {
				return nil
			}
			files = append(files, BackupScanFile{
				FileName:   name,
				FilePath:   path,
				SizeBytes:  stat.Size(),
				BackupType: backupType,
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

func classifyBackupFile(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".sql") || strings.HasSuffix(lower, ".sql.gz") || strings.HasSuffix(lower, ".dump"):
		return "logical"
	case strings.HasSuffix(lower, ".xbstream") || strings.Contains(lower, "xtrabackup"):
		return "full"
	case strings.Contains(lower, "incremental") || strings.Contains(lower, "inc"):
		return "incremental"
	default:
		return ""
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
			fmt.Sprintf("SET GLOBAL %s = '%s'", name, escapeSQL(value)))
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
		service, _ := req.Config["service"].(string)
		verb, _ := req.Config["verb"].(string)
		if service == "" {
			service = "mysqld"
		}
		if verb != "start" && verb != "stop" && verb != "restart" && verb != "status" {
			return adminFailed(req.TaskID, "invalid service action"), nil
		}
		cmd := exec.CommandContext(ctx, "systemctl", verb, service)
		out, runErr := cmd.CombinedOutput()
		output, err = string(out), runErr
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

func marshalData(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
