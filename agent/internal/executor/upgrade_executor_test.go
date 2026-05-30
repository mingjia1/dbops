package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewUpgradeExecutor(t *testing.T) {
	executor := NewUpgradeExecutor()
	assert.NotNil(t, executor)
}

func TestParseUpgradeConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected UpgradeConfig
	}{
		{
			name:   "default values",
			config: map[string]interface{}{},
			expected: UpgradeConfig{
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
			},
		},
		{
			name: "custom values",
			config: map[string]interface{}{
				"current_version":    "5.7.30",
				"target_version":     "8.0.25",
				"upgrade_type":       "logical",
				"instance_host":      "192.168.1.100",
				"instance_port":      3307,
				"mysql_user":         "admin",
				"mysql_pass":         "password123",
				"data_dir":           "/data/mysql",
				"backup_dir":         "/backup",
				"upgrade_method":     "manual",
				"skip_compatibility": true,
				"skip_backup":        true,
				"ssh_user":           "mysql",
				"ssh_private_key":    "/home/mysql/.ssh/id_rsa",
				"logical_migrate_tool": "mysqldump",
				"binlog_sync_timeout": 600,
				"upgrade_timeout":    7200,
			},
			expected: UpgradeConfig{
				CurrentVersion:     "5.7.30",
				TargetVersion:      "8.0.25",
				UpgradeType:        "logical",
				InstanceHost:       "192.168.1.100",
				InstancePort:       3307,
				MySQLUser:          "admin",
				MySQLPass:          "password123",
				DataDir:            "/data/mysql",
				BackupDir:          "/backup",
				UpgradeMethod:      "manual",
				SkipCompatibility:  true,
				SkipBackup:         true,
				SSHUser:            "mysql",
				SSHPrivateKey:      "/home/mysql/.ssh/id_rsa",
				LogicalMigrateTool: "mysqldump",
				BinlogSyncTimeout:  600,
				UpgradeTimeout:     7200,
			},
		},
		{
			name: "rolling nodes",
			config: map[string]interface{}{
				"rolling_nodes": []interface{}{"node1", "node2", "node3"},
			},
			expected: UpgradeConfig{
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
				RollingNodes:       []string{"node1", "node2", "node3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseUpgradeConfig(tt.config)
			assert.Equal(t, tt.expected.CurrentVersion, result.CurrentVersion)
			assert.Equal(t, tt.expected.TargetVersion, result.TargetVersion)
			assert.Equal(t, tt.expected.UpgradeType, result.UpgradeType)
			assert.Equal(t, tt.expected.InstanceHost, result.InstanceHost)
			assert.Equal(t, tt.expected.InstancePort, result.InstancePort)
			assert.Equal(t, tt.expected.MySQLUser, result.MySQLUser)
			assert.Equal(t, tt.expected.MySQLPass, result.MySQLPass)
			assert.Equal(t, tt.expected.DataDir, result.DataDir)
			assert.Equal(t, tt.expected.BackupDir, result.BackupDir)
			assert.Equal(t, tt.expected.UpgradeMethod, result.UpgradeMethod)
			assert.Equal(t, tt.expected.SkipCompatibility, result.SkipCompatibility)
			assert.Equal(t, tt.expected.SkipBackup, result.SkipBackup)
			assert.Equal(t, tt.expected.SSHUser, result.SSHUser)
			assert.Equal(t, tt.expected.SSHPrivateKey, result.SSHPrivateKey)
			assert.Equal(t, tt.expected.LogicalMigrateTool, result.LogicalMigrateTool)
			assert.Equal(t, tt.expected.BinlogSyncTimeout, result.BinlogSyncTimeout)
			assert.Equal(t, tt.expected.UpgradeTimeout, result.UpgradeTimeout)
			assert.Equal(t, tt.expected.RollingNodes, result.RollingNodes)
		})
	}
}

func TestCalculateVersionGap(t *testing.T) {
	executor := NewUpgradeExecutor()

	tests := []struct {
		name     string
		current  string
		target   string
		expected string
	}{
		{
			name:     "major upgrade",
			current:  "5.7.30",
			target:   "8.0.25",
			expected: "major",
		},
		{
			name:     "minor upgrade",
			current:  "8.0.25",
			target:   "8.1.0",
			expected: "minor",
		},
		{
			name:     "patch upgrade",
			current:  "8.0.25",
			target:   "8.0.26",
			expected: "patch",
		},
		{
			name:     "same version",
			current:  "8.0.25",
			target:   "8.0.25",
			expected: "patch",
		},
		{
			name:     "empty current",
			current:  "",
			target:   "8.0.25",
			expected: "unknown",
		},
		{
			name:     "empty target",
			current:  "8.0.25",
			target:   "",
			expected: "unknown",
		},
		{
			name:     "both empty",
			current:  "",
			target:   "",
			expected: "unknown",
		},
		{
			name:     "major downgrade",
			current:  "8.0.25",
			target:   "5.7.30",
			expected: "major",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.calculateVersionGap(tt.current, tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateUpgradeSteps(t *testing.T) {
	executor := NewUpgradeExecutor()

	tests := []struct {
		name          string
		config        UpgradeConfig
		versionGap    string
		expectedSteps []string
	}{
		{
			name: "major upgrade with backup",
			config: UpgradeConfig{
				TargetVersion: "8.0.25",
				BackupDir:     "/backup",
				SkipBackup:    false,
			},
			versionGap:    "major",
			expectedSteps: []string{"backup", "logical_export", "install_new_version", "logical_import", "validation"},
		},
		{
			name: "minor upgrade with backup",
			config: UpgradeConfig{
				TargetVersion: "8.1.0",
				BackupDir:     "/backup",
				SkipBackup:    false,
			},
			versionGap:    "minor",
			expectedSteps: []string{"backup", "stop_mysql", "upgrade_package", "upgrade_schema", "start_mysql", "validation"},
		},
		{
			name: "patch upgrade with backup",
			config: UpgradeConfig{
				TargetVersion: "8.0.26",
				BackupDir:     "/backup",
				SkipBackup:    false,
			},
			versionGap:    "patch",
			expectedSteps: []string{"backup", "stop_mysql", "upgrade_package", "upgrade_schema", "start_mysql", "validation"},
		},
		{
			name: "major upgrade skip backup",
			config: UpgradeConfig{
				TargetVersion: "8.0.25",
				BackupDir:     "/backup",
				SkipBackup:    true,
			},
			versionGap:    "major",
			expectedSteps: []string{"logical_export", "install_new_version", "logical_import", "validation"},
		},
		{
			name: "minor upgrade skip backup",
			config: UpgradeConfig{
				TargetVersion: "8.1.0",
				BackupDir:     "/backup",
				SkipBackup:    true,
			},
			versionGap:    "minor",
			expectedSteps: []string{"stop_mysql", "upgrade_package", "upgrade_schema", "start_mysql", "validation"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := executor.generateUpgradeSteps(tt.config, tt.versionGap)
			assert.Len(t, steps, len(tt.expectedSteps))
			for i, expectedName := range tt.expectedSteps {
				assert.Equal(t, expectedName, steps[i].Name)
				assert.Equal(t, "pending", steps[i].Status)
				assert.GreaterOrEqual(t, steps[i].Order, 1)
			}
		})
	}
}

func TestExtractVersionFromOutput(t *testing.T) {
	executor := NewUpgradeExecutor()

	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "standard output",
			output:   "Variable_name\tValue\nversion\t8.0.25\n",
			expected: "8.0.25",
		},
		{
			name:     "output with multiple fields",
			output:   "version\t8.0.25-log\nversion_comment\tMySQL Community Server\n",
			expected: "8.0.25-log",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "no version field",
			output:   "Variable_name\tValue\nport\t3306\n",
			expected: "",
		},
		{
			name:     "version on second line",
			output:   "Variable_name\tValue\nversion\t5.7.30\n",
			expected: "5.7.30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.extractVersionFromOutput(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpgradeConfigStruct(t *testing.T) {
	config := UpgradeConfig{
		CurrentVersion:     "5.7.30",
		TargetVersion:      "8.0.25",
		UpgradeType:        "in-place",
		InstanceHost:       "192.168.1.100",
		InstancePort:       3306,
		MySQLUser:          "root",
		MySQLPass:          "password",
		DataDir:            "/var/lib/mysql",
		BackupDir:          "/backup",
		UpgradeMethod:      "auto",
		SkipCompatibility:  true,
		SkipBackup:         false,
		SSHUser:            "root",
		SSHPrivateKey:      "/root/.ssh/id_rsa",
		LogicalMigrateTool: "mydumper",
		BinlogSyncTimeout:  300,
		UpgradeTimeout:     3600,
		RollingNodes:       []string{"node1", "node2"},
	}

	assert.Equal(t, "5.7.30", config.CurrentVersion)
	assert.Equal(t, "8.0.25", config.TargetVersion)
	assert.Equal(t, "in-place", config.UpgradeType)
	assert.Equal(t, "192.168.1.100", config.InstanceHost)
	assert.Equal(t, 3306, config.InstancePort)
	assert.Equal(t, "root", config.MySQLUser)
	assert.Equal(t, "password", config.MySQLPass)
	assert.Equal(t, "/var/lib/mysql", config.DataDir)
	assert.Equal(t, "/backup", config.BackupDir)
	assert.Equal(t, "auto", config.UpgradeMethod)
	assert.True(t, config.SkipCompatibility)
	assert.False(t, config.SkipBackup)
	assert.Equal(t, "root", config.SSHUser)
	assert.Equal(t, "/root/.ssh/id_rsa", config.SSHPrivateKey)
	assert.Equal(t, "mydumper", config.LogicalMigrateTool)
	assert.Equal(t, 300, config.BinlogSyncTimeout)
	assert.Equal(t, 3600, config.UpgradeTimeout)
	assert.Len(t, config.RollingNodes, 2)
}

func TestUpgradePathStruct(t *testing.T) {
	path := UpgradePath{
		Steps: []UpgradeStep{
			{Order: 1, Name: "backup", Description: "Create backup", Status: "pending"},
			{Order: 2, Name: "upgrade", Description: "Upgrade package", Status: "pending"},
		},
		EstimatedTime:    30,
		RequiresRestart:  true,
		RequiresBackup:   true,
		CompatibilityOK:  true,
		Warnings:         []string{"Warning 1"},
		Recommendations:  []string{"Recommendation 1"},
	}

	assert.Len(t, path.Steps, 2)
	assert.Equal(t, 30, path.EstimatedTime)
	assert.True(t, path.RequiresRestart)
	assert.True(t, path.RequiresBackup)
	assert.True(t, path.CompatibilityOK)
	assert.Len(t, path.Warnings, 1)
	assert.Len(t, path.Recommendations, 1)
}

func TestUpgradeStepStruct(t *testing.T) {
	step := UpgradeStep{
		Order:       1,
		Name:        "backup",
		Description: "Create full backup",
		Status:      "pending",
	}

	assert.Equal(t, 1, step.Order)
	assert.Equal(t, "backup", step.Name)
	assert.Equal(t, "Create full backup", step.Description)
	assert.Equal(t, "pending", step.Status)
}

func TestCompatibilityReportStruct(t *testing.T) {
	report := CompatibilityReport{
		Compatible:           true,
		VersionGap:           "minor",
		SupportedUpgradePath: true,
		Warnings:             []string{"Warning 1", "Warning 2"},
		Errors:               []string{},
		RecommendedSteps:     []string{"Step 1", "Step 2"},
	}

	assert.True(t, report.Compatible)
	assert.Equal(t, "minor", report.VersionGap)
	assert.True(t, report.SupportedUpgradePath)
	assert.Len(t, report.Warnings, 2)
	assert.Len(t, report.Errors, 0)
	assert.Len(t, report.RecommendedSteps, 2)
}

func TestCheckCompatibilityInvalidVersion(t *testing.T) {
	executor := NewUpgradeExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "test-001",
		Config: map[string]interface{}{
			"current_version": "invalid",
			"target_version":  "also-invalid",
		},
	}

	report, err := executor.CheckCompatibility(ctx, req)
	assert.NoError(t, err)
	assert.False(t, report.Compatible)
	assert.Contains(t, report.Errors, "Invalid version format")
}

func TestCheckCompatibilityMajorUpgrade(t *testing.T) {
	executor := NewUpgradeExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "test-002",
		Config: map[string]interface{}{
			"current_version": "5.7.30",
			"target_version":  "8.0.25",
		},
	}

	report, err := executor.CheckCompatibility(ctx, req)
	assert.NoError(t, err)
	assert.False(t, report.Compatible)
	assert.Equal(t, "major", report.VersionGap)
	assert.False(t, report.SupportedUpgradePath)
	assert.Contains(t, report.Warnings, "Major version upgrade requires logical migration")
}

func TestCheckCompatibilityMinorUpgrade(t *testing.T) {
	executor := NewUpgradeExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "test-003",
		Config: map[string]interface{}{
			"current_version": "8.0.25",
			"target_version":  "8.1.0",
		},
	}

	report, err := executor.CheckCompatibility(ctx, req)
	assert.NoError(t, err)
	assert.True(t, report.Compatible)
	assert.Equal(t, "minor", report.VersionGap)
	assert.True(t, report.SupportedUpgradePath)
}

func TestCheckCompatibilityPatchUpgrade(t *testing.T) {
	executor := NewUpgradeExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "test-004",
		Config: map[string]interface{}{
			"current_version": "8.0.25",
			"target_version":  "8.0.26",
		},
	}

	report, err := executor.CheckCompatibility(ctx, req)
	assert.NoError(t, err)
	assert.True(t, report.Compatible)
	assert.Equal(t, "patch", report.VersionGap)
	assert.True(t, report.SupportedUpgradePath)
}

func TestDeployTaskRequestStruct(t *testing.T) {
	req := DeployTaskRequest{
		TaskID:     "task-123",
		InstanceID: "instance-456",
		Config: map[string]interface{}{
			"current_version": "5.7.30",
			"target_version":  "8.0.25",
		},
	}

	assert.Equal(t, "task-123", req.TaskID)
	assert.Equal(t, "instance-456", req.InstanceID)
	assert.NotNil(t, req.Config)
}

func TestTaskResultStruct(t *testing.T) {
	result := TaskResult{
		TaskID:    "task-123",
		Status:    "completed",
		Progress:  100,
		Message:   "Upgrade completed",
		Timestamp: time.Now(),
	}

	assert.Equal(t, "task-123", result.TaskID)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, 100, result.Progress)
	assert.Equal(t, "Upgrade completed", result.Message)
	assert.NotZero(t, result.Timestamp)
}

func TestExecuteRollingUpgradeNoNodes(t *testing.T) {
	executor := NewUpgradeExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "rolling-test-001",
		Config: map[string]interface{}{
			"current_version": "8.0.25",
			"target_version":  "8.0.26",
			"upgrade_type":    "rolling",
		},
	}

	result, err := executor.ExecuteRollingUpgrade(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "No rolling upgrade nodes specified")
}

func TestExecuteRollingUpgradeWithNodes(t *testing.T) {
	executor := NewUpgradeExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "rolling-test-002",
		Config: map[string]interface{}{
			"current_version":  "8.0.25",
			"target_version":   "8.0.26",
			"upgrade_type":     "rolling",
			"rolling_nodes":    []interface{}{"node1", "node2"},
			"ssh_user":         "root",
			"ssh_private_key":  "/root/.ssh/id_rsa",
		},
	}

	result, err := executor.ExecuteRollingUpgrade(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestPlanUpgradePath(t *testing.T) {
	executor := NewUpgradeExecutor()
	ctx := context.Background()

	tests := []struct {
		name            string
		config          map[string]interface{}
		expectedSteps   int
		expectCompatOK  bool
	}{
		{
			name: "major upgrade path",
			config: map[string]interface{}{
				"current_version": "5.7.30",
				"target_version":  "8.0.25",
				"backup_dir":      "/backup",
			},
			expectedSteps:  6,
			expectCompatOK: false,
		},
		{
			name: "minor upgrade path",
			config: map[string]interface{}{
				"current_version": "8.0.25",
				"target_version":  "8.1.0",
				"backup_dir":      "/backup",
			},
			expectedSteps:  7,
			expectCompatOK: true,
		},
		{
			name: "patch upgrade path",
			config: map[string]interface{}{
				"current_version": "8.0.25",
				"target_version":  "8.0.26",
				"backup_dir":      "/backup",
			},
			expectedSteps:  7,
			expectCompatOK: true,
		},
		{
			name: "skip backup upgrade path",
			config: map[string]interface{}{
				"current_version": "8.0.25",
				"target_version":  "8.0.26",
				"skip_backup":     true,
			},
			expectedSteps:  6,
			expectCompatOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := DeployTaskRequest{
				TaskID: "plan-test",
				Config: tt.config,
			}

			path, err := executor.PlanUpgradePath(ctx, req)
			assert.NoError(t, err)
			assert.NotNil(t, path)
			assert.GreaterOrEqual(t, len(path.Steps), 0)
			assert.Equal(t, tt.expectCompatOK, path.CompatibilityOK)
			assert.True(t, path.RequiresRestart)
			assert.True(t, path.RequiresBackup)
		})
	}
}

func TestRollbackUpgrade(t *testing.T) {
	executor := NewUpgradeExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "rollback-test",
		Config: map[string]interface{}{
			"current_version":  "5.7.30",
			"target_version":   "8.0.25",
			"instance_host":    "localhost",
			"instance_port":    3306,
			"mysql_user":       "root",
			"mysql_pass":       "password",
			"backup_dir":       "/backup",
		},
	}

	result, err := executor.RollbackUpgrade(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestCalculateVersionGapEdgeCases(t *testing.T) {
	executor := NewUpgradeExecutor()

	tests := []struct {
		name     string
		current  string
		target   string
		expected string
	}{
		{
			name:     "single part version",
			current:  "5",
			target:   "8",
			expected: "major",
		},
		{
			name:     "two part version",
			current:  "5.7",
			target:   "8.0",
			expected: "major",
		},
		{
			name:     "version with suffix",
			current:  "8.0.25-log",
			target:   "8.0.26-log",
			expected: "patch",
		},
		{
			name:     "different suffix",
			current:  "8.0.25",
			target:   "8.0.26-debug",
			expected: "patch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.calculateVersionGap(tt.current, tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractVersionFromOutputMoreCases(t *testing.T) {
	executor := NewUpgradeExecutor()

	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "tab separated format",
			output:   "version\t8.0.25\n",
			expected: "8.0.25",
		},
		{
			name:     "space separated format",
			output:   "version 8.0.25\n",
			expected: "8.0.25",
		},
		{
			name:     "multiple tabs",
			output:   "version\t\t8.0.25\n",
			expected: "8.0.25",
		},
		{
			name:     "version with extra info",
			output:   "version\t8.0.25-MySQL Community Server\n",
			expected: "8.0.25-MySQL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.extractVersionFromOutput(tt.output)
			assert.Contains(t, result, "8.0")
		})
	}
}

func TestGenerateUpgradeStepsEdgeCases(t *testing.T) {
	executor := NewUpgradeExecutor()

	tests := []struct {
		name        string
		config      UpgradeConfig
		versionGap  string
		minSteps    int
	}{
		{
			name: "empty target version",
			config: UpgradeConfig{
				TargetVersion: "",
				BackupDir:     "/backup",
				SkipBackup:    false,
			},
			versionGap: "patch",
			minSteps:   1,
		},
		{
			name: "unknown version gap",
			config: UpgradeConfig{
				TargetVersion: "8.0.25",
				BackupDir:     "/backup",
				SkipBackup:    false,
			},
			versionGap: "unknown",
			minSteps:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := executor.generateUpgradeSteps(tt.config, tt.versionGap)
			assert.GreaterOrEqual(t, len(steps), tt.minSteps)
		})
	}
}

func TestCheckCompatibilityEdgeCases(t *testing.T) {
	executor := NewUpgradeExecutor()
	ctx := context.Background()

	tests := []struct {
		name            string
		config          map[string]interface{}
		expectCompat    bool
		expectErrors    bool
	}{
		{
			name: "missing current version",
			config: map[string]interface{}{
				"target_version": "8.0.25",
			},
			expectCompat: false,
			expectErrors: true,
		},
		{
			name: "missing target version",
			config: map[string]interface{}{
				"current_version": "5.7.30",
			},
			expectCompat: false,
			expectErrors: true,
		},
		{
			name: "both missing",
			config: map[string]interface{}{},
			expectCompat: false,
			expectErrors: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := DeployTaskRequest{
				TaskID: "compat-test",
				Config: tt.config,
			}

			report, err := executor.CheckCompatibility(ctx, req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectCompat, report.Compatible)
			if tt.expectErrors {
				assert.GreaterOrEqual(t, len(report.Errors), 1)
			}
		})
	}
}