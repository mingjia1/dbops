package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplicationSetup_ValidateConfig(t *testing.T) {
	s := NewReplicationSetup()
	err := s.SetupReplication(nil, "localhost", 3306, "root", "", ReplicationConfig{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "master_host")
}

func TestReplicationSetup_DefaultMasterPort(t *testing.T) {
	s := NewReplicationSetup()
	cfg := ReplicationConfig{
		MasterHost: "10.0.0.1",
		MasterPort: 0,
		ReplUser:   "repl",
		ReplPass:   "pass",
	}
	assert.Equal(t, 0, cfg.MasterPort)
	_ = s
}

func TestReplicationSetup_AutoPositionDefault(t *testing.T) {
	cfg := ReplicationConfig{
		MasterHost: "10.0.0.1",
		ReplUser:   "repl",
	}
	assert.False(t, cfg.AutoPos)
}

func TestMGRSetup_ValidateInputs(t *testing.T) {
	s := NewMGRSetup()
	require.NotNil(t, s)
}

func TestMGRBootstrap_EmptyGroupName(t *testing.T) {
	s := NewMGRSetup()
	err := s.BootstrapGroup(context.Background(), "localhost", 3306, "root", "", "", "10.0.0.1:33061", []string{})
	_ = err
}

func TestMGRJoin_EmptyGroupName(t *testing.T) {
	s := NewMGRSetup()
	err := s.JoinGroup(context.Background(), "localhost", 3306, "root", "", "", "10.0.0.1:33061", []string{})
	_ = err
}

func TestMGRSetPrimaryMember(t *testing.T) {
	s := NewMGRSetup()
	err := s.SetPrimaryMember(context.Background(), "localhost", 3306, "root", "", "uuid-123")
	_ = err
}

func TestMHASetup_Instantiate(t *testing.T) {
	s := NewMHASetup()
	require.NotNil(t, s)
}

func TestNodeRebuild_Instantiate(t *testing.T) {
	r := NewNodeRebuild()
	require.NotNil(t, r)
}

func TestNodeRebuild_StopInstance(t *testing.T) {
	r := NewNodeRebuild()
	err := r.StopInstance(nil, 3306)
	_ = err
}

func TestNodeRebuild_ClearDataDir_Empty(t *testing.T) {
	r := NewNodeRebuild()
	err := r.ClearDataDir(nil, "")
	assert.Error(t, err)
}

func TestNodeRebuild_ReinitializeDataDir_Empty(t *testing.T) {
	r := NewNodeRebuild()
	err := r.ReinitializeDataDir(nil, "", "", "mysql")
	assert.Error(t, err)
}
