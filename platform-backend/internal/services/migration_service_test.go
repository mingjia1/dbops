package services

import (
	"context"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestMigrationService_CreateTask(t *testing.T) {
	service := NewMigrationService()

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
	assert.Contains(t, taskID, "migration-")
}

func TestMigrationService_ExecutePhysicalMigration(t *testing.T) {
	service := NewMigrationService()

	ctx := context.Background()

	result, err := service.ExecutePhysicalMigration(ctx, "migration-001")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "migration-001", result.TaskID)
	assert.Equal(t, models.MigrationStatusCompleted, result.Status)
	assert.Equal(t, models.MigrationStrategyPhysical, result.Strategy)
	assert.Equal(t, 100, result.Progress)
}

func TestMigrationService_ExecuteReplicationMigration(t *testing.T) {
	service := NewMigrationService()

	ctx := context.Background()

	result, err := service.ExecuteReplicationMigration(ctx, "migration-002")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "migration-002", result.TaskID)
	assert.Equal(t, models.MigrationStatusCompleted, result.Status)
	assert.Equal(t, models.MigrationStrategyReplication, result.Strategy)
	assert.Equal(t, 100, result.Progress)
}

func TestMigrationService_ExecuteGTIDMigration(t *testing.T) {
	service := NewMigrationService()

	ctx := context.Background()

	result, err := service.ExecuteGTIDMigration(ctx, "migration-003")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "migration-003", result.TaskID)
	assert.Equal(t, models.MigrationStatusCompleted, result.Status)
	assert.Equal(t, models.MigrationStrategyGTID, result.Strategy)
	assert.Equal(t, 100, result.Progress)
}

func TestMigrationService_OrchestrateMigration(t *testing.T) {
	service := NewMigrationService()

	ctx := context.Background()

	result, err := service.OrchestrateMigration(ctx, "migration-001")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "migration-001", result.TaskID)
	assert.Equal(t, models.MigrationStatusCompleted, result.Status)
	assert.Equal(t, 100, result.Progress)
}

func TestMigrationService_MonitorMigrationProgress(t *testing.T) {
	service := NewMigrationService()

	ctx := context.Background()

	progress, err := service.MonitorMigrationProgress(ctx, "migration-001")

	assert.NoError(t, err)
	assert.NotNil(t, progress)
	assert.Equal(t, "migration-001", progress.TaskID)
	assert.Equal(t, models.MigrationStatusMigrating, progress.Status)
	assert.Equal(t, 65, progress.Progress)
	assert.Equal(t, "data_sync", progress.CurrentStep)
	assert.Equal(t, 5, progress.TotalSteps)
	assert.Equal(t, 3, progress.CompletedSteps)
	assert.Greater(t, progress.DataTransferred, int64(0))
	assert.Greater(t, progress.EstimatedTime, 0)
}

func TestMigrationService_VerifyMigration(t *testing.T) {
	service := NewMigrationService()

	ctx := context.Background()

	verification, err := service.VerifyMigration(ctx, "migration-001")

	assert.NoError(t, err)
	assert.NotNil(t, verification)
	assert.Equal(t, "migration-001", verification.TaskID)
	assert.Equal(t, int64(1000000), verification.SourceCount)
	assert.Equal(t, int64(1000000), verification.TargetCount)
	assert.True(t, verification.DataConsistency)
	assert.True(t, verification.SchemaMatch)
	assert.True(t, verification.ChecksumMatch)
	assert.Empty(t, verification.Errors)
}

func TestMigrationService_ExecuteSwitch(t *testing.T) {
	service := NewMigrationService()

	ctx := context.Background()

	switchResult, err := service.ExecuteSwitch(ctx, "migration-001")

	assert.NoError(t, err)
	assert.NotNil(t, switchResult)
	assert.Equal(t, "migration-001", switchResult.TaskID)
	assert.NotEmpty(t, switchResult.OldMaster)
	assert.NotEmpty(t, switchResult.NewMaster)
	assert.True(t, switchResult.ApplicationUpdated)
	assert.Equal(t, "success", switchResult.Status)
}

func TestMigrationService_GetTask(t *testing.T) {
	service := NewMigrationService()

	ctx := context.Background()

	task, err := service.GetTask(ctx, "migration-001")

	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, "migration-001", task.ID)
	assert.NotEmpty(t, task.Name)
	assert.NotEmpty(t, task.SourceInstanceID)
	assert.NotEmpty(t, task.TargetInstanceID)
	assert.Equal(t, models.MigrationStrategyPhysical, task.Strategy)
	assert.Equal(t, models.MigrationStatusCompleted, task.Status)
	assert.Equal(t, 100, task.Progress)
}

func TestMigrationService_ListTasks(t *testing.T) {
	service := NewMigrationService()

	ctx := context.Background()

	tasks, err := service.ListTasks(ctx, "instance-001")

	assert.NoError(t, err)
	assert.NotNil(t, tasks)
	assert.NotEmpty(t, tasks)

	for _, task := range tasks {
		assert.NotEmpty(t, task.ID)
		assert.NotEmpty(t, task.Name)
	}
}

func TestMigrationService_CancelTask(t *testing.T) {
	service := NewMigrationService()

	ctx := context.Background()

	err := service.CancelTask(ctx, "migration-001")

	assert.NoError(t, err)
}