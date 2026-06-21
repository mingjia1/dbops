package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseMHAConfig ---

func TestParseMHAConfig_Defaults(t *testing.T) {
	got := parseMHAConfig(map[string]interface{}{})
	assert.Equal(t, "localhost", got.ManagerHost)
	assert.Equal(t, "localhost", got.MasterHost)
	assert.Equal(t, 3306, got.MasterPort)
	assert.Equal(t, "192.168.1.100", got.VIP)
	assert.Equal(t, "eth0", got.VIPInterface)
	assert.Equal(t, "repl", got.ReplUser)
	assert.Equal(t, "repl123", got.ReplPass)
	assert.Equal(t, "mha_manager", got.ManagerUser)
	assert.Equal(t, "mha123", got.ManagerPass)
	assert.Equal(t, "root", got.MySQLUser)
	assert.Equal(t, 3, got.PingInterval)
	assert.Equal(t, 3, got.PingRetry)
	assert.Equal(t, "root", got.SSHUser)
	assert.Equal(t, "/root/.ssh/id_rsa", got.SSHPrivateKey)
	assert.Empty(t, got.SlaveHosts)
	assert.Empty(t, got.SlavePorts)
	assert.Empty(t, got.SSHPasswords)
	assert.Empty(t, got.MySQLPassword)
}

func TestParseMHAConfig_CustomValues(t *testing.T) {
	got := parseMHAConfig(map[string]interface{}{
		"manager_host":    "mgr1",
		"master_host":     "db1",
		"master_port":     3307,
		"vip":             "10.0.0.100",
		"vip_interface":   "eth1",
		"repl_user":       "myrepl",
		"repl_pass":       "myrepl!@#",
		"manager_user":    "mgr-user",
		"manager_pass":    "mgr-pass",
		"mysql_user":      "admin",
		"mysql_password":  "admin!@#",
		"ping_interval":   5,
		"ping_retry":      10,
		"ssh_user":        "dbops",
		"ssh_private_key": "/home/dbops/.ssh/id_rsa",
	})
	assert.Equal(t, "mgr1", got.ManagerHost)
	assert.Equal(t, "db1", got.MasterHost)
	assert.Equal(t, 3307, got.MasterPort)
	assert.Equal(t, "10.0.0.100", got.VIP)
	assert.Equal(t, "eth1", got.VIPInterface)
	assert.Equal(t, "myrepl", got.ReplUser)
	assert.Equal(t, "myrepl!@#", got.ReplPass)
	assert.Equal(t, "mgr-user", got.ManagerUser)
	assert.Equal(t, "mgr-pass", got.ManagerPass)
	assert.Equal(t, "admin", got.MySQLUser)
	assert.Equal(t, "admin!@#", got.MySQLPassword)
	assert.Equal(t, 5, got.PingInterval)
	assert.Equal(t, 10, got.PingRetry)
	assert.Equal(t, "dbops", got.SSHUser)
	assert.Equal(t, "/home/dbops/.ssh/id_rsa", got.SSHPrivateKey)
}

func TestParseMHAConfig_SlaveHosts(t *testing.T) {
	got := parseMHAConfig(map[string]interface{}{
		"slave_hosts": []interface{}{"db2", "db3"},
		"slave_ports": []interface{}{3306, 3307},
	})
	assert.Equal(t, []string{"db2", "db3"}, got.SlaveHosts)
	assert.Equal(t, []int{3306, 3307}, got.SlavePorts)
}

func TestParseMHAConfig_SlaveHostsAsStringSlice(t *testing.T) {
	got := parseMHAConfig(map[string]interface{}{
		"slave_hosts": []string{"db2", "db3"},
	})
	assert.Equal(t, []string{"db2", "db3"}, got.SlaveHosts)
}

func TestParseMHAConfig_SSHPasswords(t *testing.T) {
	got := parseMHAConfig(map[string]interface{}{
		"ssh_passwords": map[string]interface{}{
			"db1": "pass1",
			"db2": "pass2",
		},
	})
	assert.Equal(t, "pass1", got.SSHPasswords["db1"])
	assert.Equal(t, "pass2", got.SSHPasswords["db2"])
}

func TestParseMHAConfig_SSHPasswordsAsStringMap(t *testing.T) {
	got := parseMHAConfig(map[string]interface{}{
		"ssh_passwords": map[string]string{
			"db1": "pass1",
		},
	})
	assert.Equal(t, "pass1", got.SSHPasswords["db1"])
}

func TestParseMHAConfig_WrongTypes(t *testing.T) {
	got := parseMHAConfig(map[string]interface{}{
		"master_port":    "abc",
		"ping_interval":  false,
		"ping_retry":     3.14,
	})
	// Wrong types should fall back to defaults
	assert.Equal(t, 3306, got.MasterPort)
	assert.Equal(t, 3, got.PingInterval)
	assert.Equal(t, 3, got.PingRetry)
}

// --- MHAConfig struct ---

func TestMHAConfig_Fields(t *testing.T) {
	c := MHAConfig{
		ManagerHost:   "manager",
		MasterHost:    "master",
		MasterPort:    3306,
		SlaveHosts:    []string{"slave1", "slave2"},
		SlavePorts:    []int{3306, 3307},
		VIP:           "10.0.0.100",
		VIPInterface:  "eth0",
		ReplUser:      "repl",
		ReplPass:      "repl123",
		ManagerUser:   "mha",
		ManagerPass:   "mha123",
		MySQLUser:     "root",
		MySQLPassword: "rootpass",
		SSHUser:       "root",
		SSHPrivateKey: "/root/.ssh/id_rsa",
	}
	assert.Equal(t, "manager", c.ManagerHost)
	assert.Equal(t, "master", c.MasterHost)
	assert.Len(t, c.SlaveHosts, 2)
	assert.Equal(t, "repl", c.ReplUser)
}

// --- NewMHAExecutor ---

func TestNewMHAExecutor(t *testing.T) {
	e := NewMHAExecutor()
	require.NotNil(t, e)
	_, ok := interface{}(e).(*MHAExecutor)
	assert.True(t, ok)
}

// --- DeployMHA (struct-level test, no mysql) ---

func TestDeployMHA_MissingConfig(t *testing.T) {
	e := NewMHAExecutor()
	result, err := e.DeployMHA(context.Background(), DeployTaskRequest{
		TaskID: "test-mha",
		Config: map[string]interface{}{},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
}

// --- generateMHAAppConfig ---

func TestGenerateMHAAppConfig(t *testing.T) {
	config := MHAConfig{
		ManagerHost:  "10.0.0.1",
		MasterHost:   "10.0.0.2",
		MasterPort:   3306,
		SlaveHosts:   []string{"10.0.0.3", "10.0.0.4"},
		SlavePorts:   []int{3306, 3307},
		ReplUser:     "repl",
		ReplPass:     "repl123",
		ManagerUser:  "mha",
		ManagerPass:  "mha123",
		SSHUser:      "root",
		PingInterval: 3,
	}

	content := generateMHAAppConfig(config)

	assert.Contains(t, content, "user=mha")
	assert.Contains(t, content, "password=mha123")
	assert.Contains(t, content, "repl_user=repl")
	assert.Contains(t, content, "repl_password=repl123")
	assert.Contains(t, content, "ssh_user=root")
	assert.Contains(t, content, "ping_interval=3")
	assert.Contains(t, content, "[server1]")
	assert.Contains(t, content, "hostname=10.0.0.2")
	assert.Contains(t, content, "port=3306")
	assert.Contains(t, content, "[server2]")
	assert.Contains(t, content, "hostname=10.0.0.3")
	assert.Contains(t, content, "[server3]")
	assert.Contains(t, content, "hostname=10.0.0.4")
	assert.Contains(t, content, "port=3307")
	assert.Contains(t, content, "candidate_master=1")
}

func TestGenerateMHAAppConfig_SingleSlave(t *testing.T) {
	config := MHAConfig{
		MasterHost:   "10.0.0.2",
		MasterPort:   3306,
		SlaveHosts:   []string{"10.0.0.3"},
		ManagerUser:  "mha",
		ManagerPass:  "mha123",
		ReplUser:     "repl",
		ReplPass:     "repl",
	}
	content := generateMHAAppConfig(config)
	assert.Contains(t, content, "[server1]")
	assert.Contains(t, content, "[server2]")
	assert.NotContains(t, content, "[server3]")
}

// --- installMHACommand ---

func TestInstallMHACommand_Node(t *testing.T) {
	cmd := installMHACommand("node")
	assert.Contains(t, cmd, "mha4mysql-node")
	assert.Contains(t, cmd, "save_binary_logs")
	assert.NotContains(t, cmd, "mha4mysql-manager")
}

func TestInstallMHACommand_Manager(t *testing.T) {
	cmd := installMHACommand("manager")
	assert.Contains(t, cmd, "mha4mysql-manager")
	assert.Contains(t, cmd, "masterha_check_ssh")
	assert.Contains(t, cmd, "masterha_check_repl")
}

// --- isLocalHost ---

func TestIsLocalHost_Empty(t *testing.T) {
	assert.True(t, isLocalHost(""))
}

func TestIsLocalHost_Localhost(t *testing.T) {
	assert.True(t, isLocalHost("localhost"))
	assert.True(t, isLocalHost("127.0.0.1"))
	assert.True(t, isLocalHost("::1"))
}

func TestIsLocalHost_Remote(t *testing.T) {
	assert.False(t, isLocalHost("10.0.0.1"))
}

// --- uniqueStrings ---

func TestUniqueStrings_Deduplicates(t *testing.T) {
	got := uniqueStrings([]string{"a", "b", "a", "c", "b", ""})
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

func TestUniqueStrings_EmptyInput(t *testing.T) {
	got := uniqueStrings(nil)
	assert.Empty(t, got)
}

// --- shellQuote ---

func TestShellQuote_Simple(t *testing.T) {
	assert.Equal(t, "'hello'", shellQuote("hello"))
}

func TestShellQuote_WithQuotes(t *testing.T) {
	assert.Equal(t, "'it'\\''s'", shellQuote("it's"))
}

// --- passwordForHost ---

func TestPasswordForHost_ExactMatch(t *testing.T) {
	passwords := map[string]string{"db1": "pass1", "db2": "pass2"}
	assert.Equal(t, "pass1", passwordForHost(passwords, "db1"))
}

func TestPasswordForHost_NilMap(t *testing.T) {
	assert.Equal(t, "", passwordForHost(nil, "db1"))
}

func TestPasswordForHost_NotFound(t *testing.T) {
	passwords := map[string]string{"db1": "pass1"}
	assert.Equal(t, "", passwordForHost(passwords, "db3"))
}

// --- mhaOutputContains ---

func TestMHAOutputContains_Found(t *testing.T) {
	assert.True(t, mhaOutputContains("All SSH connection tests passed successfully", "passed"))
}

func TestMHAOutputContains_NotFound(t *testing.T) {
	assert.False(t, mhaOutputContains("some output", "not-found"))
}

// --- mhaReplicaStatusContainsRunning ---

func TestMHAReplicaStatusContainsRunning_Modern(t *testing.T) {
	assert.True(t, mhaReplicaStatusContainsRunning("Replica_IO_Running: Yes\nReplica_SQL_Running: Yes", "Replica_IO_Running", "Slave_IO_Running"))
	assert.True(t, mhaReplicaStatusContainsRunning("Replica_SQL_Running: Yes\nReplica_IO_Running: Yes", "Replica_SQL_Running", "Slave_SQL_Running"))
}

func TestMHAReplicaStatusContainsRunning_Legacy(t *testing.T) {
	assert.True(t, mhaReplicaStatusContainsRunning("Slave_IO_Running: Yes", "Replica_IO_Running", "Slave_IO_Running"))
	assert.True(t, mhaReplicaStatusContainsRunning("Slave_SQL_Running: Yes", "Replica_SQL_Running", "Slave_SQL_Running"))
}

func TestMHAReplicaStatusContainsRunning_NotRunning(t *testing.T) {
	assert.False(t, mhaReplicaStatusContainsRunning("Slave_IO_Running: No", "Replica_IO_Running", "Slave_IO_Running"))
	assert.False(t, mhaReplicaStatusContainsRunning("", "Replica_IO_Running", "Slave_IO_Running"))
}

// --- passwordKeyboardInteractive ---

func TestPasswordKeyboardInteractive_ReturnsPassword(t *testing.T) {
	fn := passwordKeyboardInteractive("secret")
	answers, err := fn("root", "", []string{"password:"}, []bool{false})
	require.NoError(t, err)
	assert.Equal(t, []string{"secret"}, answers)
}

func TestPasswordKeyboardInteractive_MultipleQuestions(t *testing.T) {
	fn := passwordKeyboardInteractive("secret")
	answers, err := fn("root", "", []string{"q1", "q2"}, []bool{false, false})
	require.NoError(t, err)
	assert.Equal(t, []string{"secret", "secret"}, answers)
}

// --- validateMHAReplica (no mysql - would fail) ---
// Skipped because it requires real MySQL connection

// --- buildReplicaLocalSQLCommand ---

func TestBuildReplicaLocalSQLCommand_ContainsSocketAndTCP(t *testing.T) {
	cmd := buildReplicaLocalSQLCommand(3306, "root", "pass", "SELECT 1")
	assert.Contains(t, cmd, "mysql.sock")
	assert.Contains(t, cmd, "127.0.0.1")
	assert.Contains(t, cmd, "MYSQL_PWD=")
}

func TestBuildReplicaLocalSQLCommand_NoPassword(t *testing.T) {
	cmd := buildReplicaLocalSQLCommand(3306, "root", "", "SELECT 1")
	assert.NotContains(t, cmd, "MYSQL_PWD=")
}
