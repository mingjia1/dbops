package services

import (
	"context"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/mock"
)

type SwitchServiceInterface interface {
	SingleToMHA(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error)
	SingleToMGR(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error)
	SingleToPXC(ctx context.Context, req SwitchClusterRequest) (*SwitchClusterResult, error)
	SwitchRoleWithinCluster(ctx context.Context, req RoleSwitchRequest) (*RoleSwitchResult, error)
	ListRoleSwitchHistory(ctx context.Context, clusterID string, limit int) ([]RoleSwitchRecord, error)
}

type HostServiceInterface interface {
	Create(ctx context.Context, host *models.Host) error
	GetByID(ctx context.Context, id string) (*models.Host, error)
	List(ctx context.Context, limit, offset int) ([]*models.Host, error)
	Update(ctx context.Context, host *models.Host) error
	Delete(ctx context.Context, id string) error
}

type InstanceServiceInterface interface {
	Create(ctx context.Context, inst *models.Instance) error
	GetByID(ctx context.Context, id string) (*models.Instance, error)
	List(ctx context.Context, limit, offset int) ([]*models.Instance, error)
	Update(ctx context.Context, inst *models.Instance) error
	Delete(ctx context.Context, id string) error
}

type HostRepositoryInterface interface {
	Create(ctx context.Context, host *models.Host) error
	GetByID(ctx context.Context, id string) (*models.Host, error)
	List(ctx context.Context, limit, offset int) ([]*models.Host, error)
	Update(ctx context.Context, host *models.Host) error
	Delete(ctx context.Context, id string) error
}

type InstanceRepositoryInterface interface {
	Create(ctx context.Context, inst *models.Instance) error
	GetByID(ctx context.Context, id string) (*models.Instance, error)
	List(ctx context.Context, limit, offset int) ([]models.Instance, error)
	Update(ctx context.Context, inst *models.Instance) error
	Delete(ctx context.Context, id string) error
	GetConnection(ctx context.Context, instanceID string) (*models.InstanceConnection, error)
	CreateConnection(ctx context.Context, conn *models.InstanceConnection) error
	UpdateConnection(ctx context.Context, conn *models.InstanceConnection) error
	UpdateConnectionPassword(ctx context.Context, instanceID, encryptedPassword string) error
	UpsertStatus(ctx context.Context, instanceID string, status *models.InstanceStatus) error
	UpsertTopology(ctx context.Context, instanceID string, topology *models.InstanceTopology) error
	ListByHostID(ctx context.Context, hostID string, limit, offset int) ([]models.Instance, error)
	ListByClusterID(ctx context.Context, clusterID string) ([]*models.Instance, error)
}

// MockInstanceRepo is a mock implementation for testing
type MockInstanceRepo struct {
	mock.Mock
}

func (m *MockInstanceRepo) Create(ctx context.Context, inst *models.Instance) error {
	args := m.Called(ctx, inst)
	return args.Error(0)
}

func (m *MockInstanceRepo) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *MockInstanceRepo) List(ctx context.Context, limit, offset int) ([]models.Instance, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]models.Instance), args.Error(1)
}

func (m *MockInstanceRepo) Update(ctx context.Context, inst *models.Instance) error {
	args := m.Called(ctx, inst)
	return args.Error(0)
}

func (m *MockInstanceRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockInstanceRepo) GetConnection(ctx context.Context, instanceID string) (*models.InstanceConnection, error) {
	args := m.Called(ctx, instanceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.InstanceConnection), args.Error(1)
}

func (m *MockInstanceRepo) CreateConnection(ctx context.Context, conn *models.InstanceConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockInstanceRepo) UpdateConnection(ctx context.Context, conn *models.InstanceConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockInstanceRepo) UpdateConnectionPassword(ctx context.Context, instanceID, encryptedPassword string) error {
	args := m.Called(ctx, instanceID, encryptedPassword)
	return args.Error(0)
}

func (m *MockInstanceRepo) UpsertStatus(ctx context.Context, instanceID string, status *models.InstanceStatus) error {
	args := m.Called(ctx, instanceID, status)
	return args.Error(0)
}

func (m *MockInstanceRepo) UpsertTopology(ctx context.Context, instanceID string, topology *models.InstanceTopology) error {
	args := m.Called(ctx, instanceID, topology)
	return args.Error(0)
}

func (m *MockInstanceRepo) ListByHostID(ctx context.Context, hostID string, limit, offset int) ([]models.Instance, error) {
	args := m.Called(ctx, hostID, limit, offset)
	return args.Get(0).([]models.Instance), args.Error(1)
}

func (m *MockInstanceRepo) ListByClusterID(ctx context.Context, clusterID string) ([]*models.Instance, error) {
	args := m.Called(ctx, clusterID)
	return args.Get(0).([]*models.Instance), args.Error(1)
}
