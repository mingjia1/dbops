package services

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchAgentActionAsyncReturnsSubmittedWithoutSSHWait(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewHostRepository(newTestDB(t))
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
	repo := repositories.NewHostRepository(newTestDB(t))
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

func TestDeleteAgentActionIsLongRunning(t *testing.T) {
	assert.True(t, IsLongRunningAgentAction("delete"))
	assert.True(t, IsLongRunningAgentAction("remove"))
}

func TestSubmitAgentActionReturnsSubmittedWithoutSSHWait(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewHostRepository(newTestDB(t))
	host := &models.Host{
		ID:        "host-single-async-agent",
		Name:      "single-async-agent-host",
		Address:   "10.255.255.1",
		SSHPort:   22,
		SSHUser:   "root",
		AgentPort: 9090,
	}
	require.NoError(t, repo.Create(ctx, host))

	service := NewHostService(repo, "test-encryption-key")
	started := time.Now()
	result, err := service.SubmitAgentAction(ctx, host.ID, HostAgentActionRequest{Action: "install"})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, time.Since(started) < time.Second, "single async agent install should return before SSH execution")
	assert.Equal(t, "submitted", result.Status)
	assert.Equal(t, "install", result.Action)
	assert.Equal(t, host.ID, result.HostID)
}

func TestAgentActionResultPersistsOnHost(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewHostRepository(newTestDB(t))
	host := &models.Host{
		ID:        "host-agent-result",
		Name:      "agent-result-host",
		Address:   "10.255.255.1",
		SSHPort:   22,
		SSHUser:   "root",
		AgentPort: 9090,
	}
	require.NoError(t, repo.Create(ctx, host))

	service := NewHostService(repo, "test-encryption-key")
	service.updateAgentActionHostStatus(&HostAgentActionResult{
		HostID:  host.ID,
		Action:  "restart",
		Status:  "failed",
		Message: "restart failed for test",
	})

	updated, err := repo.GetByID(ctx, host.ID)
	require.NoError(t, err)
	assert.Equal(t, "restart", updated.AgentLastAction)
	assert.Equal(t, "failed", updated.AgentLastResult)
	assert.Equal(t, "restart failed for test", updated.AgentLastMessage)
	require.NotNil(t, updated.AgentLastAt)
	assert.Equal(t, "failed", updated.Status)
}

func TestAgentDeleteSuccessMarksHostInactive(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewHostRepository(newTestDB(t))
	host := &models.Host{
		ID:        "host-agent-delete-result",
		Name:      "agent-delete-result-host",
		Address:   "10.255.255.1",
		SSHPort:   22,
		SSHUser:   "root",
		AgentPort: 9090,
	}
	require.NoError(t, repo.Create(ctx, host))

	service := NewHostService(repo, "test-encryption-key")
	service.updateAgentActionHostStatus(&HostAgentActionResult{
		HostID:  host.ID,
		Action:  "delete",
		Status:  "success",
		Message: "agent delete completed",
	})

	updated, err := repo.GetByID(ctx, host.ID)
	require.NoError(t, err)
	assert.Equal(t, "inactive", updated.Status)
	assert.Equal(t, "delete", updated.AgentLastAction)
	assert.Equal(t, "success", updated.AgentLastResult)
}

func TestAgentStatusWithoutConfiguredClientDoesNotPanic(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewHostRepository(newTestDB(t))
	host := &models.Host{
		ID:        "host-agent-no-client",
		Name:      "agent-no-client-host",
		Address:   "127.0.0.1",
		SSHPort:   22,
		SSHUser:   "root",
		AgentPort: 1,
	}
	require.NoError(t, repo.Create(ctx, host))

	service := NewHostService(repo, "test-encryption-key")
	result, err := service.AgentAction(ctx, host.ID, HostAgentActionRequest{Action: "status"})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	updated, err := repo.GetByID(ctx, host.ID)
	require.NoError(t, err)
	assert.Equal(t, "status", updated.AgentLastAction)
	assert.Equal(t, "failed", updated.AgentLastResult)
}

func TestRegisterScannedInstancesUpdatesPasswordForManagedPort(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	service := NewHostService(hostRepo, "test-encryption-key")
	service.SetInstanceRepo(instanceRepo)

	host := &models.Host{
		ID:      "host-scan-existing",
		Name:    "host-scan-existing",
		Address: "10.0.0.21",
		SSHPort: 22,
		SSHUser: "root",
	}
	require.NoError(t, hostRepo.Create(ctx, host))

	hostID := host.ID
	instance := &models.Instance{
		ID:     "instance-existing-23306",
		Name:   "existing-23306",
		HostID: &hostID,
	}
	require.NoError(t, instanceRepo.Create(ctx, instance))
	oldPassword, err := utils.Encrypt("old-password", "test-encryption-key")
	require.NoError(t, err)
	require.NoError(t, instanceRepo.CreateConnection(ctx, &models.InstanceConnection{
		InstanceID:        instance.ID,
		Host:              host.Address,
		Port:              23306,
		Username:          "root",
		PasswordEncrypted: oldPassword,
	}))

	result, err := service.RegisterScannedInstances(ctx, host.ID, BatchRegisterScannedInstanceRequest{
		Instances: []RegisterScannedInstanceRequest{{
			Port:     23306,
			Name:     "existing-23306",
			Username: "root",
			Password: "new-password",
		}},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Rows, 1)
	assert.Equal(t, 1, result.Updated)
	assert.Equal(t, 0, result.Registered)
	assert.Equal(t, "updated", result.Rows[0].Status)
	assert.Equal(t, instance.ID, result.Rows[0].InstanceID)

	conn, err := instanceRepo.GetConnection(ctx, instance.ID)
	require.NoError(t, err)
	plain, err := utils.Decrypt(conn.PasswordEncrypted, "test-encryption-key")
	require.NoError(t, err)
	assert.Equal(t, "new-password", plain)
}

func TestRegisterScannedInstancePersistsScanMetadata(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	service := NewHostService(hostRepo, "test-encryption-key")
	service.SetInstanceRepo(instanceRepo)

	host := &models.Host{
		ID:      "host-scan-metadata",
		Name:    "host-scan-metadata",
		Address: "10.0.0.31",
		SSHPort: 22,
		SSHUser: "root",
	}
	require.NoError(t, hostRepo.Create(ctx, host))

	instanceID, err := service.RegisterScannedInstance(ctx, host.ID, RegisterScannedInstanceRequest{
		Port:      3307,
		Name:      "scanned-3307",
		Username:  "root",
		Password:  "secret-password",
		VersionID: "mysql-8.0.36",
		Datadir:   "/data/mysql_3307",
	})

	require.NoError(t, err)
	conn, err := instanceRepo.GetConnection(ctx, instanceID)
	require.NoError(t, err)
	assert.Equal(t, "mysql-8.0.36", conn.VersionID)
	assert.Equal(t, "/data/mysql_3307", conn.Datadir)
	assert.Equal(t, host.Address, conn.Host)
	assert.Equal(t, 3307, conn.Port)
}
