package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRepository_ListLogsOrdersAndPaginates(t *testing.T) {
	ctx := context.Background()
	db := newRepoTestDB(t)
	instanceRepo := NewInstanceRepository(db)
	require.NoError(t, instanceRepo.Create(ctx, &models.Instance{ID: "instance-log", Name: "instance-log"}))
	repo := NewTaskRepository(db)
	task := &models.Task{ID: "task-log", TaskType: "deploy", InstanceID: "instance-log", Status: "running"}
	require.NoError(t, repo.Create(ctx, task))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.AddLog(ctx, &models.TaskLog{TaskID: task.ID, LogID: "l2", Timestamp: base.Add(2 * time.Second), Level: "info", Message: "second"}))
	require.NoError(t, repo.AddLog(ctx, &models.TaskLog{TaskID: task.ID, LogID: "l1", Timestamp: base.Add(1 * time.Second), Level: "info", Message: "first"}))
	require.NoError(t, repo.AddLog(ctx, &models.TaskLog{TaskID: task.ID, LogID: "l3", Timestamp: base.Add(3 * time.Second), Level: "error", Message: "third"}))

	logs, err := repo.ListLogs(ctx, task.ID, 2, 1)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "second", logs[0].Message)
	assert.Equal(t, "third", logs[1].Message)

	got, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	require.Len(t, got.Logs, 3)
	assert.Equal(t, "first", got.Logs[0].Message)
}

func TestTaskRepository_UpdateStatusWithMessagePreservesUpgradePlanID(t *testing.T) {
	ctx := context.Background()
	db := newRepoTestDB(t)
	instanceRepo := NewInstanceRepository(db)
	require.NoError(t, instanceRepo.Create(ctx, &models.Instance{
		ID:   "instance-1",
		Name: "instance-1",
	}))
	repo := NewTaskRepository(db)
	task := &models.Task{
		ID:           "upgrade-task-with-plan",
		TaskType:     "upgrade_in_place",
		InstanceID:   "instance-1",
		Status:       "running",
		Progress:     10,
		ErrorMessage: "plan:plan-001",
	}
	require.NoError(t, repo.Create(ctx, task))

	require.NoError(t, repo.UpdateStatusWithMessage(ctx, task.ID, "failed", 100, "agent request failed"))

	got, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", got.Status)
	assert.Equal(t, 100, got.Progress)
	assert.Equal(t, "plan:plan-001\nagent request failed", got.ErrorMessage)
}
