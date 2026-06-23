package repositories

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskRepository_UpdateStatusWithMessagePreservesUpgradePlanID(t *testing.T) {
	ctx := context.Background()
	db := newRepoTestDB()
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
