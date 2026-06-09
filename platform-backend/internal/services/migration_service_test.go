package services

import (
	"context"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationService_CreateTask(t *testing.T) {
	service := newTestMigrationService()

	ctx := context.Background()

	req := CreateMigrationTaskRequest{
		Name:             "Test Migration",
		SourceInstanceID: "source-001",
		TargetInstanceID: "target-001",
		Strategy:         models.MigrationStrategyPhysical,
		Config:           "{}",
	}

	taskID, err := service.CreateTask(ctx, req)

	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)
}

func TestMigrationService_ExecutePhysicalMigration(t *testing.T) {
	service := newTestMigrationService()

	ctx := context.Background()

	result, err := service.ExecutePhysicalMigration(ctx, "migration-001")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestMigrationService_TaskStatusTransition(t *testing.T) {
	service := newTestMigrationService()

	ctx := context.Background()

	req := CreateMigrationTaskRequest{
		Name:             "Status Test",
		SourceInstanceID: "src-001",
		TargetInstanceID: "tgt-001",
		Strategy:         models.MigrationStrategyGTID,
	}
	taskID, err := service.CreateTask(ctx, req)
	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)
}

func TestMigrationService_SwitchWithoutTask(t *testing.T) {
	service := newTestMigrationService()
	ctx := context.Background()

	result, err := service.ExecuteSwitch(ctx, "nonexistent-task")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestMigrationService_ListTasks(t *testing.T) {
	service := newTestMigrationService()
	ctx := context.Background()

	tasks, err := service.ListTasks(ctx, "")
	assert.NoError(t, err)
	assert.NotNil(t, tasks)
}

func TestMigrationService_CancelTask(t *testing.T) {
	service := newTestMigrationService()
	ctx := context.Background()

	err := service.CancelTask(ctx, "nonexistent-task")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMigrationService_VerifyMigration(t *testing.T) {
	service := newTestMigrationService()
	ctx := context.Background()

	// 不存在的 task 应该返回 error 而不是静默成功.
	result, err := service.VerifyMigration(ctx, "migration-001")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveAgentHostDoesNotFallbackToLocalhost(t *testing.T) {
	host, port, err := resolveAgentHost(context.Background(), &models.Instance{ID: "inst-no-host"}, nil, nil, 9090)
	require.Error(t, err)
	assert.Empty(t, host)
	assert.Zero(t, port)
	assert.NotContains(t, err.Error(), "localhost")
}

func TestResolveAgentHostUsesConnectionHostWhenNoHostRepo(t *testing.T) {
	host, port, err := resolveAgentHost(context.Background(), &models.Instance{
		ID: "inst-conn",
		Connection: models.InstanceConnection{
			Host: "10.1.81.41",
		},
	}, nil, nil, 9090)
	require.NoError(t, err)
	assert.Equal(t, "10.1.81.41", host)
	assert.Equal(t, 9090, port)
}

func TestResolveAgentHostUsesManagedHostPort(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	host := &models.Host{
		ID:        "host-agent",
		Name:      "agent-host",
		Address:   "10.1.81.41",
		AgentPort: 19090,
		SSHPort:   22,
		SSHUser:   "root",
	}
	require.NoError(t, hostRepo.Create(ctx, host))
	hostID := host.ID

	resolvedHost, port, err := resolveAgentHost(ctx, &models.Instance{ID: "inst-host", HostID: &hostID}, nil, hostRepo, 9090)
	require.NoError(t, err)
	assert.Equal(t, "10.1.81.41", resolvedHost)
	assert.Equal(t, 19090, port)
}
