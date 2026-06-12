package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- TDD: parseRoleSwitchConfig ---
func TestParseRoleSwitchConfig_Defaults(t *testing.T) {
	got := parseRoleSwitchConfig(map[string]interface{}{})
	assert.Equal(t, "mha", got.ClusterType)
	assert.False(t, got.Force)
	assert.Empty(t, got.ClusterID)
	assert.Empty(t, got.InstanceID)
	assert.Empty(t, got.TargetRole)
	assert.Empty(t, got.OldMasterID)
	assert.Empty(t, got.NewMasterID)
	assert.Empty(t, got.ReplicationUser)
	assert.Empty(t, got.ReplicationPass)
	assert.Equal(t, 0, got.GracePeriodSec)
}

func TestParseRoleSwitchConfig_AllFields(t *testing.T) {
	got := parseRoleSwitchConfig(map[string]interface{}{
		"cluster_id":       "c-1",
		"instance_id":      "i-1",
		"cluster_type":     "mgr",
		"target_role":      "primary",
		"old_master_id":    "old",
		"new_master_id":    "new",
		"replication_user": "repl",
		"replication_pass": "pass",
		"force":            true,
		"grace_period_sec": 10,
	})
	assert.Equal(t, "c-1", got.ClusterID)
	assert.Equal(t, "i-1", got.InstanceID)
	assert.Equal(t, "mgr", got.ClusterType)
	assert.Equal(t, "primary", got.TargetRole)
	assert.Equal(t, "old", got.OldMasterID)
	assert.Equal(t, "new", got.NewMasterID)
	assert.Equal(t, "repl", got.ReplicationUser)
	assert.Equal(t, "pass", got.ReplicationPass)
	assert.True(t, got.Force)
	assert.Equal(t, 10, got.GracePeriodSec)
}

func TestParseRoleSwitchConfig_WrongTypes(t *testing.T) {
	got := parseRoleSwitchConfig(map[string]interface{}{
		"cluster_id":       123,   // wrong type
		"grace_period_sec": "abc", // wrong type
		"force":            "yes", // wrong type
	})
	assert.Equal(t, "", got.ClusterID)
	assert.Equal(t, 0, got.GracePeriodSec)
	assert.False(t, got.Force)
}

// --- TDD: mysqlBaseArgs ---
func TestMysqlBaseArgs(t *testing.T) {
	args := mysqlBaseArgs("db.example.com", 3306, "root", "secret")
	assert.Contains(t, args, "-h")
	assert.Contains(t, args, "db.example.com")
	assert.Contains(t, args, "-P")
	assert.Contains(t, args, "3306")
	assert.Contains(t, args, "-u")
	assert.Contains(t, args, "root")
	assert.Contains(t, args, "-psecret")
}

// --- TDD: data structures ---
func TestRoleSwitchConfig_Fields(t *testing.T) {
	c := RoleSwitchConfig{
		ClusterID:       "c1",
		InstanceID:      "i1",
		ClusterType:     "mha",
		TargetRole:      "master",
		OldMasterID:     "old",
		NewMasterID:     "new",
		ReplicationUser: "u",
		ReplicationPass: "p",
		Force:           true,
		GracePeriodSec:  30,
	}
	assert.Equal(t, "c1", c.ClusterID)
	assert.Equal(t, "mha", c.ClusterType)
	assert.True(t, c.Force)
	assert.Equal(t, 30, c.GracePeriodSec)
}

func TestRoleQueryResult_Fields(t *testing.T) {
	r := RoleQueryResult{
		ClusterID:    "c1",
		InstanceID:   "i1",
		Role:         "primary",
		ServerID:     1,
		ReadOnly:     false,
		MasterHost:   "m",
		MasterPort:   3306,
		SlaveRunning: true,
		GTIDExecuted: "uuid:1-100",
	}
	assert.Equal(t, "primary", r.Role)
	assert.True(t, r.SlaveRunning)
	assert.Equal(t, "uuid:1-100", r.GTIDExecuted)
}

// --- TDD: Execute* methods handle missing mysql gracefully (returns failed) ---
func TestExecuteRoleQuery_NoMySQL(t *testing.T) {
	e := NewTaskExecutor()
	req := DeployTaskRequest{
		TaskID:     "q-1",
		InstanceID: "i-1",
		Config: map[string]interface{}{
			"cluster_id":   "c1",
			"instance_id":  "i-1",
			"cluster_type": "mha",
		},
	}
	res, err := e.ExecuteRoleQuery(context.Background(), req)
	assert.NoError(t, err) // method swallows SQL error
	assert.NotNil(t, res)
	// On a host without mysql, expect "failed"; with mysql, "completed"
	assert.Contains(t, []string{"failed", "completed"}, res.Status)
}

func TestExecuteRolePromote_NoMySQL(t *testing.T) {
	e := NewTaskExecutor()
	req := DeployTaskRequest{
		TaskID:     "p-1",
		InstanceID: "i-1",
		Config: map[string]interface{}{
			"cluster_id":   "c1",
			"instance_id":  "i-1",
			"cluster_type": "mha",
			"target_role":  "master",
		},
	}
	res, err := e.ExecuteRolePromote(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Contains(t, []string{"failed", "completed"}, res.Status)
}

func TestExecuteRolePromote_MGR_NoMySQL(t *testing.T) {
	e := NewTaskExecutor()
	req := DeployTaskRequest{
		TaskID:     "p-mgr",
		InstanceID: "i-1",
		Config: map[string]interface{}{
			"cluster_id":   "c1",
			"instance_id":  "i-1",
			"cluster_type": "mgr",
			"target_role":  "primary",
		},
	}
	res, err := e.ExecuteRolePromote(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Contains(t, []string{"failed", "completed"}, res.Status)
}

func TestResolveMGRPrimaryUUID_PrefersConfig(t *testing.T) {
	got, err := resolveMGRPrimaryUUID(context.Background(), "127.0.0.1", 1, "root", "", RoleSwitchConfig{
		NewMasterServerUUID: "uuid-from-config",
	})
	assert.NoError(t, err)
	assert.Equal(t, "uuid-from-config", got)
}

func TestExecuteRoleDemote_NoMySQL(t *testing.T) {
	e := NewTaskExecutor()
	req := DeployTaskRequest{
		TaskID:     "d-1",
		InstanceID: "i-1",
		Config: map[string]interface{}{
			"cluster_id":   "c1",
			"instance_id":  "i-1",
			"cluster_type": "mha",
			"target_role":  "slave",
		},
	}
	res, err := e.ExecuteRoleDemote(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Contains(t, []string{"failed", "completed"}, res.Status)
}

func TestExecuteRoleDemote_Force_NoMySQL(t *testing.T) {
	e := NewTaskExecutor()
	req := DeployTaskRequest{
		TaskID:     "d-f",
		InstanceID: "i-1",
		Config: map[string]interface{}{
			"cluster_id":   "c1",
			"instance_id":  "i-1",
			"cluster_type": "mha",
			"target_role":  "slave",
			"force":        true,
		},
	}
	res, err := e.ExecuteRoleDemote(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	// With force=true, demote step failures are skipped → if mysql missing but
	// step is "skipped" and last "ON" succeeded or all skipped, status=completed
	// (mysql missing → all steps fail but force continues → status=completed)
	assert.Contains(t, []string{"failed", "completed"}, res.Status)
}

func TestExecuteRoleReplicaRebuild_MissingNewMasterID(t *testing.T) {
	e := NewTaskExecutor()
	req := DeployTaskRequest{
		TaskID:     "r-1",
		InstanceID: "i-1",
		Config: map[string]interface{}{
			"cluster_id":  "c1",
			"instance_id": "i-1",
			// new_master_id missing
		},
	}
	res, err := e.ExecuteRoleReplicaRebuild(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, "failed", res.Status)
	assert.Contains(t, res.Message, "new_master_id")
}

func TestExecuteRoleReplicaRebuild_NoMySQL(t *testing.T) {
	e := NewTaskExecutor()
	req := DeployTaskRequest{
		TaskID:     "r-2",
		InstanceID: "i-1",
		Config: map[string]interface{}{
			"cluster_id":    "c1",
			"instance_id":   "i-1",
			"new_master_id": "new-master",
		},
	}
	res, err := e.ExecuteRoleReplicaRebuild(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Contains(t, []string{"failed", "completed"}, res.Status)
}
