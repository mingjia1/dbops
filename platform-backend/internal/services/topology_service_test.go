package services

import (
	"context"
	"errors"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
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