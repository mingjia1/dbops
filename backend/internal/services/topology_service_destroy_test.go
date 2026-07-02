package services

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockInstanceRepoForDestroy 用于测试集群销毁的mock
type MockInstanceRepoForDestroy struct {
	mock.Mock
	instances map[string]*models.Instance
}

func (m *MockInstanceRepoForDestroy) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	if inst, ok := m.instances[id]; ok {
		return inst, nil
	}
	return nil, fmt.Errorf("instance not found: %s", id)
}

func (m *MockInstanceRepoForDestroy) ListByClusterID(ctx context.Context, clusterID string) ([]*models.Instance, error) {
	var result []*models.Instance
	for _, inst := range m.instances {
		if inst.ClusterID == clusterID {
			result = append(result, inst)
		}
	}
	return result, nil
}

func (m *MockInstanceRepoForDestroy) List(ctx context.Context, limit, offset int) ([]models.Instance, error) {
	return nil, nil
}

func (m *MockInstanceRepoForDestroy) Create(ctx context.Context, instance *models.Instance) error {
	return nil
}

func (m *MockInstanceRepoForDestroy) Update(ctx context.Context, instance *models.Instance) error {
	return nil
}

func (m *MockInstanceRepoForDestroy) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *MockInstanceRepoForDestroy) GetConnection(ctx context.Context, instanceID string) (*models.InstanceConnection, error) {
	return nil, nil
}

func (m *MockInstanceRepoForDestroy) CreateConnection(ctx context.Context, conn *models.InstanceConnection) error {
	return nil
}

func (m *MockInstanceRepoForDestroy) UpdateConnection(ctx context.Context, conn *models.InstanceConnection) error {
	return nil
}

func (m *MockInstanceRepoForDestroy) UpdateConnectionPassword(ctx context.Context, instanceID, encryptedPassword string) error {
	return nil
}

func (m *MockInstanceRepoForDestroy) UpsertStatus(ctx context.Context, instanceID string, status *models.InstanceStatus) error {
	return nil
}

func (m *MockInstanceRepoForDestroy) UpsertTopology(ctx context.Context, instanceID string, topology *models.InstanceTopology) error {
	return nil
}

func (m *MockInstanceRepoForDestroy) ListByHostID(ctx context.Context, hostID string, limit, offset int) ([]models.Instance, error) {
	return nil, nil
}

func (m *MockInstanceRepoForDestroy) ListAllEndpoints(ctx context.Context) ([]repositories.InstanceEndpoint, error) {
	return nil, nil
}

func (m *MockInstanceRepoForDestroy) ListEndpointsByHosts(ctx context.Context, hosts []string) ([]repositories.InstanceEndpoint, error) {
	return nil, nil
}

// TestTopologyAfterClusterDestroy 测试集群销毁后拓扑信息的清除
func TestTopologyAfterClusterDestroy(t *testing.T) {
	mockRepo := &MockInstanceRepoForDestroy{
		instances: make(map[string]*models.Instance),
	}
	topoSvc := NewTopologyService(mockRepo)
	clusterID := "test-cluster-destroy-topo"

	ctx := context.Background()

	// 1. 创建测试集群和实例
	mockRepo.instances["master-1"] = &models.Instance{
		ID:        "master-1",
		Name:      "master-node",
		ClusterID: clusterID,
		Status: models.InstanceStatus{
			RunStatus:    "running",
			HealthStatus: "healthy",
			Role:         "master",
		},
		Topology: models.InstanceTopology{
			InstanceID:      "master-1",
			ClusterID:       clusterID,
			SlaveIDs:        `["slave-1", "slave-2"]`,
			ReplicationMode: "async",
		},
	}

	mockRepo.instances["slave-1"] = &models.Instance{
		ID:        "slave-1",
		Name:      "slave-node-1",
		ClusterID: clusterID,
		Status: models.InstanceStatus{
			RunStatus:    "running",
			HealthStatus: "healthy",
			Role:         "replica",
		},
		Topology: models.InstanceTopology{
			InstanceID:      "slave-1",
			ClusterID:       clusterID,
			MasterID:        "master-1",
			ReplicationMode: "async",
		},
	}

	mockRepo.instances["slave-2"] = &models.Instance{
		ID:        "slave-2",
		Name:      "slave-node-2",
		ClusterID: clusterID,
		Status: models.InstanceStatus{
			RunStatus:    "running",
			HealthStatus: "healthy",
			Role:         "replica",
		},
		Topology: models.InstanceTopology{
			InstanceID:      "slave-2",
			ClusterID:       clusterID,
			MasterID:        "master-1",
			ReplicationMode: "async",
		},
	}

	mockRepo.On("GetByID", ctx, "master-1").Return(nil, nil)
	mockRepo.On("GetByID", ctx, "slave-1").Return(nil, nil)
	mockRepo.On("GetByID", ctx, "slave-2").Return(nil, nil)

	// 2. 验证销毁前拓扑可正常获取
	clusterTopo, err := topoSvc.GetClusterTopology(ctx, clusterID)
	assert.NoError(t, err)
	assert.NotNil(t, clusterTopo)
	assert.Equal(t, 3, len(clusterTopo.Instances))

	graph, err := topoSvc.BuildTopologyGraph(ctx, clusterID)
	assert.NoError(t, err)
	assert.NotNil(t, graph)
	assert.Equal(t, 3, len(graph.Nodes))
	assert.True(t, len(graph.Edges) >= 2, "应该有复制关系边")

	// 3. 模拟集群销毁：清除所有实例的集群关联和拓扑信息
	for id, inst := range mockRepo.instances {
		inst.ClusterID = ""
		inst.Status.RunStatus = "stopped"
		inst.Status.HealthStatus = "offline"
		inst.Status.Role = ""
		inst.Topology.ClusterID = ""
		inst.Topology.MasterID = ""
		inst.Topology.SlaveIDs = ""
		inst.Topology.ReplicationMode = ""
		mockRepo.instances[id] = inst
	}

	// 4. 验证销毁后拓扑查询应返回错误（集群不再有有效实例）
	clusterTopo, err = topoSvc.GetClusterTopology(ctx, clusterID)
	assert.Error(t, err, "已销毁集群应返回错误")
	if err != nil {
		assert.True(t,
			strings.Contains(err.Error(), "no valid instances") || strings.Contains(err.Error(), "has no instances"),
			"错误信息应表明没有有效实例，实际: %s", err.Error())
	}

	graph, err = topoSvc.BuildTopologyGraph(ctx, clusterID)
	assert.Error(t, err, "已销毁集群图应返回错误")
}

// TestTopologyNormalization 测试拓扑信息的规范化展示
func TestTopologyNormalization(t *testing.T) {
	testCases := []struct {
		role         string
		expectedRole string
	}{
		{"primary_master", "primary"},
		{"slave", "replica"},
		{"secondary", "replica"},
		{"", "unknown"},
		{"master", "master"},
	}

	for _, tc := range testCases {
		t.Run(tc.role, func(t *testing.T) {
			normalized := normalizeDisplayRole(tc.role)
			assert.Equal(t, tc.expectedRole, normalized)
		})
	}
}

// TestReplicationLabelNormalization 测试复制模式标签规范化
func TestReplicationLabelNormalization(t *testing.T) {
	testCases := []struct {
		mode         string
		expectedMode string
	}{
		{"", "async"},
		{"ASYNC", "async"},
		{"semi_sync", "semi_sync"},
		{"  sync  ", "sync"},
	}

	for _, tc := range testCases {
		t.Run(tc.mode, func(t *testing.T) {
			normalized := normalizeReplicationLabel(tc.mode)
			assert.Equal(t, tc.expectedMode, normalized)
		})
	}
}
