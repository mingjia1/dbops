package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- parseMasterSlaveConfig ---

func TestParseMasterSlaveConfig_Defaults(t *testing.T) {
	got := parseMasterSlaveConfig(map[string]interface{}{})
	assert.Equal(t, "localhost", got.MasterHost)
	assert.Equal(t, 3306, got.MasterPort)
	assert.Equal(t, "localhost", got.SlaveHost)
	assert.Equal(t, 3307, got.SlavePort)
	assert.Equal(t, "repl", got.ReplicateUser)
	assert.Equal(t, "", got.ReplicatePass)
	assert.Equal(t, "root", got.MySQLUser)
	assert.Equal(t, "", got.MySQLPass)
	assert.Equal(t, 1, got.ServerID)
	assert.Equal(t, "async", got.DeployMode)
}

func TestParseMasterSlaveConfig_CustomValues(t *testing.T) {
	got := parseMasterSlaveConfig(map[string]interface{}{
		"master_host":    "10.0.0.1",
		"master_port":    3306,
		"slave_host":     "10.0.0.2",
		"slave_port":     3307,
		"replicate_user": "myrepl",
		"replicate_pass": "myrepl!@#",
		"mysql_user":     "admin",
		"mysql_password": "admin!@#",
		"server_id":      42,
	})
	assert.Equal(t, "10.0.0.1", got.MasterHost)
	assert.Equal(t, 3306, got.MasterPort)
	assert.Equal(t, "10.0.0.2", got.SlaveHost)
	assert.Equal(t, 3307, got.SlavePort)
	assert.Equal(t, "myrepl", got.ReplicateUser)
	assert.Equal(t, "myrepl!@#", got.ReplicatePass)
	assert.Equal(t, "admin", got.MySQLUser)
	assert.Equal(t, "admin!@#", got.MySQLPass)
	assert.Equal(t, 42, got.ServerID)
}

func TestParseMasterSlaveConfig_MysqlPassFallback(t *testing.T) {
	got := parseMasterSlaveConfig(map[string]interface{}{
		"mysql_pass": "fallback-pass",
	})
	assert.Equal(t, "fallback-pass", got.MySQLPass)
}

func TestParseMasterSlaveConfig_MysqlPasswordPrecedence(t *testing.T) {
	got := parseMasterSlaveConfig(map[string]interface{}{
		"mysql_password": "primary-pass",
		"mysql_pass":     "fallback-pass",
	})
	assert.Equal(t, "primary-pass", got.MySQLPass)
}

func TestParseMasterSlaveConfig_WrongTypes(t *testing.T) {
	got := parseMasterSlaveConfig(map[string]interface{}{
		"master_port": "abc",
		"slave_port":  true,
		"server_id":   "def",
	})
	assert.Equal(t, 3306, got.MasterPort)
	assert.Equal(t, 3307, got.SlavePort)
	assert.Equal(t, 1, got.ServerID)
}

// --- MasterSlaveConfig struct ---

func TestMasterSlaveConfig_Fields(t *testing.T) {
	c := MasterSlaveConfig{
		MasterHost:    "10.0.0.1",
		MasterPort:    3306,
		SlaveHost:     "10.0.0.2",
		SlavePort:     3307,
		ReplicateUser: "repl",
		ReplicatePass: "repl123",
		MySQLUser:     "root",
		MySQLPass:     "rootpass",
		ServerID:      2,
		DeployMode:    "async",
	}
	assert.Equal(t, "10.0.0.1", c.MasterHost)
	assert.Equal(t, "root", c.MySQLUser)
	assert.Equal(t, "async", c.DeployMode)
}

// --- parseMasterStatus ---

func TestParseMasterStatus_Standard(t *testing.T) {
	output := "mysql-bin.000001\t12345\tmulti-master\t1-2-3\n"
	file, pos := parseMasterStatus(output)
	assert.Equal(t, "mysql-bin.000001", file)
	assert.Equal(t, "12345", pos)
}

func TestParseMasterStatus_Empty(t *testing.T) {
	file, pos := parseMasterStatus("")
	assert.Equal(t, "", file)
	assert.Equal(t, "", pos)
}

func TestParseMasterStatus_MultipleLines(t *testing.T) {
	output := "File\tPosition\tBinlog_Do_DB\tBinlog_Ignore_DB\nmysql-bin.000042\t67890\t\t\n"
	file, pos := parseMasterStatus(output)
	assert.Equal(t, "mysql-bin.000042", file)
	assert.Equal(t, "67890", pos)
}

func TestParseMasterStatus_NoFormatMatch(t *testing.T) {
	output := "No binary log files\n"
	file, pos := parseMasterStatus(output)
	assert.Equal(t, "", file)
	assert.Equal(t, "", pos)
}

// --- replicationThreadsRunning ---

func TestReplicationThreadsRunning_Modern(t *testing.T) {
	status := `Replica_IO_Running: Yes
Replica_SQL_Running: Yes
Last_Error: `
	assert.True(t, replicationThreadsRunning(status))
}

func TestReplicationThreadsRunning_Legacy(t *testing.T) {
	status := `Slave_IO_Running: Yes
Slave_SQL_Running: Yes`
	assert.True(t, replicationThreadsRunning(status))
}

func TestReplicationThreadsRunning_IONotRunning(t *testing.T) {
	status := `Slave_IO_Running: No
Slave_SQL_Running: Yes`
	assert.False(t, replicationThreadsRunning(status))
}

func TestReplicationThreadsRunning_SQLNotRunning(t *testing.T) {
	status := `Slave_IO_Running: Yes
Slave_SQL_Running: No`
	assert.False(t, replicationThreadsRunning(status))
}

func TestReplicationThreadsRunning_Empty(t *testing.T) {
	assert.False(t, replicationThreadsRunning(""))
}

func TestParseSlaveStatusG_ModernReplicaFields(t *testing.T) {
	output := `Replica_IO_Running: Yes
Replica_SQL_Running: Yes
Seconds_Behind_Source: 0
Source_Host: 10.1.81.21
Source_Port: 3306
Last_IO_Error:
Relay_Log_Space: 1024`

	parsed := parseSlaveStatusG(output)

	assert.Contains(t, parsed, "cluster_type=ha")
	assert.Contains(t, parsed, "role=slave")
	assert.Contains(t, parsed, "slave_io_running=Yes")
	assert.Contains(t, parsed, "slave_sql_running=Yes")
	assert.Contains(t, parsed, "seconds_behind_master=0")
	assert.Contains(t, parsed, "master_host=10.1.81.21")
	assert.Contains(t, parsed, "master_port=3306")
	assert.Contains(t, parsed, "relay_log_space=1024")
}

func TestParseSlaveStatusG_LegacySlaveFields(t *testing.T) {
	output := `Slave_IO_Running: Yes
Slave_SQL_Running: Yes
Seconds_Behind_Master: 0
Master_Host: 10.1.81.21
Master_Port: 3306`

	parsed := parseSlaveStatusG(output)

	assert.Contains(t, parsed, "slave_io_running=Yes")
	assert.Contains(t, parsed, "slave_sql_running=Yes")
	assert.Contains(t, parsed, "seconds_behind_master=0")
	assert.Contains(t, parsed, "master_host=10.1.81.21")
	assert.Contains(t, parsed, "master_port=3306")
}

// --- cleanMySQLOutput ---

func TestCleanMySQLOutput_RemovesWarning(t *testing.T) {
	output := "mysql: [Warning] Using a password on the command line...\n10.0.0.1"
	cleaned := cleanMySQLOutput(output)
	assert.Equal(t, "10.0.0.1", cleaned)
}

func TestCleanMySQLOutput_NoWarning(t *testing.T) {
	output := "8.0.36"
	cleaned := cleanMySQLOutput(output)
	assert.Equal(t, "8.0.36", cleaned)
}

func TestCleanMySQLOutput_MultipleLines(t *testing.T) {
	output := "mysql: [Warning] insecure\nline1\nline2\n"
	cleaned := cleanMySQLOutput(output)
	assert.Equal(t, "line1\nline2", cleaned)
}

func TestCleanMySQLOutput_Empty(t *testing.T) {
	assert.Equal(t, "", cleanMySQLOutput(""))
}

// --- resolveTargetConn ---

func TestResolveTargetConn_WithConfig(t *testing.T) {
	cfg := RoleSwitchConfig{TargetHost: "10.0.0.1", TargetPort: 3306, TargetUser: "admin", TargetPass: "pass"}
	host, port, user, pass := resolveTargetConn(cfg)
	assert.Equal(t, "10.0.0.1", host)
	assert.Equal(t, 3306, port)
	assert.Equal(t, "admin", user)
	assert.Equal(t, "pass", pass)
}

func TestResolveTargetConn_Defaults(t *testing.T) {
	cfg := RoleSwitchConfig{}
	host, port, user, pass := resolveTargetConn(cfg)
	assert.Equal(t, "127.0.0.1", host)
	assert.Equal(t, 3306, port)
	assert.Equal(t, "root", user)
	assert.Equal(t, "", pass)
}

// --- primaryRoleName ---

func TestPrimaryRoleName_MHA(t *testing.T) {
	assert.Equal(t, "master", primaryRoleName("mha"))
	assert.Equal(t, "master", primaryRoleName("ha"))
}

func TestPrimaryRoleName_MGRPXC(t *testing.T) {
	assert.Equal(t, "primary", primaryRoleName("mgr"))
	assert.Equal(t, "primary", primaryRoleName("pxc"))
	assert.Equal(t, "primary", primaryRoleName("unknown"))
}

// --- roleDemoteMessage ---

func TestRoleDemoteMessage_PXC(t *testing.T) {
	cfg := RoleSwitchConfig{InstanceID: "i-1", TargetRole: "secondary", ClusterType: "pxc"}
	msg := roleDemoteMessage(cfg)
	assert.Contains(t, msg, "PXC instance")
	assert.Contains(t, msg, "i-1")
}

func TestRoleDemoteMessage_NonPXC(t *testing.T) {
	cfg := RoleSwitchConfig{InstanceID: "i-1", TargetRole: "slave", ClusterType: "mha"}
	msg := roleDemoteMessage(cfg)
	assert.Contains(t, msg, "mha cluster")
}

// --- isDataDirInitialized (test without actual filesystem side effects) ---
// Note: This test checks that the function returns false for non-existent paths.

func TestIsDataDirInitialized_NonExistent(t *testing.T) {
	assert.False(t, isDataDirInitialized("/nonexistent/path"))
}

// --- config helpers (configInt, configString, configBool, configNodeList) ---
// These are already tested indirectly, but let's add explicit tests.

func TestConfigInt(t *testing.T) {
	assert.Equal(t, 3306, configInt(map[string]interface{}{"port": 3306}, "port"))
	assert.Equal(t, 0, configInt(map[string]interface{}{"port": "abc"}, "port"))
	assert.Equal(t, 0, configInt(map[string]interface{}{}, "port"))
}

func TestConfigString(t *testing.T) {
	assert.Equal(t, "hello", configString(map[string]interface{}{"key": "hello"}, "key"))
	assert.Equal(t, "", configString(map[string]interface{}{"key": 123}, "key"))
	assert.Equal(t, "", configString(map[string]interface{}{}, "key"))
}

func TestConfigBool(t *testing.T) {
	assert.True(t, configBool(map[string]interface{}{"flag": true}, "flag"))
	// String values like "yes" are NOT handled by configBool (only bool type assertion)
	assert.False(t, configBool(map[string]interface{}{"flag": 1}, "flag"))
	assert.False(t, configBool(map[string]interface{}{}, "flag"))
}

func TestConfigNodeList_NilInput(t *testing.T) {
	assert.Nil(t, configNodeList(nil))
}

func TestConfigNodeList_ValidNodes(t *testing.T) {
	nodes := configNodeList([]interface{}{
		map[string]interface{}{"host": "a"},
		map[string]interface{}{"host": "b"},
	})
	assert.Len(t, nodes, 2)
}

func TestConfigNodeList_InvalidNodeTypes(t *testing.T) {
	nodes := configNodeList([]interface{}{"string", 123})
	assert.Empty(t, nodes)
}
