package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterDeployRepositoryListClustersReturnsLatestPerCluster(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewClusterDeployRepository(db)
	ctx := context.Background()
	oldTime := time.Now().Add(-time.Hour)
	newTime := time.Now()

	require.NoError(t, repo.Create(ctx, &models.ClusterDeployment{
		ID:          "dep-old",
		ClusterType: "mha",
		Name:        "old",
		Status:      "completed",
		ClusterID:   "cluster-001",
		CreatedAt:   oldTime,
		UpdatedAt:   oldTime,
	}))
	require.NoError(t, repo.Create(ctx, &models.ClusterDeployment{
		ID:          "dep-new",
		ClusterType: "mgr",
		Name:        "new",
		Status:      "completed",
		ClusterID:   "cluster-001",
		CreatedAt:   newTime,
		UpdatedAt:   newTime,
	}))
	require.NoError(t, repo.Create(ctx, &models.ClusterDeployment{
		ID:          "dep-empty-cluster",
		ClusterType: "mha",
		Name:        "empty",
		Status:      "completed",
		CreatedAt:   newTime,
		UpdatedAt:   newTime,
	}))

	clusters, err := repo.ListClusters(ctx)

	require.NoError(t, err)
	require.Len(t, clusters, 1)
	assert.Equal(t, "dep-new", clusters[0].ID)
	assert.Equal(t, "cluster-001", clusters[0].ClusterID)
	assert.Equal(t, "mgr", clusters[0].ClusterType)
}
