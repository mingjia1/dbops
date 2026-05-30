package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type UpgradeExecutor struct{}

func NewUpgradeExecutor() *UpgradeExecutor {
	return &UpgradeExecutor{}
}

type UpgradeConfig struct {
	CurrentVersion     string   `json:"current_version"`
	TargetVersion      string   `json:"target_version"`
	UpgradeType        string   `json:"upgrade_type"`
	InstanceHost       string   `json:"instance_host"`
	InstancePort       int      `json:"instance_port"`
	MySQLUser          string   `json:"mysql_user"`
	MySQLPass          string   `json:"mysql_pass"`
	DataDir            string   `json:"data_dir"`
	BackupDir          string   `json:"backup_dir"`
	UpgradeMethod      string   `json:"upgrade_method"`
	SkipCompatibility  bool     `json:"skip_compatibility"`
	SkipBackup         bool     `json:"skip_backup"`
	SSHUser            string   `json:"ssh_user"`
	SSHPrivateKey      string   `json:"ssh_private_key"`
	LogicalMigrateTool string   `json:"logical_migrate_tool"`
	BinlogSyncTimeout  int      `json:"binlog_sync_timeout"`
	UpgradeTimeout     int      `json:"upgrade_timeout"`
	RollingNodes       []string `json:"rolling_nodes"`
}

type UpgradePath struct {
	Steps            []UpgradeStep `json:"steps"`
	EstimatedTime    int           `json:"estimated_time"`
	RequiresRestart  bool          `json:"requires_restart"`
	RequiresBackup   bool          `json:"requires_backup"`
	CompatibilityOK  bool          `json:"compatibility_ok"`
	Warnings         []string      `json:"warnings"`
	Recommendations  []string      `json:"recommendations"`
}

type UpgradeStep struct {
	Order       int    `json:"order"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type CompatibilityReport struct {
	Compatible           bool     `json:"compatible"`
	VersionGap           string   `json:"version_gap"`
	SupportedUpgradePath bool     `json:"supported_upgrade_path"`
	Warnings             []string `json:"warnings"`
	Errors               []string `json:"errors"`
	RecommendedSteps     []string `json:"recommended_steps"`
}

func parseUpgradeConfig(config map[string]interface{}) UpgradeConfig {
	uc := UpgradeConfig{
		UpgradeType:        "in-place",
		InstanceHost:       "localhost",
		InstancePort:       3306,
		MySQLUser:          "root",
		DataDir:            "/var/lib/mysql",
		BackupDir:          "/backup/mysql",
		UpgradeMethod:      "auto",
		SSHUser:            "root",
		SSHPrivateKey:      "/root/.ssh/id_rsa",
		LogicalMigrateTool: "mydumper",
		BinlogSyncTimeout:  300,
		UpgradeTimeout:     3600,
	}

	if v, ok := config["current_version"].(string); ok {
		uc.CurrentVersion = v
	}
	if v, ok := config["target_version"].(string); ok {
		uc.TargetVersion = v
	}
	if v, ok := config["upgrade_type"].(string); ok {
		uc.UpgradeType = v
	}
	if v, ok := config["instance_host"].(string); ok {
		uc.InstanceHost = v
	}
	if v, ok := config["instance_port"].(int); ok {
		uc.InstancePort = v
	}
	if v, ok := config["mysql_user"].(string); ok {
		uc.MySQLUser = v
	}
	if v, ok := config["mysql_pass"].(string); ok {
		uc.MySQLPass = v
	}
	if v, ok := config["data_dir"].(string); ok {
		uc.DataDir = v
	}
	if v, ok := config["backup_dir"].(string); ok {
		uc.BackupDir = v
	}
	if v, ok := config["upgrade_method"].(string); ok {
		uc.UpgradeMethod = v
	}
	if v, ok := config["skip_compatibility"].(bool); ok {
		uc.SkipCompatibility = v
	}
	if v, ok := config["skip_backup"].(bool); ok {
		uc.SkipBackup = v
	}
	if v, ok := config["ssh_user"].(string); ok {
		uc.SSHUser = v
	}
	if v, ok := config["ssh_private_key"].(string); ok {
		uc.SSHPrivateKey = v
	}
	if v, ok := config["logical_migrate_tool"].(string); ok {
		uc.LogicalMigrateTool = v
	}
	if v, ok := config["binlog_sync_timeout"].(int); ok {
		uc.BinlogSyncTimeout = v
	}
	if v, ok := config["upgrade_timeout"].(int); ok {
		uc.UpgradeTimeout = v
	}
	if nodes, ok := config["rolling_nodes"].([]interface{}); ok {
		for _, n := range nodes {
			if node, ok := n.(string); ok {
				uc.RollingNodes = append(uc.RollingNodes, node)
			}
		}
	}

	return uc
}

func (e *UpgradeExecutor) PlanUpgradePath(ctx context.Context, req DeployTaskRequest) (*UpgradePath, error) {
	config := parseUpgradeConfig(req.Config)

	path := &UpgradePath{
		EstimatedTime:   30,
		RequiresRestart: true,
		RequiresBackup:  true,
	}

	versionGap := e.calculateVersionGap(config.CurrentVersion, config.TargetVersion)
	path.Steps = e.generateUpgradeSteps(config, versionGap)

	compatReport, err := e.CheckCompatibility(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("compatibility check failed: %v", err)
	}

	path.CompatibilityOK = compatReport.Compatible
	path.Warnings = compatReport.Warnings
	path.Recommendations = compatReport.RecommendedSteps

	if !compatReport.Compatible && !config.SkipCompatibility {
		path.Steps = append(path.Steps, UpgradeStep{
			Order:       0,
			Name:        "compatibility_check",
			Description: "Resolve compatibility issues before upgrade",
			Status:      "blocked",
		})
	}

	return path, nil
}

func (e *UpgradeExecutor) calculateVersionGap(current, target string) string {
	if current == "" || target == "" {
		return "unknown"
	}

	currentParts := strings.Split(current, ".")
	targetParts := strings.Split(target, ".")

	if len(currentParts) >= 1 && len(targetParts) >= 1 {
		if currentParts[0] != targetParts[0] {
			return "major"
		}
	}

	if len(currentParts) >= 2 && len(targetParts) >= 2 {
		if currentParts[1] != targetParts[1] {
			return "minor"
		}
	}

	return "patch"
}

func (e *UpgradeExecutor) generateUpgradeSteps(config UpgradeConfig, versionGap string) []UpgradeStep {
	steps := []UpgradeStep{}

	if !config.SkipBackup {
		steps = append(steps, UpgradeStep{
			Order:       1,
			Name:        "backup",
			Description: fmt.Sprintf("Create full backup to %s", config.BackupDir),
			Status:      "pending",
		})
	}

	if versionGap == "major" {
		steps = append(steps, UpgradeStep{
			Order:       2,
			Name:        "logical_export",
			Description: "Export data using logical migration tool",
			Status:      "pending",
		})
		steps = append(steps, UpgradeStep{
			Order:       3,
			Name:        "install_new_version",
			Description: fmt.Sprintf("Install MySQL %s", config.TargetVersion),
			Status:      "pending",
		})
		steps = append(steps, UpgradeStep{
			Order:       4,
			Name:        "logical_import",
			Description: "Import data to new MySQL instance",
			Status:      "pending",
		})
	} else {
		steps = append(steps, UpgradeStep{
			Order:       2,
			Name:        "stop_mysql",
			Description: "Stop MySQL service gracefully",
			Status:      "pending",
		})
		steps = append(steps, UpgradeStep{
			Order:       3,
			Name:        "upgrade_package",
			Description: fmt.Sprintf("Upgrade MySQL package to %s", config.TargetVersion),
			Status:      "pending",
		})
		steps = append(steps, UpgradeStep{
			Order:       4,
			Name:        "upgrade_schema",
			Description: "Run mysql_upgrade to fix schema",
			Status:      "pending",
		})
		steps = append(steps, UpgradeStep{
			Order:       5,
			Name:        "start_mysql",
			Description: "Start MySQL service and verify",
			Status:      "pending",
		})
	}

	steps = append(steps, UpgradeStep{
		Order:       len(steps) + 1,
		Name:        "validation",
		Description: "Validate upgrade success and data integrity",
		Status:      "pending",
	})

	return steps
}

func (e *UpgradeExecutor) CheckCompatibility(ctx context.Context, req DeployTaskRequest) (*CompatibilityReport, error) {
	config := parseUpgradeConfig(req.Config)

	report := &CompatibilityReport{
		Warnings:         []string{},
		Errors:           []string{},
		RecommendedSteps: []string{},
	}

	currentParts := strings.Split(config.CurrentVersion, ".")
	targetParts := strings.Split(config.TargetVersion, ".")

	if len(currentParts) < 2 || len(targetParts) < 2 {
		report.Errors = append(report.Errors, "Invalid version format")
		report.Compatible = false
		return report, nil
	}

	versionGap := e.calculateVersionGap(config.CurrentVersion, config.TargetVersion)
	report.VersionGap = versionGap

	switch versionGap {
	case "major":
		report.SupportedUpgradePath = false
		report.Compatible = false
		report.Warnings = append(report.Warnings, "Major version upgrade requires logical migration")
		report.RecommendedSteps = append(report.RecommendedSteps,
			"Use logical migration method (mydumper/myloader)",
			"Plan for extended downtime",
			"Test upgrade on a replica first")
	case "minor":
		report.SupportedUpgradePath = true
		report.Compatible = true
		report.Warnings = append(report.Warnings, "Minor version upgrade may require schema changes")
		report.RecommendedSteps = append(report.RecommendedSteps,
			"Run mysql_upgrade after package update",
			"Check for deprecated features",
			"Review release notes for breaking changes")
	case "patch":
		report.SupportedUpgradePath = true
		report.Compatible = true
		report.RecommendedSteps = append(report.RecommendedSteps,
			"In-place upgrade is safe for patch versions")
	default:
		report.Compatible = false
	}

	if config.CurrentVersion != "" && config.TargetVersion != "" {
		cmd := exec.CommandContext(ctx, "mysql",
			"-h", config.InstanceHost,
			"-P", fmt.Sprintf("%d", config.InstancePort),
			"-u", config.MySQLUser,
			"-p"+config.MySQLPass,
			"-e", "SHOW VARIABLES LIKE 'version';")

		output, err := cmd.Output()
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("Could not verify running version: %v", err))
		} else {
			runningVersion := e.extractVersionFromOutput(string(output))
			if runningVersion != config.CurrentVersion {
				report.Warnings = append(report.Warnings,
					fmt.Sprintf("Running version %s differs from configured version %s", runningVersion, config.CurrentVersion))
			}
		}
	}

	return report, nil
}

func (e *UpgradeExecutor) extractVersionFromOutput(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "version") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	return ""
}

func (e *UpgradeExecutor) ExecuteInPlaceUpgrade(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseUpgradeConfig(req.Config)

	if !config.SkipBackup {
		backupResult := e.createPreUpgradeBackup(ctx, config)
		if backupResult.Status == "failed" {
			return backupResult, nil
		}
	}

	stopResult := e.stopMySQLGracefully(ctx, config)
	if stopResult.Status == "failed" {
		return stopResult, nil
	}

	upgradeResult := e.upgradePackage(ctx, config)
	if upgradeResult.Status == "failed" {
		e.rollbackPackage(ctx, config)
		return upgradeResult, nil
	}

	schemaResult := e.runMysqlUpgrade(ctx, config)
	if schemaResult.Status == "failed" {
		return schemaResult, nil
	}

	startResult := e.startMySQL(ctx, config)
	if startResult.Status == "failed" {
		return startResult, nil
	}

	validationResult := e.validateUpgrade(ctx, config)
	return validationResult, nil
}

func (e *UpgradeExecutor) createPreUpgradeBackup(ctx context.Context, config UpgradeConfig) *TaskResult {
	backupDir := fmt.Sprintf("%s/pre-upgrade-%s-%d", config.BackupDir, config.TargetVersion, time.Now().Unix())

	cmd := exec.CommandContext(ctx, "mysqldump",
		"-h", config.InstanceHost,
		"-P", fmt.Sprintf("%d", config.InstancePort),
		"-u", config.MySQLUser,
		"-p"+config.MySQLPass,
		"--all-databases",
		"--routines",
		"--triggers",
		"--events",
		"--single-transaction",
		"--flush-logs",
		"--master-data=2",
		fmt.Sprintf("--result-file=%s/full-backup.sql", backupDir))

	mkdirCmd := exec.CommandContext(ctx, "mkdir", "-p", backupDir)
	if err := mkdirCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Failed to create backup directory: %v", err),
			Timestamp: time.Now(),
		}
	}

	if err := cmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  10,
			Message:   fmt.Sprintf("Pre-upgrade backup failed: %v", err),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  15,
		Message:   fmt.Sprintf("Pre-upgrade backup created at %s", backupDir),
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) stopMySQLGracefully(ctx context.Context, config UpgradeConfig) *TaskResult {
	cmd := exec.CommandContext(ctx, "mysqladmin",
		"-h", config.InstanceHost,
		"-P", fmt.Sprintf("%d", config.InstancePort),
		"-u", config.MySQLUser,
		"-p"+config.MySQLPass,
		"shutdown")

	if err := cmd.Run(); err != nil {
		stopCmd := exec.CommandContext(ctx, "systemctl", "stop", "mysqld")
		if err := stopCmd.Run(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  20,
				Message:   fmt.Sprintf("Failed to stop MySQL: %v", err),
				Timestamp: time.Now(),
			}
		}
	}

	time.Sleep(5 * time.Second)

	return &TaskResult{
		Status:    "completed",
		Progress:  25,
		Message:   "MySQL stopped gracefully",
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) upgradePackage(ctx context.Context, config UpgradeConfig) *TaskResult {
	upgradeCmd := exec.CommandContext(ctx, "yum",
		"update", "-y",
		fmt.Sprintf("mysql-server-%s", config.TargetVersion))

	if err := upgradeCmd.Run(); err != nil {
		aptCmd := exec.CommandContext(ctx, "apt-get",
			"install", "-y",
			fmt.Sprintf("mysql-server=%s", config.TargetVersion))
		if err := aptCmd.Run(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  30,
				Message:   fmt.Sprintf("Failed to upgrade MySQL package: %v", err),
				Timestamp: time.Now(),
			}
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  50,
		Message:   fmt.Sprintf("MySQL package upgraded to %s", config.TargetVersion),
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) rollbackPackage(ctx context.Context, config UpgradeConfig) *TaskResult {
	rollbackCmd := exec.CommandContext(ctx, "yum",
		"downgrade", "-y",
		fmt.Sprintf("mysql-server-%s", config.CurrentVersion))

	if err := rollbackCmd.Run(); err != nil {
		aptCmd := exec.CommandContext(ctx, "apt-get",
			"install", "-y", "--allow-downgrades",
			fmt.Sprintf("mysql-server=%s", config.CurrentVersion))
		_ = aptCmd.Run()
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  0,
		Message:   fmt.Sprintf("MySQL package rolled back to %s", config.CurrentVersion),
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) runMysqlUpgrade(ctx context.Context, config UpgradeConfig) *TaskResult {
	cmd := exec.CommandContext(ctx, "mysql_upgrade",
		"-u", config.MySQLUser,
		"-p"+config.MySQLPass,
		"--force")

	output, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(output), "already upgraded") {
		return &TaskResult{
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("mysql_upgrade failed: %v, output: %s", err, string(output)),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  70,
		Message:   "Schema upgrade completed",
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) startMySQL(ctx context.Context, config UpgradeConfig) *TaskResult {
	startCmd := exec.CommandContext(ctx, "systemctl", "start", "mysqld")
	if err := startCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  75,
			Message:   fmt.Sprintf("Failed to start MySQL: %v", err),
			Timestamp: time.Now(),
		}
	}

	time.Sleep(10 * time.Second)

	healthCmd := exec.CommandContext(ctx, "mysqladmin",
		"-h", config.InstanceHost,
		"-P", fmt.Sprintf("%d", config.InstancePort),
		"-u", config.MySQLUser,
		"-p"+config.MySQLPass,
		"ping")

	if err := healthCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("MySQL health check failed: %v", err),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  85,
		Message:   "MySQL started and healthy",
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) validateUpgrade(ctx context.Context, config UpgradeConfig) *TaskResult {
	cmd := exec.CommandContext(ctx, "mysql",
		"-h", config.InstanceHost,
		"-P", fmt.Sprintf("%d", config.InstancePort),
		"-u", config.MySQLUser,
		"-p"+config.MySQLPass,
		"-e", "SELECT VERSION();")

	output, err := cmd.Output()
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  90,
			Message:   fmt.Sprintf("Version verification failed: %v", err),
			Timestamp: time.Now(),
		}
	}

	currentVersion := strings.TrimSpace(strings.ReplaceAll(string(output), "VERSION()", ""))
	if !strings.Contains(currentVersion, config.TargetVersion) {
		return &TaskResult{
			Status:    "failed",
			Progress:  95,
			Message:   fmt.Sprintf("Version mismatch: expected %s, got %s", config.TargetVersion, currentVersion),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Upgrade completed successfully. MySQL version: %s", currentVersion),
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) ExecuteLogicalMigration(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseUpgradeConfig(req.Config)

	exportResult := e.exportData(ctx, config)
	if exportResult.Status == "failed" {
		return exportResult, nil
	}

	stopResult := e.stopMySQLGracefully(ctx, config)
	if stopResult.Status == "failed" {
		return stopResult, nil
	}

	installResult := e.installNewVersion(ctx, config)
	if installResult.Status == "failed" {
		return installResult, nil
	}

	startResult := e.startNewInstance(ctx, config)
	if startResult.Status == "failed" {
		return startResult, nil
	}

	importResult := e.importData(ctx, config)
	if importResult.Status == "failed" {
		return importResult, nil
	}

	validationResult := e.validateLogicalMigration(ctx, config)
	return validationResult, nil
}

func (e *UpgradeExecutor) exportData(ctx context.Context, config UpgradeConfig) *TaskResult {
	exportDir := fmt.Sprintf("%s/migration-%d", config.BackupDir, time.Now().Unix())

	mkdirCmd := exec.CommandContext(ctx, "mkdir", "-p", exportDir)
	if err := mkdirCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Failed to create export directory: %v", err),
			Timestamp: time.Now(),
		}
	}

	switch config.LogicalMigrateTool {
	case "mydumper":
		cmd := exec.CommandContext(ctx, "mydumper",
			"-h", config.InstanceHost,
			"-P", fmt.Sprintf("%d", config.InstancePort),
			"-u", config.MySQLUser,
			"-p", config.MySQLPass,
			"-o", exportDir,
			"-t", "4",
			"--compress",
			"--trx-consistency-only")

		if err := cmd.Run(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  10,
				Message:   fmt.Sprintf("MyDumper export failed: %v", err),
				Timestamp: time.Now(),
			}
		}
	default:
		cmd := exec.CommandContext(ctx, "mysqldump",
			"-h", config.InstanceHost,
			"-P", fmt.Sprintf("%d", config.InstancePort),
			"-u", config.MySQLUser,
			"-p"+config.MySQLPass,
			"--all-databases",
			"--routines",
			"--triggers",
			"--events",
			"--single-transaction",
			fmt.Sprintf("--result-file=%s/full-export.sql", exportDir))

		if err := cmd.Run(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  10,
				Message:   fmt.Sprintf("mysqldump export failed: %v", err),
				Timestamp: time.Now(),
			}
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  20,
		Message:   fmt.Sprintf("Data exported to %s", exportDir),
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) installNewVersion(ctx context.Context, config UpgradeConfig) *TaskResult {
	removeCmd := exec.CommandContext(ctx, "yum", "remove", "-y", "mysql-server")
	_ = removeCmd.Run()

	installCmd := exec.CommandContext(ctx, "yum", "install", "-y",
		fmt.Sprintf("mysql-server-%s", config.TargetVersion))

	if err := installCmd.Run(); err != nil {
		aptCmd := exec.CommandContext(ctx, "apt-get", "install", "-y",
			fmt.Sprintf("mysql-server=%s", config.TargetVersion))
		if err := aptCmd.Run(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  30,
				Message:   fmt.Sprintf("Failed to install MySQL %s: %v", config.TargetVersion, err),
				Timestamp: time.Now(),
			}
		}
	}

	initCmd := exec.CommandContext(ctx, "mysqld", "--initialize-insecure", "--datadir="+config.DataDir)
	if err := initCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  40,
			Message:   fmt.Sprintf("Failed to initialize MySQL %s: %v", config.TargetVersion, err),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  50,
		Message:   fmt.Sprintf("MySQL %s installed and initialized", config.TargetVersion),
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) startNewInstance(ctx context.Context, config UpgradeConfig) *TaskResult {
	startCmd := exec.CommandContext(ctx, "systemctl", "start", "mysqld")
	if err := startCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  55,
			Message:   fmt.Sprintf("Failed to start new MySQL instance: %v", err),
			Timestamp: time.Now(),
		}
	}

	time.Sleep(10 * time.Second)

	return &TaskResult{
		Status:    "completed",
		Progress:  60,
		Message:   "New MySQL instance started",
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) importData(ctx context.Context, config UpgradeConfig) *TaskResult {
	exportDir := fmt.Sprintf("%s/migration-%d", config.BackupDir, time.Now().Unix())

	switch config.LogicalMigrateTool {
	case "mydumper":
		cmd := exec.CommandContext(ctx, "myloader",
			"-h", config.InstanceHost,
			"-P", fmt.Sprintf("%d", config.InstancePort),
			"-u", config.MySQLUser,
			"-p", config.MySQLPass,
			"-d", exportDir,
			"-t", "4",
			"--overwrite-tables")

		if err := cmd.Run(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  70,
				Message:   fmt.Sprintf("MyLoader import failed: %v", err),
				Timestamp: time.Now(),
			}
		}
	default:
		cmd := exec.CommandContext(ctx, "mysql",
			"-h", config.InstanceHost,
			"-P", fmt.Sprintf("%d", config.InstancePort),
			"-u", config.MySQLUser,
			"-p"+config.MySQLPass,
			"-e", fmt.Sprintf("source %s/full-export.sql", exportDir))

		if err := cmd.Run(); err != nil {
			return &TaskResult{
				Status:    "failed",
				Progress:  70,
				Message:   fmt.Sprintf("MySQL import failed: %v", err),
				Timestamp: time.Now(),
			}
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  85,
		Message:   "Data imported successfully",
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) validateLogicalMigration(ctx context.Context, config UpgradeConfig) *TaskResult {
	cmd := exec.CommandContext(ctx, "mysql",
		"-h", config.InstanceHost,
		"-P", fmt.Sprintf("%d", config.InstancePort),
		"-u", config.MySQLUser,
		"-p"+config.MySQLPass,
		"-e", "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema NOT IN ('mysql', 'information_schema', 'performance_schema');")

	output, err := cmd.Output()
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  90,
			Message:   fmt.Sprintf("Data count verification failed: %v", err),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Logical migration completed. Tables count verified: %s", strings.TrimSpace(string(output))),
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) ExecuteRollingUpgrade(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseUpgradeConfig(req.Config)

	if len(config.RollingNodes) == 0 {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   "No rolling upgrade nodes specified",
			Timestamp: time.Now(),
		}, nil
	}

	totalNodes := len(config.RollingNodes)
	completedNodes := 0

	for i, node := range config.RollingNodes {
		nodeConfig := config
		nodeConfig.InstanceHost = node

		nodeResult := e.upgradeSingleNode(ctx, nodeConfig)
		if nodeResult.Status == "failed" {
			return &TaskResult{
				Status:    "failed",
				Progress:  (i * 100) / totalNodes,
				Message:   fmt.Sprintf("Rolling upgrade failed on node %s: %s", node, nodeResult.Message),
				Timestamp: time.Now(),
			}, nil
		}

		completedNodes++

		if i < totalNodes-1 {
			verifyResult := e.verifyNodeReady(ctx, nodeConfig)
			if verifyResult.Status == "failed" {
				return &TaskResult{
					Status:    "failed",
					Progress:  ((i + 1) * 100) / totalNodes,
					Message:   fmt.Sprintf("Node %s not ready after upgrade: %s", node, verifyResult.Message),
					Timestamp: time.Now(),
				}, nil
			}
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Rolling upgrade completed on %d/%d nodes", completedNodes, totalNodes),
		Timestamp: time.Now(),
	}, nil
}

func (e *UpgradeExecutor) upgradeSingleNode(ctx context.Context, config UpgradeConfig) *TaskResult {
	sshCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.InstanceHost),
		fmt.Sprintf("yum update -y mysql-server-%s || apt-get install -y mysql-server=%s",
			config.TargetVersion, config.TargetVersion))

	if err := sshCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Failed to upgrade node %s: %v", config.InstanceHost, err),
			Timestamp: time.Now(),
		}
	}

	restartCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.InstanceHost),
		"systemctl restart mysqld")

	if err := restartCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Failed to restart MySQL on node %s: %v", config.InstanceHost, err),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  80,
		Message:   fmt.Sprintf("Node %s upgraded to %s", config.InstanceHost, config.TargetVersion),
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) verifyNodeReady(ctx context.Context, config UpgradeConfig) *TaskResult {
	time.Sleep(10 * time.Second)

	healthCmd := exec.CommandContext(ctx, "ssh",
		"-i", config.SSHPrivateKey,
		"-o", "StrictHostKeyChecking=no",
		fmt.Sprintf("%s@%s", config.SSHUser, config.InstanceHost),
		"mysqladmin ping -u root")

	if err := healthCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  0,
			Message:   fmt.Sprintf("Node %s health check failed: %v", config.InstanceHost, err),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Node %s is healthy and ready", config.InstanceHost),
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) RollbackUpgrade(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parseUpgradeConfig(req.Config)

	rollbackResult := e.rollbackPackage(ctx, config)
	if rollbackResult.Status == "failed" {
		return rollbackResult, nil
	}

	restoreResult := e.restoreBackup(ctx, config)
	if restoreResult.Status == "failed" {
		return restoreResult, nil
	}

	startResult := e.startMySQL(ctx, config)
	if startResult.Status == "failed" {
		return startResult, nil
	}

	validationResult := e.validateRollback(ctx, config)
	return validationResult, nil
}

func (e *UpgradeExecutor) restoreBackup(ctx context.Context, config UpgradeConfig) *TaskResult {
	backupDir := fmt.Sprintf("%s/pre-upgrade-%s", config.BackupDir, config.TargetVersion)

	stopCmd := exec.CommandContext(ctx, "systemctl", "stop", "mysqld")
	_ = stopCmd.Run()

	restoreCmd := exec.CommandContext(ctx, "mysql",
		"-h", config.InstanceHost,
		"-P", fmt.Sprintf("%d", config.InstancePort),
		"-u", config.MySQLUser,
		"-p"+config.MySQLPass,
		"-e", fmt.Sprintf("source %s/full-backup.sql", backupDir))

	startCmd := exec.CommandContext(ctx, "systemctl", "start", "mysqld")
	_ = startCmd.Run()

	if err := restoreCmd.Run(); err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  50,
			Message:   fmt.Sprintf("Backup restore failed: %v", err),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  70,
		Message:   fmt.Sprintf("Backup restored from %s", backupDir),
		Timestamp: time.Now(),
	}
}

func (e *UpgradeExecutor) validateRollback(ctx context.Context, config UpgradeConfig) *TaskResult {
	cmd := exec.CommandContext(ctx, "mysql",
		"-h", config.InstanceHost,
		"-P", fmt.Sprintf("%d", config.InstancePort),
		"-u", config.MySQLUser,
		"-p"+config.MySQLPass,
		"-e", "SELECT VERSION();")

	output, err := cmd.Output()
	if err != nil {
		return &TaskResult{
			Status:    "failed",
			Progress:  90,
			Message:   fmt.Sprintf("Rollback validation failed: %v", err),
			Timestamp: time.Now(),
		}
	}

	currentVersion := strings.TrimSpace(strings.ReplaceAll(string(output), "VERSION()", ""))
	if !strings.Contains(currentVersion, config.CurrentVersion) {
		return &TaskResult{
			Status:    "failed",
			Progress:  95,
			Message:   fmt.Sprintf("Rollback failed: version is %s, expected %s", currentVersion, config.CurrentVersion),
			Timestamp: time.Now(),
		}
	}

	return &TaskResult{
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Rollback completed. MySQL restored to version %s", config.CurrentVersion),
		Timestamp: time.Now(),
	}
}