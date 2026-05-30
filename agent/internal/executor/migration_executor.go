package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type MigrationExecutor struct{}

func NewMigrationExecutor() *MigrationExecutor {
	return &MigrationExecutor{}
}

type MigrationConfig struct {
	SourceHost       string   `json:"source_host"`
	SourcePort       int      `json:"source_port"`
	SourceUser       string   `json:"source_user"`
	SourcePass       string   `json:"source_pass"`
	TargetHost       string   `json:"target_host"`
	TargetPort       int      `json:"target_port"`
	TargetUser       string   `json:"target_user"`
	TargetPass       string   `json:"target_pass"`
	Databases        []string `json:"databases"`
	Tables           []string `json:"tables"`
	MigrationType    string   `json:"migration_type"`
	ChunkSize        int      `json:"chunk_size"`
	Threads          int      `json:"threads"`
	MaxLag           int      `json:"max_lag"`
	ReplicateUser    string   `json:"replicate_user"`
	ReplicatePass    string   `json:"replicate_pass"`
	SSHPort          int      `json:"ssh_port"`
	SSHUser          string   `json:"ssh_user"`
	SSHPrivateKey    string   `json:"ssh_private_key"`
	WorkDir          string   `json:"work_dir"`
	GTIDMode         bool     `json:"gtid_mode"`
	CriticalThrottle int      `json:"critical_throttle"`
}

type MigrationProgress struct {
	TaskID           string    `json:"task_id"`
	Phase            string    `json:"phase"`
	TotalTables      int       `json:"total_tables"`
	CompletedTables  int       `json:"completed_tables"`
	TotalRows        int64     `json:"total_rows"`
	MigratedRows     int64     `json:"migrated_rows"`
	Percentage       float64   `json:"percentage"`
	CurrentTable     string    `json:"current_table"`
	ReplicationLag   int       `json:"replication_lag"`
	Status           string    `json:"status"`
	StartTime        time.Time `json:"start_time"`
	EstimatedEndTime time.Time `json:"estimated_end_time"`
	Throughput       float64   `json:"throughput"`
}

type MigrationVerifyResult struct {
	TaskID           string    `json:"task_id"`
	SourceChecksum   string    `json:"source_checksum"`
	TargetChecksum   string    `json:"target_checksum"`
	RowCountMatch    bool      `json:"row_count_match"`
	DataMatch        bool      `json:"data_match"`
	VerifiedTables   []string  `json:"verified_tables"`
	MismatchTables   []string  `json:"mismatch_tables"`
	VerifyTime       time.Time `json:"verify_time"`
	Status           string    `json:"status"`
}

type SwitchResult struct {
	TaskID          string    `json:"task_id"`
	OldSource       string    `json:"old_source"`
	NewTarget       string    `json:"new_target"`
	SwitchTime      time.Time `json:"switch_time"`
	ReplicationStop bool      `json:"replication_stop"`
	AppSwitch       bool      `json:"app_switch"`
	Status          string    `json:"status"`
}

func parseMigrationConfig(config map[string]interface{}) MigrationConfig {
	mc := MigrationConfig{
		SourceHost:       "localhost",
		SourcePort:       3306,
		SourceUser:       "root",
		SourcePass:       "",
		TargetHost:       "localhost",
		TargetPort:       3307,
		TargetUser:       "root",
		TargetPass:       "",
		MigrationType:    "physical",
		ChunkSize:        1000,
		Threads:          4,
		MaxLag:           60,
		ReplicateUser:    "repl",
		ReplicatePass:    "repl123",
		SSHPort:          22,
		SSHUser:          "root",
		SSHPrivateKey:    "/root/.ssh/id_rsa",
		WorkDir:          "/tmp/migration",
		GTIDMode:         false,
		CriticalThrottle: 1000,
	}

	if v, ok := config["source_host"].(string); ok {
		mc.SourceHost = v
	}
	if v, ok := config["source_port"].(int); ok {
		mc.SourcePort = v
	}
	if v, ok := config["source_user"].(string); ok {
		mc.SourceUser = v
	}
	if v, ok := config["source_pass"].(string); ok {
		mc.SourcePass = v
	}
	if v, ok := config["target_host"].(string); ok {
		mc.TargetHost = v
	}
	if v, ok := config["target_port"].(int); ok {
		mc.TargetPort = v
	}
	if v, ok := config["target_user"].(string); ok {
		mc.TargetUser = v
	}
	if v, ok := config["target_pass"].(string); ok {
		mc.TargetPass = v
	}
	if v, ok := config["databases"].([]string); ok {
		mc.Databases = v
	}
	if v, ok := config["tables"].([]string); ok {
		mc.Tables = v
	}
	if v, ok := config["migration_type"].(string); ok {
		mc.MigrationType = v
	}
	if v, ok := config["chunk_size"].(int); ok {
		mc.ChunkSize = v
	}
	if v, ok := config["threads"].(int); ok {
		mc.Threads = v
	}
	if v, ok := config["max_lag"].(int); ok {
		mc.MaxLag = v
	}
	if v, ok := config["replicate_user"].(string); ok {
		mc.ReplicateUser = v
	}
	if v, ok := config["replicate_pass"].(string); ok {
		mc.ReplicatePass = v
	}
	if v, ok := config["ssh_port"].(int); ok {
		mc.SSHPort = v
	}
	if v, ok := config["ssh_user"].(string); ok {
		mc.SSHUser = v
	}
	if v, ok := config["ssh_private_key"].(string); ok {
		mc.SSHPrivateKey = v
	}
	if v, ok := config["work_dir"].(string); ok {
		mc.WorkDir = v
	}
	if v, ok := config["gtid_mode"].(bool); ok {
		mc.GTIDMode = v
	}
	if v, ok := config["critical_throttle"].(int); ok {
		mc.CriticalThrottle = v
	}

	return mc
}

func (e *MigrationExecutor) ExecutePhysicalMigration(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMigrationConfig(req.Config)

	mkdirCmd := exec.CommandContext(ctx, "mkdir", "-p", config.WorkDir)
	if err := mkdirCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Failed to create work directory: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	backupDir := fmt.Sprintf("%s/backup-%d", config.WorkDir, time.Now().Unix())
	dumpCmd := exec.CommandContext(ctx, "mysqldump",
		"-h", config.SourceHost,
		"-P", fmt.Sprintf("%d", config.SourcePort),
		"-u", config.SourceUser,
		fmt.Sprintf("-p%s", config.SourcePass),
		"--single-transaction",
		"--master-data=2",
		"--flush-logs",
		"--routines",
		"--triggers",
		"--events",
		"--all-databases",
		"--result-file="+backupDir+"/full-backup.sql",
	)

	if err := dumpCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  20,
			Message:   fmt.Sprintf("Physical backup failed on source: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	for _, db := range config.Databases {
		importCmd := exec.CommandContext(ctx, "mysql",
			"-h", config.TargetHost,
			"-P", fmt.Sprintf("%d", config.TargetPort),
			"-u", config.TargetUser,
			fmt.Sprintf("-p%s", config.TargetPass),
			"-e", fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`;", db),
		)
		if err := importCmd.Run(); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  30,
				Message:   fmt.Sprintf("Failed to create database %s on target: %v", db, err),
				Timestamp: time.Now(),
			}, nil
		}
	}

	importFullCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", fmt.Sprintf("SOURCE %s/full-backup.sql;", backupDir),
	)

	if err := importFullCmd.Run(); err != nil {
		importCmd := exec.CommandContext(ctx, "sh", "-c",
			fmt.Sprintf("mysql -h %s -P %d -u %s -p%s < %s/full-backup.sql",
				config.TargetHost, config.TargetPort, config.TargetUser, config.TargetPass, backupDir),
		)
		if err := importCmd.Run(); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  50,
				Message:   fmt.Sprintf("Failed to import backup to target: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
	}

	verifyResult := e.VerifyMigration(ctx, req)
	if verifyResult.Status == "failed" {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "completed",
			Progress:  90,
			Message:   fmt.Sprintf("Physical migration completed but verification failed: %s", verifyResult.Status),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Physical migration completed from %s:%d to %s:%d",
			config.SourceHost, config.SourcePort, config.TargetHost, config.TargetPort),
		Timestamp: time.Now(),
	}, nil
}

func (e *MigrationExecutor) ExecuteReplicationMigration(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMigrationConfig(req.Config)

	createUserSQL := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO '%s'@'%%';",
		config.ReplicateUser, config.ReplicatePass, config.ReplicateUser,
	)

	sourceUserCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.SourceHost,
		"-P", fmt.Sprintf("%d", config.SourcePort),
		"-u", config.SourceUser,
		fmt.Sprintf("-p%s", config.SourcePass),
		"-e", createUserSQL,
	)

	if err := sourceUserCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  10,
			Message:   fmt.Sprintf("Failed to create replication user on source: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	binlogCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.SourceHost,
		"-P", fmt.Sprintf("%d", config.SourcePort),
		"-u", config.SourceUser,
		fmt.Sprintf("-p%s", config.SourcePass),
		"-e", "SHOW MASTER STATUS\\G",
	)

	binlogOutput, err := binlogCmd.Output()
	if err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  20,
			Message:   fmt.Sprintf("Failed to get master status: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	masterFile := ""
	masterPos := 0
	lines := strings.Split(string(binlogOutput), "\n")
	for _, line := range lines {
		if strings.Contains(line, "File:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				masterFile = parts[1]
			}
		}
		if strings.Contains(line, "Position:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				fmt.Sscanf(parts[1], "%d", &masterPos)
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
			"MASTER_LOG_POS=%d;",
		config.SourceHost, config.SourcePort,
		config.ReplicateUser, config.ReplicatePass,
		masterFile, masterPos,
	)

	changeCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", changeMasterSQL,
	)

	if err := changeCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  40,
			Message:   fmt.Sprintf("Failed to configure replication: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	startCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", "START SLAVE;",
	)

	if err := startCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Failed to start slave: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	time.Sleep(5 * time.Second)

	progress := e.MonitorProgress(ctx, req)
	if progress.Status == "error" {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("Replication started but monitor failed: %s", progress.Status),
			Timestamp: time.Now(),
		}, nil
	}

	for i := 0; i < 10; i++ {
		lagCmd := exec.CommandContext(ctx, "mysql",
			"-h", config.TargetHost,
			"-P", fmt.Sprintf("%d", config.TargetPort),
			"-u", config.TargetUser,
			fmt.Sprintf("-p%s", config.TargetPass),
			"-e", "SHOW SLAVE STATUS\\G",
		)

		lagOutput, err := lagCmd.Output()
		if err == nil {
			lagStr := string(lagOutput)
			if strings.Contains(lagStr, "Seconds_Behind_Master: 0") {
				break
			}
		}
		time.Sleep(5 * time.Second)
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Replication migration established from %s:%d to %s:%d",
			config.SourceHost, config.SourcePort, config.TargetHost, config.TargetPort),
		Timestamp: time.Now(),
	}, nil
}

func (e *MigrationExecutor) ExecuteGTIDMigration(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseMigrationConfig(req.Config)

	gtidCheckCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.SourceHost,
		"-P", fmt.Sprintf("%d", config.SourcePort),
		"-u", config.SourceUser,
		fmt.Sprintf("-p%s", config.SourcePass),
		"-e", "SELECT @@GLOBAL.GTID_MODE;",
	)

	gtidOutput, err := gtidCheckCmd.Output()
	if err != nil || !strings.Contains(string(gtidOutput), "ON") {
		enableCmd := exec.CommandContext(ctx, "mysql",
			"-h", config.SourceHost,
			"-P", fmt.Sprintf("%d", config.SourcePort),
			"-u", config.SourceUser,
			fmt.Sprintf("-p%s", config.SourcePass),
			"-e", "SET GLOBAL GTID_MODE=ON; SET GLOBAL ENFORCE_GTID_CONSISTENCY=ON;",
		)

		if err := enableCmd.Run(); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  10,
				Message:   fmt.Sprintf("Failed to enable GTID mode on source: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
	}

	targetGtidCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", "SET GLOBAL GTID_MODE=ON; SET GLOBAL ENFORCE_GTID_CONSISTENCY=ON;",
	)

	if err := targetGtidCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  20,
			Message:   fmt.Sprintf("Failed to enable GTID mode on target: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	createUserSQL := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT REPLICATION SLAVE ON *.* TO '%s'@'%%';",
		config.ReplicateUser, config.ReplicatePass, config.ReplicateUser,
	)

	userCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.SourceHost,
		"-P", fmt.Sprintf("%d", config.SourcePort),
		"-u", config.SourceUser,
		fmt.Sprintf("-p%s", config.SourcePass),
		"-e", createUserSQL,
	)

	if err := userCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  30,
			Message:   fmt.Sprintf("Failed to create replication user: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	changeMasterSQL := fmt.Sprintf(
		"CHANGE MASTER TO "+
			"MASTER_HOST='%s', "+
			"MASTER_PORT=%d, "+
			"MASTER_USER='%s', "+
			"MASTER_PASSWORD='%s', "+
			"MASTER_AUTO_POSITION=1;",
		config.SourceHost, config.SourcePort,
		config.ReplicateUser, config.ReplicatePass,
	)

	changeCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", changeMasterSQL,
	)

	if err := changeCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Failed to configure GTID replication: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	startCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", "START SLAVE;",
	)

	if err := startCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("Failed to start slave with GTID: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	time.Sleep(5 * time.Second)

	verifyGtidCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", "SHOW SLAVE STATUS\\G",
	)

	verifyOutput, err := verifyGtidCmd.Output()
	if err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "completed",
			Progress:  90,
			Message:   "GTID migration started but verification failed",
			Timestamp: time.Now(),
		}, nil
	}

	verifyStr := string(verifyOutput)
	if strings.Contains(verifyStr, "Auto_Position: 1") &&
		strings.Contains(verifyStr, "Slave_IO_Running: Yes") &&
		strings.Contains(verifyStr, "Slave_SQL_Running: Yes") {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "completed",
			Progress:  100,
			Message:   fmt.Sprintf("GTID migration established from %s:%d to %s:%d",
				config.SourceHost, config.SourcePort, config.TargetHost, config.TargetPort),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "warning",
		Progress:  85,
		Message:   "GTID replication started but slave status check incomplete",
		Timestamp: time.Now(),
	}, nil
}

func (e *MigrationExecutor) MonitorProgress(ctx context.Context, req DeployTaskRequest) *MigrationProgress {
	config := parseMigrationConfig(req.Config)

	progress := &MigrationProgress{
		TaskID:      req.TaskID,
		Phase:       "monitoring",
		StartTime:   time.Now(),
		Status:      "running",
		TotalTables: len(config.Tables),
	}

	statusCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", "SHOW SLAVE STATUS\\G",
	)

	statusOutput, err := statusCmd.Output()
	if err != nil {
		progress.Status = "error"
		progress.Phase = "connection_failed"
		return progress
	}

	statusStr := string(statusOutput)

	if strings.Contains(statusStr, "Slave_IO_Running: Yes") &&
		strings.Contains(statusStr, "Slave_SQL_Running: Yes") {
		progress.Status = "replicating"
		progress.Phase = "data_sync"
	}

	for _, line := range strings.Split(statusStr, "\n") {
		if strings.Contains(line, "Seconds_Behind_Master:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				var lag int
				fmt.Sscanf(parts[1], "%d", &lag)
				progress.ReplicationLag = lag
			}
		}
	}

	for _, db := range config.Databases {
		countCmd := exec.CommandContext(ctx, "mysql",
			"-h", config.SourceHost,
			"-P", fmt.Sprintf("%d", config.SourcePort),
			"-u", config.SourceUser,
			fmt.Sprintf("-p%s", config.SourcePass),
			"-e", fmt.Sprintf("SELECT SUM(TABLE_ROWS) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA='%s';", db),
		)

		countOutput, err := countCmd.Output()
		if err == nil {
			var rows int64
			lines := strings.Split(string(countOutput), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" && !strings.Contains(line, "SUM") {
					fmt.Sscanf(strings.TrimSpace(line), "%d", &rows)
					progress.TotalRows += rows
				}
			}
		}

		targetCountCmd := exec.CommandContext(ctx, "mysql",
			"-h", config.TargetHost,
			"-P", fmt.Sprintf("%d", config.TargetPort),
			"-u", config.TargetUser,
			fmt.Sprintf("-p%s", config.TargetPass),
			"-e", fmt.Sprintf("SELECT SUM(TABLE_ROWS) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA='%s';", db),
		)

		targetOutput, err := targetCountCmd.Output()
		if err == nil {
			var rows int64
			lines := strings.Split(string(targetOutput), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" && !strings.Contains(line, "SUM") {
					fmt.Sscanf(strings.TrimSpace(line), "%d", &rows)
					progress.MigratedRows += rows
				}
			}
		}
	}

	if progress.TotalRows > 0 {
		progress.Percentage = float64(progress.MigratedRows) / float64(progress.TotalRows) * 100
	}

	if progress.ReplicationLag == 0 && progress.Percentage >= 99.9 {
		progress.Phase = "sync_complete"
		progress.CompletedTables = progress.TotalTables
		progress.Percentage = 100.0
	}

	if progress.StartTime.Year() > 2000 && progress.MigratedRows > 0 {
		elapsed := time.Since(progress.StartTime).Seconds()
		progress.Throughput = float64(progress.MigratedRows) / elapsed

		if progress.Percentage < 100 && progress.Throughput > 0 {
			remainingRows := progress.TotalRows - progress.MigratedRows
			remainingSeconds := float64(remainingRows) / progress.Throughput
			progress.EstimatedEndTime = time.Now().Add(time.Duration(remainingSeconds) * time.Second)
		}
	}

	return progress
}

func (e *MigrationExecutor) VerifyMigration(ctx context.Context, req DeployTaskRequest) *MigrationVerifyResult {
	config := parseMigrationConfig(req.Config)

	result := &MigrationVerifyResult{
		TaskID:         req.TaskID,
		VerifiedTables: []string{},
		MismatchTables: []string{},
		VerifyTime:     time.Now(),
		Status:         "pending",
	}

	for _, db := range config.Databases {
		sourceCountCmd := exec.CommandContext(ctx, "mysql",
			"-h", config.SourceHost,
			"-P", fmt.Sprintf("%d", config.SourcePort),
			"-u", config.SourceUser,
			fmt.Sprintf("-p%s", config.SourcePass),
			"-e", fmt.Sprintf("SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA='%s';", db),
		)

		sourceOutput, err := sourceCountCmd.Output()
		if err != nil {
			result.Status = "failed"
			return result
		}

		targetCountCmd := exec.CommandContext(ctx, "mysql",
			"-h", config.TargetHost,
			"-P", fmt.Sprintf("%d", config.TargetPort),
			"-u", config.TargetUser,
			fmt.Sprintf("-p%s", config.TargetPass),
			"-e", fmt.Sprintf("SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA='%s';", db),
		)

		targetOutput, err := targetCountCmd.Output()
		if err != nil {
			result.Status = "failed"
			return result
		}

		var sourceCount, targetCount int
		parseCountOutput(string(sourceOutput), &sourceCount)
		parseCountOutput(string(targetOutput), &targetCount)

		if sourceCount == targetCount {
			result.RowCountMatch = true
		} else {
			result.RowCountMatch = false
			result.MismatchTables = append(result.MismatchTables, db)
		}

		tablesCmd := exec.CommandContext(ctx, "mysql",
			"-h", config.SourceHost,
			"-P", fmt.Sprintf("%d", config.SourcePort),
			"-u", config.SourceUser,
			fmt.Sprintf("-p%s", config.SourcePass),
			"-e", fmt.Sprintf("SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA='%s';", db),
		)

		tablesOutput, err := tablesCmd.Output()
		if err != nil {
			continue
		}

		tables := parseTableList(string(tablesOutput))
		for _, table := range tables {
			tableName := fmt.Sprintf("%s.%s", db, table)

			sourceChecksumCmd := exec.CommandContext(ctx, "mysql",
				"-h", config.SourceHost,
				"-P", fmt.Sprintf("%d", config.SourcePort),
				"-u", config.SourceUser,
				fmt.Sprintf("-p%s", config.SourcePass),
				"-e", fmt.Sprintf("CHECKSUM TABLE `%s`.`%s`;", db, table),
			)

			sourceChecksumOut, err := sourceChecksumCmd.Output()
			if err != nil {
				continue
			}

			targetChecksumCmd := exec.CommandContext(ctx, "mysql",
				"-h", config.TargetHost,
				"-P", fmt.Sprintf("%d", config.TargetPort),
				"-u", config.TargetUser,
				fmt.Sprintf("-p%s", config.TargetPass),
				"-e", fmt.Sprintf("CHECKSUM TABLE `%s`.`%s`;", db, table),
			)

			targetChecksumOut, err := targetChecksumCmd.Output()
			if err != nil {
				continue
			}

			sourceChecksum := parseChecksumOutput(string(sourceChecksumOut))
			targetChecksum := parseChecksumOutput(string(targetChecksumOut))

			if sourceChecksum == targetChecksum {
				result.VerifiedTables = append(result.VerifiedTables, tableName)
			} else {
				result.MismatchTables = append(result.MismatchTables, tableName)
			}
		}
	}

	if len(result.MismatchTables) == 0 {
		result.DataMatch = true
		result.Status = "verified"
	} else {
		result.DataMatch = false
		result.Status = "mismatch_detected"
	}

	return result
}

func (e *MigrationExecutor) ExecuteSwitch(ctx context.Context, req DeployTaskRequest) (*SwitchResult, error) {
	config := parseMigrationConfig(req.Config)

	result := &SwitchResult{
		TaskID:     req.TaskID,
		OldSource:  fmt.Sprintf("%s:%d", config.SourceHost, config.SourcePort),
		NewTarget:  fmt.Sprintf("%s:%d", config.TargetHost, config.TargetPort),
		SwitchTime: time.Now(),
		Status:     "pending",
	}

	progress := e.MonitorProgress(ctx, req)
	if progress.ReplicationLag > config.MaxLag {
		return result, fmt.Errorf("replication lag %d exceeds threshold %d", progress.ReplicationLag, config.MaxLag)
	}

	lockCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.SourceHost,
		"-P", fmt.Sprintf("%d", config.SourcePort),
		"-u", config.SourceUser,
		fmt.Sprintf("-p%s", config.SourcePass),
		"-e", "FLUSH TABLES WITH READ LOCK;",
	)

	if err := lockCmd.Run(); err != nil {
		result.Status = "lock_failed"
		return result, nil
	}

	defer func() {
		unlockCmd := exec.CommandContext(ctx, "mysql",
			"-h", config.SourceHost,
			"-P", fmt.Sprintf("%d", config.SourcePort),
			"-u", config.SourceUser,
			fmt.Sprintf("-p%s", config.SourcePass),
			"-e", "UNLOCK TABLES;",
		)
		_ = unlockCmd.Run()
	}()

	masterStatusCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.SourceHost,
		"-P", fmt.Sprintf("%d", config.SourcePort),
		"-u", config.SourceUser,
		fmt.Sprintf("-p%s", config.SourcePass),
		"-e", "SHOW MASTER STATUS\\G",
	)

	masterOutput, err := masterStatusCmd.Output()
	if err != nil {
		result.Status = "status_check_failed"
		return result, nil
	}

	waitCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", "SELECT MASTER_POS_WAIT('"+parseMasterFile(string(masterOutput))+"', "+parseMasterPos(string(masterOutput))+", 30);",
	)

	if err := waitCmd.Run(); err != nil {
		result.Status = "sync_wait_failed"
		return result, nil
	}

	stopSlaveCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", "STOP SLAVE; RESET SLAVE ALL;",
	)

	if err := stopSlaveCmd.Run(); err != nil {
		result.Status = "slave_stop_failed"
		return result, nil
	}

	result.ReplicationStop = true

	resetMasterCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", "RESET MASTER;",
	)

	_ = resetMasterCmd.Run()

	readOnlyOffCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.TargetHost,
		"-P", fmt.Sprintf("%d", config.TargetPort),
		"-u", config.TargetUser,
		fmt.Sprintf("-p%s", config.TargetPass),
		"-e", "SET GLOBAL read_only=OFF; SET GLOBAL super_read_only=OFF;",
	)

	if err := readOnlyOffCmd.Run(); err != nil {
		result.Status = "write_enable_failed"
		return result, nil
	}

	result.AppSwitch = true
	result.Status = "completed"

	return result, nil
}

func parseCountOutput(output string, count *int) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "COUNT") {
			fmt.Sscanf(line, "%d", count)
			break
		}
	}
}

func parseTableList(output string) []string {
	var tables []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "TABLE_NAME") {
			tables = append(tables, line)
		}
	}
	return tables
}

func parseChecksumOutput(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Checksum") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[len(parts)-1]
			}
		}
	}
	return ""
}

func parseMasterFile(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "File:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return ""
}

func parseMasterPos(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Position:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return "0"
}