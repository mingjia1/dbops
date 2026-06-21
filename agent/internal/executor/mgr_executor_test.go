package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- parseMGRConfig ---

func TestParseMGRConfig_Defaults(t *testing.T) {
	got := parseMGRConfig(map[string]interface{}{})
	assert.Equal(t, "mgr_cluster", got.GroupName)
	assert.Equal(t, "repl", got.ReplicateUser)
	assert.Equal(t, "repl123", got.ReplicatePass)
	assert.Equal(t, "single-primary", got.DeployMode)
	assert.Equal(t, 33061, got.LocalPort)
	assert.Equal(t, 3306, got.MySQLPort)
	assert.Equal(t, 1, got.ServerID)
	assert.Equal(t, 3306, got.PrimaryPort)
	assert.Equal(t, "root", got.MySQLUser)
	assert.Empty(t, got.LocalAddress)
	assert.Empty(t, got.PrimaryHost)
	assert.Empty(t, got.MySQLPassword)
	assert.Empty(t, got.GroupSeeds)
}

func TestParseMGRConfig_CustomValues(t *testing.T) {
	got := parseMGRConfig(map[string]interface{}{
		"group_name":       "my-mgr",
		"replicate_user":   "myrepl",
		"replicate_pass":   "myrepl!@#",
		"deploy_mode":      "multi-primary",
		"local_address":    "10.0.0.1",
		"local_port":       33062,
		"mysql_port":       3307,
		"server_id":        42,
		"primary_host":     "10.0.0.2",
		"primary_port":     3308,
		"mysql_user":       "admin",
		"mysql_password":   "admin!@#",
		"group_seeds":      []interface{}{"10.0.0.1:33062", "10.0.0.2:33062"},
	})
	assert.Equal(t, "my-mgr", got.GroupName)
	assert.Equal(t, "myrepl", got.ReplicateUser)
	assert.Equal(t, "myrepl!@#", got.ReplicatePass)
	assert.Equal(t, "multi-primary", got.DeployMode)
	assert.Equal(t, "10.0.0.1", got.LocalAddress)
	assert.Equal(t, 33062, got.LocalPort)
	assert.Equal(t, 3307, got.MySQLPort)
	assert.Equal(t, 42, got.ServerID)
	assert.Equal(t, "10.0.0.2", got.PrimaryHost)
	assert.Equal(t, 3308, got.PrimaryPort)
	assert.Equal(t, "admin", got.MySQLUser)
	assert.Equal(t, "admin!@#", got.MySQLPassword)
	assert.Equal(t, []string{"10.0.0.1:33062", "10.0.0.2:33062"}, got.GroupSeeds)
}

func TestParseMGRConfig_WrongTypes(t *testing.T) {
	got := parseMGRConfig(map[string]interface{}{
		"server_id":    "abc",
		"local_port":   "xyz",
		"mysql_port":   false,
		"primary_port": "def",
	})
	// Wrong types should fall back to defaults
	assert.Equal(t, 1, got.ServerID)
	assert.Equal(t, 33061, got.LocalPort)
	assert.Equal(t, 3306, got.MySQLPort)
	assert.Equal(t, 3306, got.PrimaryPort)
}

func TestParseMGRConfig_GroupSeedsAsStringSlice(t *testing.T) {
	got := parseMGRConfig(map[string]interface{}{
		"group_seeds": []string{"node1:33061", "node2:33061"},
	})
	// parseMGRConfig only handles []interface{} for group_seeds, not []string
	// So the expected result is nil (empty)
	assert.Empty(t, got.GroupSeeds)
}

func TestParseMGRConfig_EmptyGroupSeeds(t *testing.T) {
	got := parseMGRConfig(map[string]interface{}{})
	assert.Empty(t, got.GroupSeeds)
}

// --- MGRConfig struct ---

func TestMGRConfig_Fields(t *testing.T) {
	c := MGRConfig{
		GroupName:     "test-group",
		GroupSeeds:    []string{"a:33061", "b:33061"},
		ReplicateUser: "repl",
		ReplicatePass: "secret",
		DeployMode:    "single-primary",
		LocalAddress:  "10.0.0.1",
		LocalPort:     33061,
		MySQLPort:     3306,
		ServerID:      1,
		PrimaryHost:   "10.0.0.2",
		PrimaryPort:   3306,
		MySQLUser:     "root",
		MySQLPassword: "rootpass",
	}
	assert.Equal(t, "test-group", c.GroupName)
	assert.Len(t, c.GroupSeeds, 2)
	assert.Equal(t, "repl", c.ReplicateUser)
}

// --- GroupMemberStatus struct ---

func TestGroupMemberStatus_Fields(t *testing.T) {
	m := GroupMemberStatus{
		MemberID:    "uuid-abc",
		MemberHost:  "10.0.0.1",
		MemberPort:  3306,
		MemberState: "ONLINE",
		MemberRole:  "PRIMARY",
		Primary:     true,
	}
	assert.Equal(t, "ONLINE", m.MemberState)
	assert.Equal(t, "PRIMARY", m.MemberRole)
	assert.True(t, m.Primary)
}

// --- memberMatchesLocal ---

func TestMemberMatchesLocal_ExactHostMatch(t *testing.T) {
	member := GroupMemberStatus{MemberHost: "10.0.0.1", MemberPort: 3306}
	config := MGRConfig{LocalAddress: "10.0.0.1", MySQLPort: 3306}
	assert.True(t, memberMatchesLocal(member, config, ""))
}

func TestMemberMatchesLocal_HostnameMatch(t *testing.T) {
	member := GroupMemberStatus{MemberHost: "db-host-1", MemberPort: 3306}
	config := MGRConfig{LocalAddress: "10.0.0.1", MySQLPort: 3306}
	assert.True(t, memberMatchesLocal(member, config, "db-host-1"))
}

func TestMemberMatchesLocal_PortMismatch(t *testing.T) {
	member := GroupMemberStatus{MemberHost: "10.0.0.1", MemberPort: 3307}
	config := MGRConfig{LocalAddress: "10.0.0.1", MySQLPort: 3306}
	assert.False(t, memberMatchesLocal(member, config, ""))
}

func TestMemberMatchesLocal_NoMatch(t *testing.T) {
	member := GroupMemberStatus{MemberHost: "10.0.0.2", MemberPort: 3306}
	config := MGRConfig{LocalAddress: "10.0.0.1", MySQLPort: 3306}
	assert.False(t, memberMatchesLocal(member, config, ""))
}

// --- NewMGRExecutor ---

func TestNewMGRExecutor(t *testing.T) {
	e := NewMGRExecutor()
	require.NotNil(t, e)
	_, ok := interface{}(e).(*MGRExecutor)
	assert.True(t, ok)
}

// --- mgrLocalPort ---

func TestMgrLocalPort_Default(t *testing.T) {
	assert.Equal(t, 33061, mgrLocalPort(0))
}

func TestMgrLocalPort_FromMySQLPort(t *testing.T) {
	assert.Equal(t, 33061+3306%100, mgrLocalPort(3306))  // 33061+6 = 33067
	assert.Equal(t, 33061+3307%100, mgrLocalPort(3307))  // 33061+7 = 33068
	assert.Equal(t, 33061+24410%100, mgrLocalPort(24410)) // 33061+10 = 33071
}

// --- stableMGRGroupName ---

func TestStableMGRGroupName_FromClusterID(t *testing.T) {
	name := stableMGRGroupName("cluster-1", "")
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, name)
}

func TestStableMGRGroupName_Deterministic(t *testing.T) {
	a := stableMGRGroupName("cluster-1", "mgr-prod")
	b := stableMGRGroupName("cluster-1", "mgr-prod")
	assert.Equal(t, a, b)
}

func TestStableMGRGroupName_DifferentInputs(t *testing.T) {
	a := stableMGRGroupName("cluster-1", "")
	b := stableMGRGroupName("cluster-2", "")
	assert.NotEqual(t, a, b)
}

func TestStableMGRGroupName_EmptyFallback(t *testing.T) {
	name := stableMGRGroupName("", "")
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", name)
}

// --- DeployMGRSinglePrimary / ConfigureGroupMember (struct-level, not calling real mysql) ---

func TestDeployMGRSinglePrimary_MissingConfig(t *testing.T) {
	e := NewMGRExecutor()
	result, err := e.DeployMGRSinglePrimary(context.Background(), DeployTaskRequest{
		TaskID: "test-mgr",
		Config: map[string]interface{}{},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	// Without mysql, will fail during bootstrapPrimaryNode
	assert.Equal(t, "failed", result.Status)
}

func TestConfigureGroupMember_MissingConfig(t *testing.T) {
	e := NewMGRExecutor()
	result, err := e.ConfigureGroupMember(context.Background(), DeployTaskRequest{
		TaskID: "test-mgr-join",
		Config: map[string]interface{}{},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
}

func TestMonitorGroupStatus_MissingConfig(t *testing.T) {
	e := NewMGRExecutor()
	result, err := e.MonitorGroupStatus(context.Background(), DeployTaskRequest{
		TaskID: "test-mgr-mon",
		Config: map[string]interface{}{},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
}
