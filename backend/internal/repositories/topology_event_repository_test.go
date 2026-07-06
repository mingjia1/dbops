package repositories

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testTopoDBCounter uint64

func newTestTopologyEventRepo() *TopologyEventRepository {
	n := atomic.AddUint64(&testTopoDBCounter, 1)
	dbPath := filepath.Join(os.TempDir(), fmt.Sprintf("topo-test-%d.db", n))
	_ = os.Remove(dbPath)
	db, err := NewDatabaseWithMode("", dbPath, "sqlite")
	if err != nil {
		panic(err)
	}
	RunMigrations(context.Background(), db)
	return NewTopologyEventRepository(db)
}

func TestTopologyEventRepository_CreateAndList(t *testing.T) {
	repo := newTestTopologyEventRepo()
	ctx := context.Background()

	event := &models.TopologyEvent{
		ID:          "evt-001",
		ClusterID:   "cluster-001",
		EventType:   "failover",
		OldMasterID: "inst-001",
		NewMasterID: "inst-002",
		NodeID:      "inst-002",
		Details:     "automatic failover",
		CreatedAt:   time.Now(),
	}

	err := repo.Create(ctx, event)
	require.NoError(t, err)

	events, err := repo.ListByCluster(ctx, "cluster-001", 10)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "failover", events[0].EventType)
}

func TestTopologyEventRepository_LatestByCluster(t *testing.T) {
	repo := newTestTopologyEventRepo()
	ctx := context.Background()

	now := time.Now()
	_ = repo.Create(ctx, &models.TopologyEvent{
		ID: "evt-001", ClusterID: "cluster-001", EventType: "switch", CreatedAt: now.Add(-time.Hour),
	})
	_ = repo.Create(ctx, &models.TopologyEvent{
		ID: "evt-002", ClusterID: "cluster-001", EventType: "failover", CreatedAt: now,
	})

	latest, err := repo.LatestByCluster(ctx, "cluster-001")
	require.NoError(t, err)
	assert.Equal(t, "failover", latest.EventType)
}

func TestTopologyEventRepository_DeleteByCluster(t *testing.T) {
	repo := newTestTopologyEventRepo()
	ctx := context.Background()

	_ = repo.Create(ctx, &models.TopologyEvent{
		ID: "evt-001", ClusterID: "cluster-001", EventType: "switch", CreatedAt: time.Now(),
	})

	err := repo.DeleteByCluster(ctx, "cluster-001")
	require.NoError(t, err)

	events, err := repo.ListByCluster(ctx, "cluster-001", 10)
	require.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestTopologyEventRepository_EmptyCluster(t *testing.T) {
	repo := newTestTopologyEventRepo()
	ctx := context.Background()

	events, err := repo.ListByCluster(ctx, "nonexistent", 10)
	require.NoError(t, err)
	assert.Len(t, events, 0)
}
