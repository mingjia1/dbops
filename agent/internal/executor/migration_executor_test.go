package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMigrationExecutor(t *testing.T) {
	executor := NewMigrationExecutor()
	assert.NotNil(t, executor)
}

func TestParseMigrationConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected MigrationConfig
	}{
		{
			name:   "default values",
			config: map[string]interface{}{},
			expected: MigrationConfig{
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
			},
		},
		{
			name: "custom values",
			config: map[string]interface{}{
				"source_host":       "192.168.1.100",
				"source_port":       3306,
				"source_user":       "admin",
				"source_pass":       "sourcepass",
				"target_host":       "192.168.1.200",
				"target_port":       3307,
				"target_user":       "admin",
				"target_pass":       "targetpass",
				"databases":         []string{"db1", "db2"},
				"tables":            []string{"table1", "table2"},
				"migration_type":    "replication",
				"chunk_size":        5000,
				"threads":           8,
				"max_lag":           30,
				"replicate_user":    "repl_user",
				"replicate_pass":    "repl_pass",
				"ssh_port":          2222,
				"ssh_user":          "mysql",
				"ssh_private_key":   "/home/mysql/.ssh/id_rsa",
				"work_dir":          "/data/migration",
				"gtid_mode":         true,
				"critical_throttle": 500,
			},
			expected: MigrationConfig{
				SourceHost:       "192.168.1.100",
				SourcePort:       3306,
				SourceUser:       "admin",
				SourcePass:       "sourcepass",
				TargetHost:       "192.168.1.200",
				TargetPort:       3307,
				TargetUser:       "admin",
				TargetPass:       "targetpass",
				Databases:        []string{"db1", "db2"},
				Tables:           []string{"table1", "table2"},
				MigrationType:    "replication",
				ChunkSize:        5000,
				Threads:          8,
				MaxLag:           30,
				ReplicateUser:    "repl_user",
				ReplicatePass:    "repl_pass",
				SSHPort:          2222,
				SSHUser:          "mysql",
				SSHPrivateKey:    "/home/mysql/.ssh/id_rsa",
				WorkDir:          "/data/migration",
				GTIDMode:         true,
				CriticalThrottle: 500,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMigrationConfig(tt.config)
			assert.Equal(t, tt.expected.SourceHost, result.SourceHost)
			assert.Equal(t, tt.expected.SourcePort, result.SourcePort)
			assert.Equal(t, tt.expected.SourceUser, result.SourceUser)
			assert.Equal(t, tt.expected.SourcePass, result.SourcePass)
			assert.Equal(t, tt.expected.TargetHost, result.TargetHost)
			assert.Equal(t, tt.expected.TargetPort, result.TargetPort)
			assert.Equal(t, tt.expected.TargetUser, result.TargetUser)
			assert.Equal(t, tt.expected.TargetPass, result.TargetPass)
			assert.Equal(t, tt.expected.Databases, result.Databases)
			assert.Equal(t, tt.expected.Tables, result.Tables)
			assert.Equal(t, tt.expected.MigrationType, result.MigrationType)
			assert.Equal(t, tt.expected.ChunkSize, result.ChunkSize)
			assert.Equal(t, tt.expected.Threads, result.Threads)
			assert.Equal(t, tt.expected.MaxLag, result.MaxLag)
			assert.Equal(t, tt.expected.ReplicateUser, result.ReplicateUser)
			assert.Equal(t, tt.expected.ReplicatePass, result.ReplicatePass)
			assert.Equal(t, tt.expected.SSHPort, result.SSHPort)
			assert.Equal(t, tt.expected.SSHUser, result.SSHUser)
			assert.Equal(t, tt.expected.SSHPrivateKey, result.SSHPrivateKey)
			assert.Equal(t, tt.expected.WorkDir, result.WorkDir)
			assert.Equal(t, tt.expected.GTIDMode, result.GTIDMode)
			assert.Equal(t, tt.expected.CriticalThrottle, result.CriticalThrottle)
		})
	}
}

func TestMigrationConfigStruct(t *testing.T) {
	config := MigrationConfig{
		SourceHost:       "192.168.1.100",
		SourcePort:       3306,
		SourceUser:       "root",
		SourcePass:       "password",
		TargetHost:       "192.168.1.200",
		TargetPort:       3307,
		TargetUser:       "root",
		TargetPass:       "password",
		Databases:        []string{"db1", "db2", "db3"},
		Tables:           []string{"users", "orders"},
		MigrationType:    "replication",
		ChunkSize:        1000,
		Threads:          4,
		MaxLag:           60,
		ReplicateUser:    "repl",
		ReplicatePass:    "repl123",
		SSHPort:          22,
		SSHUser:          "root",
		SSHPrivateKey:    "/root/.ssh/id_rsa",
		WorkDir:          "/tmp/migration",
		GTIDMode:         true,
		CriticalThrottle: 1000,
	}

	assert.Equal(t, "192.168.1.100", config.SourceHost)
	assert.Equal(t, 3306, config.SourcePort)
	assert.Equal(t, "root", config.SourceUser)
	assert.Equal(t, "password", config.SourcePass)
	assert.Equal(t, "192.168.1.200", config.TargetHost)
	assert.Equal(t, 3307, config.TargetPort)
	assert.Equal(t, "root", config.TargetUser)
	assert.Equal(t, "password", config.TargetPass)
	assert.Len(t, config.Databases, 3)
	assert.Len(t, config.Tables, 2)
	assert.Equal(t, "replication", config.MigrationType)
	assert.Equal(t, 1000, config.ChunkSize)
	assert.Equal(t, 4, config.Threads)
	assert.Equal(t, 60, config.MaxLag)
	assert.Equal(t, "repl", config.ReplicateUser)
	assert.Equal(t, "repl123", config.ReplicatePass)
	assert.Equal(t, 22, config.SSHPort)
	assert.Equal(t, "root", config.SSHUser)
	assert.Equal(t, "/root/.ssh/id_rsa", config.SSHPrivateKey)
	assert.Equal(t, "/tmp/migration", config.WorkDir)
	assert.True(t, config.GTIDMode)
	assert.Equal(t, 1000, config.CriticalThrottle)
}

func TestMigrationProgressStruct(t *testing.T) {
	progress := MigrationProgress{
		TaskID:           "migration-001",
		Phase:            "data_sync",
		TotalTables:      10,
		CompletedTables:  5,
		TotalRows:        1000000,
		MigratedRows:     500000,
		Percentage:       50.0,
		CurrentTable:     "db1.users",
		ReplicationLag:   10,
		Status:           "running",
		StartTime:        time.Now(),
		EstimatedEndTime: time.Now().Add(30 * time.Minute),
		Throughput:       10000.0,
	}

	assert.Equal(t, "migration-001", progress.TaskID)
	assert.Equal(t, "data_sync", progress.Phase)
	assert.Equal(t, 10, progress.TotalTables)
	assert.Equal(t, 5, progress.CompletedTables)
	assert.Equal(t, int64(1000000), progress.TotalRows)
	assert.Equal(t, int64(500000), progress.MigratedRows)
	assert.Equal(t, 50.0, progress.Percentage)
	assert.Equal(t, "db1.users", progress.CurrentTable)
	assert.Equal(t, 10, progress.ReplicationLag)
	assert.Equal(t, "running", progress.Status)
	assert.NotZero(t, progress.StartTime)
	assert.NotZero(t, progress.EstimatedEndTime)
	assert.Equal(t, 10000.0, progress.Throughput)
}

func TestMigrationVerifyResultStruct(t *testing.T) {
	result := MigrationVerifyResult{
		TaskID:         "verify-001",
		SourceChecksum: "abc123",
		TargetChecksum: "abc123",
		RowCountMatch:  true,
		DataMatch:      true,
		VerifiedTables: []string{"db1.users", "db1.orders"},
		MismatchTables: []string{},
		VerifyTime:     time.Now(),
		Status:         "verified",
	}

	assert.Equal(t, "verify-001", result.TaskID)
	assert.Equal(t, "abc123", result.SourceChecksum)
	assert.Equal(t, "abc123", result.TargetChecksum)
	assert.True(t, result.RowCountMatch)
	assert.True(t, result.DataMatch)
	assert.Len(t, result.VerifiedTables, 2)
	assert.Len(t, result.MismatchTables, 0)
	assert.Equal(t, "verified", result.Status)
}

func TestSwitchResultStruct(t *testing.T) {
	result := SwitchResult{
		TaskID:          "switch-001",
		OldSource:       "192.168.1.100:3306",
		NewTarget:       "192.168.1.200:3307",
		SwitchTime:      time.Now(),
		ReplicationStop: true,
		AppSwitch:       true,
		Status:          "completed",
	}

	assert.Equal(t, "switch-001", result.TaskID)
	assert.Equal(t, "192.168.1.100:3306", result.OldSource)
	assert.Equal(t, "192.168.1.200:3307", result.NewTarget)
	assert.NotZero(t, result.SwitchTime)
	assert.True(t, result.ReplicationStop)
	assert.True(t, result.AppSwitch)
	assert.Equal(t, "completed", result.Status)
}

func TestParseCountOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name:     "standard output",
			output:   "COUNT(*)\n42\n",
			expected: 42,
		},
		{
			name:     "single value",
			output:   "100\n",
			expected: 100,
		},
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
		{
			name:     "output with header only",
			output:   "COUNT(*)\n",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var count int
			parseCountOutput(tt.output, &count)
			assert.Equal(t, tt.expected, count)
		})
	}
}

func TestParseTableList(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedLen    int
		expectedFirst  string
	}{
		{
			name:          "standard output",
			output:        "TABLE_NAME\nusers\norders\nproducts\n",
			expectedLen:   3,
			expectedFirst: "users",
		},
		{
			name:          "empty output",
			output:        "",
			expectedLen:   0,
			expectedFirst: "",
		},
		{
			name:          "header only",
			output:        "TABLE_NAME\n",
			expectedLen:   0,
			expectedFirst: "",
		},
		{
			name:          "single table",
			output:        "TABLE_NAME\nusers\n",
			expectedLen:   1,
			expectedFirst: "users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTableList(tt.output)
			assert.Len(t, result, tt.expectedLen)
			if tt.expectedLen > 0 {
				assert.Equal(t, tt.expectedFirst, result[0])
			}
		})
	}
}

func TestParseChecksumOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "standard output with Checksum header",
			output:   "Table\tChecksum\ndb1.users\t12345678\n",
			expected: "Checksum",
		},
		{
			name:     "checksum line format",
			output:   "db1.users	12345678\n",
			expected: "",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "checksum field present",
			output:   "Table Checksum 12345678\n",
			expected: "12345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseChecksumOutput(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseMasterFile(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "standard output",
			output:   "File: mysql-bin.000001\nPosition: 1234\n",
			expected: "mysql-bin.000001",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "no file",
			output:   "Position: 1234\n",
			expected: "",
		},
		{
			name:     "file with spaces",
			output:   "       File: mysql-bin.000002\n",
			expected: "mysql-bin.000002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMasterFile(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseMasterPos(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "standard output",
			output:   "File: mysql-bin.000001\nPosition: 1234\n",
			expected: "1234",
		},
		{
			name:     "empty output",
			output:   "",
			expected: "0",
		},
		{
			name:     "no position",
			output:   "File: mysql-bin.000001\n",
			expected: "0",
		},
		{
			name:     "position with spaces",
			output:   "       Position: 5678\n",
			expected: "5678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMasterPos(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMonitorProgressEmptyConfig(t *testing.T) {
	executor := NewMigrationExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "monitor-test-001",
		Config: map[string]interface{}{},
	}

	progress := executor.MonitorProgress(ctx, req)
	assert.Equal(t, "monitor-test-001", progress.TaskID)
	assert.Equal(t, 0, progress.TotalTables)
	assert.NotEmpty(t, progress.Phase)
	assert.NotEmpty(t, progress.Status)
}

func TestMonitorProgressWithTables(t *testing.T) {
	executor := NewMigrationExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "monitor-test-002",
		Config: map[string]interface{}{
			"tables": []string{"table1", "table2", "table3"},
		},
	}

	progress := executor.MonitorProgress(ctx, req)
	assert.Equal(t, "monitor-test-002", progress.TaskID)
	assert.Equal(t, 3, progress.TotalTables)
}

func TestVerifyMigrationWithDatabases(t *testing.T) {
	executor := NewMigrationExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "verify-test-001",
		Config: map[string]interface{}{
			"databases": []string{"db1"},
		},
	}

	result := executor.VerifyMigration(ctx, req)
	assert.Equal(t, "verify-test-001", result.TaskID)
	assert.NotEmpty(t, result.Status)
	assert.NotNil(t, result.VerifiedTables)
	assert.NotNil(t, result.MismatchTables)
}

func TestExecuteSwitchLagThreshold(t *testing.T) {
	executor := NewMigrationExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "switch-test-001",
		Config: map[string]interface{}{
			"max_lag": 10,
		},
	}

	result, err := executor.ExecuteSwitch(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "switch-test-001", result.TaskID)
	assert.NotEmpty(t, result.Status)
}

func TestExecutePhysicalMigration(t *testing.T) {
	executor := NewMigrationExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "physical-test-001",
		Config: map[string]interface{}{
			"migration_type": "physical",
			"databases":      []string{"db1"},
		},
	}

	result, err := executor.ExecutePhysicalMigration(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "physical-test-001", result.TaskID)
}

func TestExecuteReplicationMigration(t *testing.T) {
	executor := NewMigrationExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "replication-test-001",
		Config: map[string]interface{}{
			"migration_type":  "replication",
			"replicate_user":  "repl",
			"replicate_pass":  "repl123",
		},
	}

	result, err := executor.ExecuteReplicationMigration(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "replication-test-001", result.TaskID)
}

func TestExecuteGTIDMigration(t *testing.T) {
	executor := NewMigrationExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "gtid-test-001",
		Config: map[string]interface{}{
			"migration_type":  "gtid",
			"replicate_user":  "repl",
			"replicate_pass":  "repl123",
			"gtid_mode":       true,
		},
	}

	result, err := executor.ExecuteGTIDMigration(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "gtid-test-001", result.TaskID)
}

func TestParseMigrationConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		checkFn  func(t *testing.T, result MigrationConfig)
	}{
		{
			name:   "nil databases",
			config: map[string]interface{}{
				"databases": nil,
			},
			checkFn: func(t *testing.T, result MigrationConfig) {
				assert.Nil(t, result.Databases)
			},
		},
		{
			name:   "nil tables",
			config: map[string]interface{}{
				"tables": nil,
			},
			checkFn: func(t *testing.T, result MigrationConfig) {
				assert.Nil(t, result.Tables)
			},
		},
		{
			name:   "empty databases",
			config: map[string]interface{}{
				"databases": []string{},
			},
			checkFn: func(t *testing.T, result MigrationConfig) {
				assert.Empty(t, result.Databases)
			},
		},
		{
			name:   "empty tables",
			config: map[string]interface{}{
				"tables": []string{},
			},
			checkFn: func(t *testing.T, result MigrationConfig) {
				assert.Empty(t, result.Tables)
			},
		},
		{
			name:   "invalid port type",
			config: map[string]interface{}{
				"source_port": "3306",
			},
			checkFn: func(t *testing.T, result MigrationConfig) {
				assert.Equal(t, 3306, result.SourcePort)
			},
		},
		{
			name:   "invalid chunk size type",
			config: map[string]interface{}{
				"chunk_size": "1000",
			},
			checkFn: func(t *testing.T, result MigrationConfig) {
				assert.Equal(t, 1000, result.ChunkSize)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMigrationConfig(tt.config)
			tt.checkFn(t, result)
		})
	}
}

func TestParseCountOutputEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name:     "negative count",
			output:   "-1\n",
			expected: -1,
		},
		{
			name:     "large count",
			output:   "1000000\n",
			expected: 1000000,
		},
		{
			name:     "count with spaces",
			output:   "  42  \n",
			expected: 42,
		},
		{
			name:     "multiline output",
			output:   "COUNT(*)\n100\n200\n",
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var count int
			parseCountOutput(tt.output, &count)
			assert.Equal(t, tt.expected, count)
		})
	}
}

func TestParseTableListEdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		expectedLen   int
	}{
		{
			name:         "whitespace only",
			output:       "   \n   \n",
			expectedLen:  0,
		},
		{
			name:         "tabs and spaces",
			output:       "TABLE_NAME\n\tusers\t\n\torders\t\n",
			expectedLen:  2,
		},
		{
			name:         "lowercase header",
			output:       "table_name\nusers\n",
			expectedLen:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTableList(tt.output)
			assert.Len(t, result, tt.expectedLen)
		})
	}
}

func TestParseMasterFileEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "file with no extension",
			output:   "File: binlog0001\n",
			expected: "binlog0001",
		},
		{
			name:     "multiple files in output",
			output:   "File: mysql-bin.000001\nFile: mysql-bin.000002\n",
			expected: "mysql-bin.000001",
		},
		{
			name:     "file with path",
			output:   "File: /var/log/mysql/mysql-bin.000001\n",
			expected: "/var/log/mysql/mysql-bin.000001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMasterFile(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseMasterPosEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "large position",
			output:   "Position: 999999999\n",
			expected: "999999999",
		},
		{
			name:     "position zero",
			output:   "Position: 0\n",
			expected: "0",
		},
		{
			name:     "multiple positions",
			output:   "Position: 100\nPosition: 200\n",
			expected: "100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMasterPos(tt.output)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMonitorProgressWithDatabases(t *testing.T) {
	executor := NewMigrationExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "monitor-db-test",
		Config: map[string]interface{}{
			"databases": []string{"db1", "db2"},
			"tables":    []string{"t1", "t2"},
		},
	}

	progress := executor.MonitorProgress(ctx, req)
	assert.Equal(t, "monitor-db-test", progress.TaskID)
	assert.Equal(t, 2, progress.TotalTables)
	assert.NotZero(t, progress.StartTime)
}

func TestVerifyMigrationEmptyDatabases(t *testing.T) {
	executor := NewMigrationExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "verify-empty-test",
		Config: map[string]interface{}{},
	}

	result := executor.VerifyMigration(ctx, req)
	assert.Equal(t, "verify-empty-test", result.TaskID)
	assert.Empty(t, result.VerifiedTables)
	assert.Empty(t, result.MismatchTables)
	assert.Equal(t, "verified", result.Status)
	assert.True(t, result.DataMatch)
}

func TestExecuteSwitchWithSourceTarget(t *testing.T) {
	executor := NewMigrationExecutor()
	ctx := context.Background()

	req := DeployTaskRequest{
		TaskID: "switch-host-test",
		Config: map[string]interface{}{
			"source_host": "192.168.1.100",
			"source_port": 3306,
			"target_host": "192.168.1.200",
			"target_port": 3307,
		},
	}

	result, err := executor.ExecuteSwitch(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, "switch-host-test", result.TaskID)
	assert.Equal(t, "192.168.1.100:3306", result.OldSource)
	assert.Equal(t, "192.168.1.200:3307", result.NewTarget)
}

func TestMigrationProgressCalculations(t *testing.T) {
	progress := MigrationProgress{
		TaskID:           "calc-test",
		Phase:            "data_sync",
		TotalTables:      10,
		CompletedTables:  5,
		TotalRows:        1000,
		MigratedRows:     500,
		StartTime:        time.Now().Add(-10 * time.Second),
		EstimatedEndTime: time.Now().Add(10 * time.Second),
	}

	assert.Equal(t, 50.0, float64(progress.MigratedRows)/float64(progress.TotalRows)*100)
	assert.Equal(t, 5, progress.CompletedTables)
	assert.Equal(t, 10, progress.TotalTables)
}

func TestMigrationVerifyResultStatus(t *testing.T) {
	tests := []struct {
		name           string
		mismatchTables []string
		expectedMatch  bool
	}{
		{
			name:           "no mismatches",
			mismatchTables: []string{},
			expectedMatch:  true,
		},
		{
			name:           "with mismatches",
			mismatchTables: []string{"db1.table1"},
			expectedMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MigrationVerifyResult{
				MismatchTables: tt.mismatchTables,
			}
			if len(tt.mismatchTables) == 0 {
				result.DataMatch = true
				result.Status = "verified"
			} else {
				result.DataMatch = false
				result.Status = "mismatch_detected"
			}
			assert.Equal(t, tt.expectedMatch, result.DataMatch)
		})
	}
}