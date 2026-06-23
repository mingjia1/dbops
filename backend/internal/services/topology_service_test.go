package services

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopologyService_GetInstanceTopology(t *testing.T) {
	mockRepo := new(MockInstanceRepo)
	service := NewTestableTopologyService(mockRepo)

	ctx := context.Background()

	testInstance := &models.Instance{
		ID:        "instance-001",
		Name:      "test-master",
		ClusterID: "cluster-001",
	}

	mockRepo.On("GetByID", ctx, "instance-001").Return(testInstance, nil)

	result, err := service.GetInstanceTopology(ctx, "instance-001")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "instance-001", result.InstanceID)
	assert.Equal(t, "cluster-001", result.ClusterID)
	assert.Equal(t, "async", result.ReplicationMode)

	mockRepo.AssertExpectations(t)
}

func TestTopologyService_GetInstanceTopology_InstanceNotFound(t *testing.T) {
	mockRepo := new(MockInstanceRepo)
	service := NewTestableTopologyService(mockRepo)

	ctx := context.Background()

	mockRepo.On("GetByID", ctx, "non-existent").Return(nil, errors.New("instance not found"))

	result, err := service.GetInstanceTopology(ctx, "non-existent")

	assert.Error(t, err)
	assert.Nil(t, result)

	mockRepo.AssertExpectations(t)
}

func TestTopologyService_GetClusterTopology(t *testing.T) {
	mockRepo := new(MockInstanceRepo)
	service := NewTestableTopologyService(mockRepo)

	ctx := context.Background()

	instances := []models.Instance{
		{
			ID:        "instance-001",
			Name:      "master",
			ClusterID: "cluster-001",
		},
		{
			ID:        "instance-002",
			Name:      "slave",
			ClusterID: "cluster-001",
		},
	}

	mockRepo.On("List", ctx, 100, 0).Return(instances, nil)

	result, err := service.GetClusterTopology(ctx, "cluster-001")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "cluster-001", result.ClusterID)
	assert.NotEmpty(t, result.Instances)
	assert.Len(t, result.Instances, 2)

	mockRepo.AssertExpectations(t)
}

func TestTopologyService_GetClusterTopology_EmptyCluster(t *testing.T) {
	mockRepo := new(MockInstanceRepo)
	service := NewTestableTopologyService(mockRepo)

	ctx := context.Background()

	instances := []models.Instance{}

	mockRepo.On("List", ctx, 100, 0).Return(instances, nil)

	result, err := service.GetClusterTopology(ctx, "cluster-empty")

	assert.Error(t, err)
	assert.Nil(t, result)

	mockRepo.AssertExpectations(t)
}

func TestTopologyService_InferClusterTopologyWhenRelationsMissing(t *testing.T) {
	instances := []InstanceTopologyResponse{
		{InstanceID: "master-1", ClusterID: "cluster-001", Role: "primary", ReplicationMode: "async"},
		{InstanceID: "replica-1", ClusterID: "cluster-001", Role: "replica", ReplicationMode: "async"},
		{InstanceID: "replica-2", ClusterID: "cluster-001", Role: "", ReplicationMode: "async"},
	}

	inferClusterTopology(instances)

	assert.Equal(t, []string{"replica-1", "replica-2"}, instances[0].SlaveIDs)
	assert.Equal(t, "master-1", instances[1].MasterID)
	assert.Equal(t, "master-1", instances[2].MasterID)
	assert.Equal(t, "replica", instances[2].Role)
}

func TestTopologyService_ParseTopologySlaveIDsSupportsJSONAndCSV(t *testing.T) {
	assert.Equal(t, []string{"replica-1", "replica-2"}, parseTopologySlaveIDs(`["replica-1","replica-2"]`))
	assert.Equal(t, []string{"replica-1", "replica-2"}, parseTopologySlaveIDs("replica-1, replica-2,,"))
	assert.Empty(t, parseTopologySlaveIDs(""))
}

func TestTopologyService_GetClusterTopologyInfersSinglePrimaryWhenRolesMissing(t *testing.T) {
	mockRepo := new(MockInstanceRepo)
	service := NewTestableTopologyService(mockRepo)

	ctx := context.Background()
	instances := []models.Instance{
		{ID: "instance-001", Name: "node-1", ClusterID: "cluster-001"},
		{ID: "instance-002", Name: "node-2", ClusterID: "cluster-001"},
		{ID: "instance-003", Name: "node-3", ClusterID: "cluster-001"},
	}

	mockRepo.On("List", ctx, 100, 0).Return(instances, nil)

	result, err := service.GetClusterTopology(ctx, "cluster-001")

	assert.NoError(t, err)
	assert.Len(t, result.Instances, 3)
	assert.Equal(t, "master", result.Instances[0].Role)
	assert.Equal(t, []string{"instance-002", "instance-003"}, result.Instances[0].SlaveIDs)
	assert.Equal(t, "replica", result.Instances[1].Role)
	assert.Equal(t, "instance-001", result.Instances[1].MasterID)
	assert.Equal(t, "replica", result.Instances[2].Role)
	assert.Equal(t, "instance-001", result.Instances[2].MasterID)

	mockRepo.AssertExpectations(t)
}

func TestTopologyService_BuildTopologyGraphUsesPersistedStatusAndMode(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	repo := repositories.NewInstanceRepository(db)
	service := NewTopologyService(repo)
	clusterID := "cluster-topology-status"

	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "topo-master", Name: "topo-master", ClusterID: clusterID}))
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "topo-replica", Name: "topo-replica", ClusterID: clusterID}))
	require.NoError(t, repo.UpsertStatus(ctx, "topo-master", &models.InstanceStatus{
		RunStatus:    "running",
		HealthStatus: "healthy",
		Role:         "master",
	}))
	require.NoError(t, repo.UpsertStatus(ctx, "topo-replica", &models.InstanceStatus{
		RunStatus:    "stopped",
		HealthStatus: "unhealthy",
		Role:         "replica",
	}))
	require.NoError(t, repo.UpsertTopology(ctx, "topo-master", &models.InstanceTopology{
		ClusterID:       clusterID,
		SlaveIDs:        `["topo-replica"]`,
		ReplicationMode: "semisync",
	}))
	require.NoError(t, repo.UpsertTopology(ctx, "topo-replica", &models.InstanceTopology{
		ClusterID:       clusterID,
		MasterID:        "topo-master",
		ReplicationMode: "semisync",
	}))

	topology, err := service.GetClusterTopology(ctx, clusterID)
	require.NoError(t, err)
	assert.Equal(t, "semisync", topology.ReplicationMode)

	graph, err := service.BuildTopologyGraph(ctx, clusterID)
	require.NoError(t, err)
	require.Len(t, graph.Nodes, 2)
	byID := map[string]TopologyNode{}
	for _, node := range graph.Nodes {
		byID[node.ID] = node
	}
	assert.Equal(t, "healthy", byID["topo-master"].Status)
	assert.Equal(t, "unhealthy", byID["topo-replica"].Status)
	require.Len(t, graph.Edges, 1)
	assert.Equal(t, "semisync", graph.Edges[0].Label)
}

func TestTopologyService_BuildTopologyGraphParsesCSVSlaveIDs(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	repo := repositories.NewInstanceRepository(db)
	service := NewTopologyService(repo)
	clusterID := "cluster-topology-csv"

	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "csv-master", Name: "csv-master", ClusterID: clusterID}))
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "csv-replica-1", Name: "csv-replica-1", ClusterID: clusterID}))
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "csv-replica-2", Name: "csv-replica-2", ClusterID: clusterID}))
	require.NoError(t, repo.UpsertStatus(ctx, "csv-master", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	require.NoError(t, repo.UpsertStatus(ctx, "csv-replica-1", &models.InstanceStatus{Role: "replica", HealthStatus: "healthy"}))
	require.NoError(t, repo.UpsertStatus(ctx, "csv-replica-2", &models.InstanceStatus{Role: "replica", HealthStatus: "healthy"}))
	require.NoError(t, repo.UpsertTopology(ctx, "csv-master", &models.InstanceTopology{
		ClusterID:       clusterID,
		SlaveIDs:        "csv-replica-1, csv-replica-2",
		ReplicationMode: "async",
	}))

	graph, err := service.BuildTopologyGraph(ctx, clusterID)

	require.NoError(t, err)
	require.Len(t, graph.Edges, 2)
	targets := map[string]bool{}
	for _, edge := range graph.Edges {
		assert.Equal(t, "csv-master", edge.SourceID)
		targets[edge.TargetID] = true
	}
	assert.True(t, targets["csv-replica-1"])
	assert.True(t, targets["csv-replica-2"])
}

func TestTopologyService_GetClusterTopologyUsesClusterQueryBeyondFirstPage(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	repo := repositories.NewInstanceRepository(db)
	service := NewTopologyService(repo)
	targetClusterID := "cluster-target-beyond-page"

	for i := 0; i < 120; i++ {
		require.NoError(t, repo.Create(ctx, &models.Instance{
			ID:        fmt.Sprintf("noise-%03d", i),
			Name:      fmt.Sprintf("noise-%03d", i),
			ClusterID: "noise-cluster",
		}))
	}
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "target-master", Name: "target-master", ClusterID: targetClusterID}))
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "target-replica", Name: "target-replica", ClusterID: targetClusterID}))
	require.NoError(t, repo.UpsertStatus(ctx, "target-master", &models.InstanceStatus{Role: "master", HealthStatus: "healthy"}))
	require.NoError(t, repo.UpsertStatus(ctx, "target-replica", &models.InstanceStatus{Role: "replica", HealthStatus: "healthy"}))
	require.NoError(t, repo.UpsertTopology(ctx, "target-replica", &models.InstanceTopology{
		ClusterID:       targetClusterID,
		MasterID:        "target-master",
		ReplicationMode: "async",
	}))

	topology, err := service.GetClusterTopology(ctx, targetClusterID)

	require.NoError(t, err)
	require.Len(t, topology.Instances, 2)
	ids := map[string]bool{}
	for _, inst := range topology.Instances {
		ids[inst.InstanceID] = true
	}
	assert.True(t, ids["target-master"])
	assert.True(t, ids["target-replica"])
}
