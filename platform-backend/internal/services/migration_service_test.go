package services

import (
	"context"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
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
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
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
	assert.NoError(t, err)
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
