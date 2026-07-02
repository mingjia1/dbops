package repositories

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationRepositoryStatusUpdatesSetLifecycleTimes(t *testing.T) {
	ctx := context.Background()
	repo := NewMigrationRepository(newRepoTestDB(t))
	task := &models.MigrationTask{
		Name:             "migration lifecycle",
		SourceInstanceID: "source-001",
		TargetInstanceID: "target-001",
		Strategy:         models.MigrationStrategyPhysical,
		Status:           models.MigrationStatusPending,
	}
	require.NoError(t, repo.Create(ctx, task))

	require.NoError(t, repo.UpdateStatus(ctx, task.ID, models.MigrationStatusMigrating, 25))
	running, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	require.NotNil(t, running.StartedAt)
	assert.Nil(t, running.CompletedAt)
	assert.Equal(t, models.MigrationStatusMigrating, running.Status)
	assert.Equal(t, 25, running.Progress)

	require.NoError(t, repo.UpdateStatus(ctx, task.ID, models.MigrationStatusCompleted, 100))
	completed, err := repo.GetByID(ctx, task.ID)
	require.NoError(t, err)
	require.NotNil(t, completed.StartedAt)
	require.NotNil(t, completed.CompletedAt)
	assert.Equal(t, models.MigrationStatusCompleted, completed.Status)
	assert.Equal(t, 100, completed.Progress)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.NotNil(t, list[0].StartedAt)
	require.NotNil(t, list[0].CompletedAt)
}
