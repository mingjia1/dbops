package repositories

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostRepositoryCreate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewHostRepository(db)
	ctx := context.Background()

	host := &models.Host{
		Name:    "test-host",
		Address: "10.0.0.1",
		SSHPort: 22,
		SSHUser: "root",
		AgentPort: 9090,
	}
	err := repo.Create(ctx, host)
	require.NoError(t, err)
	assert.NotEmpty(t, host.ID)
}

func TestHostRepositoryGetByID(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewHostRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.Host{Name: "host1", Address: "10.0.0.1", AgentPort: 9090})
	list, _ := repo.List(ctx, 10, 0)
	require.Len(t, list, 1)

	got, err := repo.GetByID(ctx, list[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "host1", got.Name)
}

func TestHostRepositoryList(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewHostRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.Host{Name: "h1", Address: "10.0.0.1", AgentPort: 9090})
	_ = repo.Create(ctx, &models.Host{Name: "h2", Address: "10.0.0.2", AgentPort: 9091})

	hosts, err := repo.List(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, hosts, 2)
}

func TestHostRepositoryUpdate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewHostRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.Host{Name: "old", Address: "10.0.0.1", AgentPort: 9090})
	list, _ := repo.List(ctx, 10, 0)

	host := &list[0]
	host.Name = "new"
	err := repo.Update(ctx, host)
	require.NoError(t, err)

	got, _ := repo.GetByID(ctx, host.ID)
	assert.Equal(t, "new", got.Name)
}

func TestHostRepositoryDelete(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewHostRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.Host{Name: "h1", Address: "10.0.0.1", AgentPort: 9090})
	list, _ := repo.List(ctx, 10, 0)

	err := repo.Delete(ctx, list[0].ID)
	require.NoError(t, err)

	hosts, _ := repo.List(ctx, 10, 0)
	assert.Len(t, hosts, 0)
}
