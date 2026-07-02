package repositories

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newInstanceRepoWithHost 创建一个共享 db 的 host+instance repo, 预先 insert 一台 host.
// instance.host_id 是外键, 没有 host 行时 insert 会失败.
func newInstanceRepoWithHost(t *testing.T) (*InstanceRepository, *HostRepository) {
	t.Helper()
	db := newRepoTestDB(t)
	hostRepo := NewHostRepository(db)
	instRepo := NewInstanceRepository(db)
	require.NoError(t, hostRepo.Create(context.Background(), &models.Host{ID: "host-1", Name: "test-host-1", Address: "127.0.0.1"}))
	require.NoError(t, hostRepo.Create(context.Background(), &models.Host{ID: "host-2", Name: "test-host-2", Address: "127.0.0.1"}))
	return instRepo, hostRepo
}

func TestNewInstanceRepository(t *testing.T) {
	repo := NewInstanceRepository(newRepoTestDB(t))
	assert.NotNil(t, repo)
	assert.NotNil(t, repo.db)
}

func TestInstanceRepository_Create_And_GetByID(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)

	hostID := "host-1"
	inst := &models.Instance{
		ID:        "inst-1",
		Name:      "my-inst",
		HostID:    &hostID,
		ClusterID: "cluster-A",
	}
	require.NoError(t, repo.Create(ctx, inst))
	assert.NotEmpty(t, inst.ID)
	assert.False(t, inst.CreatedAt.IsZero())

	got, err := repo.GetByID(ctx, "inst-1")
	require.NoError(t, err)
	assert.Equal(t, "my-inst", got.Name)
	assert.Equal(t, "cluster-A", got.ClusterID)
	assert.NotNil(t, got.HostID)
	assert.Equal(t, hostID, *got.HostID)
}

func TestInstanceRepository_Create_DuplicateName(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)

	inst1 := &models.Instance{ID: "i-1", Name: "dup"}
	inst2 := &models.Instance{ID: "i-2", Name: "dup"}

	require.NoError(t, repo.Create(ctx, inst1))
	err := repo.Create(ctx, inst2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestInstanceRepository_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)
	_, err := repo.GetByID(ctx, "missing")
	assert.Error(t, err)
}

func TestInstanceRepository_List_Empty(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)

	items, err := repo.List(ctx, 10, 0)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestInstanceRepository_List_OrderAndLimit(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)

	hostID := "host-1"
	for _, id := range []string{"a", "b", "c"} {
		require.NoError(t, repo.Create(ctx, &models.Instance{ID: id, Name: id, HostID: &hostID}))
	}

	items, err := repo.List(ctx, 2, 0)
	assert.NoError(t, err)
	assert.Len(t, items, 2)

	// offset past end
	items, err = repo.List(ctx, 10, 100)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

func TestInstanceRepository_ListByHostID(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)

	h1 := "host-1"
	h2 := "host-2"
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "a", Name: "a", HostID: &h1}))
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "b", Name: "b", HostID: &h1}))
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "c", Name: "c", HostID: &h2}))

	items, err := repo.ListByHostID(ctx, "host-1", 10, 0)
	assert.NoError(t, err)
	assert.Len(t, items, 2)

	items, err = repo.ListByHostID(ctx, "host-2", 10, 0)
	assert.NoError(t, err)
	assert.Len(t, items, 1)

	items, err = repo.ListByHostID(ctx, "host-x", 10, 0)
	assert.NoError(t, err)
	assert.Empty(t, items)
}

// --- TDD: ListByClusterID ---
func TestInstanceRepository_ListByClusterID(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)

	hostID := "host-1"
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "a", Name: "a", HostID: &hostID, ClusterID: "c1"}))
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "b", Name: "b", HostID: &hostID, ClusterID: "c1"}))
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "c", Name: "c", HostID: &hostID, ClusterID: "c2"}))

	got, err := repo.ListByClusterID(ctx, "c1")
	assert.NoError(t, err)
	assert.Len(t, got, 2)
	for _, inst := range got {
		assert.Equal(t, "c1", inst.ClusterID)
	}
}

func TestInstanceRepository_ListByClusterID_Empty(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)
	got, err := repo.ListByClusterID(ctx, "nope")
	assert.NoError(t, err)
	assert.Empty(t, got)
}

func TestInstanceRepository_ListByClusterID_NilHostIDAllowed(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "x", Name: "x", ClusterID: "c1"}))

	got, err := repo.ListByClusterID(ctx, "c1")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
}

func TestInstanceRepository_Update(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)
	hostID := "host-1"
	newHost := "host-2"
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "i-1", Name: "i-1", HostID: &hostID, ClusterID: "c1"}))

	updated := &models.Instance{ID: "i-1", Name: "renamed", ClusterID: "c2", HostID: &newHost}
	require.NoError(t, repo.Update(ctx, updated))

	got, err := repo.GetByID(ctx, "i-1")
	require.NoError(t, err)
	assert.Equal(t, "renamed", got.Name)
	assert.Equal(t, "c2", got.ClusterID)
	assert.NotNil(t, got.HostID)
	assert.Equal(t, "host-2", *got.HostID)
}

func TestInstanceRepository_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)
	err := repo.Update(ctx, &models.Instance{ID: "missing", Name: "x"})
	assert.Error(t, err)
}

func TestInstanceRepository_Delete(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "i-1", Name: "i-1"}))
	require.NoError(t, repo.Delete(ctx, "i-1"))

	_, err := repo.GetByID(ctx, "i-1")
	assert.Error(t, err)
}

func TestInstanceRepository_CreateConnection(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)
	hostID := "host-1"
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "inst-1", Name: "inst-1", HostID: &hostID}))
	conn := &models.InstanceConnection{
		InstanceID: "inst-1",
		Host:       "db.example.com",
		Port:       3306,
		Username:   "root",
		PasswordEncrypted: "enc-pass",
		SSLEnabled: false,
	}
	require.NoError(t, repo.CreateConnection(ctx, conn))
	assert.NotEmpty(t, conn.ID)

	got, err := repo.GetConnection(ctx, "inst-1")
	require.NoError(t, err)
	assert.Equal(t, "db.example.com", got.Host)
	assert.Equal(t, 3306, got.Port)
}

func TestInstanceRepository_GetConnection_NotFound(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)
	_, err := repo.GetConnection(ctx, "missing")
	assert.Error(t, err)
}

func TestInstanceRepository_CreateVersion(t *testing.T) {
	ctx := context.Background()
	repo, _ := newInstanceRepoWithHost(t)
	hostID := "host-1"
	require.NoError(t, repo.Create(ctx, &models.Instance{ID: "inst-1", Name: "inst-1", HostID: &hostID}))
	v := &models.InstanceVersion{
		InstanceID: "inst-1",
		Flavor:     "mysql",
		Version:    "8.0.32",
		FullVersion: "8.0.32-debug",
		IsLTS:      true,
	}
	require.NoError(t, repo.CreateVersion(ctx, v))
	assert.NotEmpty(t, v.ID)
}
