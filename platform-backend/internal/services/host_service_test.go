package services

import (
	"context"
	"testing"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchAgentActionAsyncReturnsSubmittedWithoutSSHWait(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewHostRepository(newTestDB())
	host := &models.Host{
		ID:        "host-async-agent",
		Name:      "async-agent-host",
		Address:   "10.255.255.1",
		SSHPort:   22,
		SSHUser:   "root",
		AgentPort: 9090,
	}
	require.NoError(t, repo.Create(ctx, host))

	service := NewHostService(repo, "test-encryption-key")
	started := time.Now()
	result, err := service.BatchAgentAction(ctx, BatchHostAgentActionRequest{
		HostIDs: []string{host.ID},
		Action:  "install",
		Async:   true,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, time.Since(started) < time.Second, "async agent install should return before SSH execution")
	assert.True(t, result.Async)
	assert.Equal(t, 1, result.Success)
	assert.Equal(t, 0, result.Failed)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, "submitted", result.Rows[0].Status)
	assert.Equal(t, "install", result.Rows[0].Action)
}

func TestBatchAgentActionForcesLongRunningActionsAsync(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewHostRepository(newTestDB())
	host := &models.Host{
		ID:        "host-forced-async-agent",
		Name:      "forced-async-agent-host",
		Address:   "10.255.255.1",
		SSHPort:   22,
		SSHUser:   "root",
		AgentPort: 9090,
	}
	require.NoError(t, repo.Create(ctx, host))

	service := NewHostService(repo, "test-encryption-key")
	started := time.Now()
	result, err := service.BatchAgentAction(ctx, BatchHostAgentActionRequest{
		HostIDs: []string{host.ID},
		Action:  "install",
		Async:   false,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, time.Since(started) < time.Second, "long-running batch agent action should be submitted instead of waiting for SSH")
	assert.True(t, result.Async)
	assert.Equal(t, 1, result.Success)
	assert.Equal(t, 0, result.Failed)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, "submitted", result.Rows[0].Status)
}
