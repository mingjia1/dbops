package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyVersionRepositoryCreate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewKeyVersionRepository(db)
	ctx := context.Background()

	kv := &models.KeyVersion{
		ID:        "kv-1",
		KeyDigest: "abc123",
		Version:   1,
		CreatedAt: time.Now(),
		Note:      "initial",
	}
	err := repo.Create(ctx, kv)
	require.NoError(t, err)
}

func TestKeyVersionRepositoryGetLatest(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewKeyVersionRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.KeyVersion{ID: "kv-1", KeyDigest: "a", Version: 1, CreatedAt: time.Now()})
	_ = repo.Create(ctx, &models.KeyVersion{ID: "kv-2", KeyDigest: "b", Version: 2, CreatedAt: time.Now()})

	latest, err := repo.GetLatest(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, latest.Version)
}

func TestKeyVersionRepositoryGetLatestEmpty(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewKeyVersionRepository(db)

	kv, err := repo.GetLatest(context.Background())
	require.NoError(t, err)
	assert.Nil(t, kv)
}

func TestKeyVersionRepositoryGetMaxVersion(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewKeyVersionRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.KeyVersion{ID: "kv-1", KeyDigest: "a", Version: 3, CreatedAt: time.Now()})

	max, err := repo.GetMaxVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, max)
}

func TestKeyVersionRepositoryGetMaxVersionEmpty(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewKeyVersionRepository(db)

	max, err := repo.GetMaxVersion(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, max)
}

func TestKeyVersionRepositoryList(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewKeyVersionRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.KeyVersion{ID: "kv-1", KeyDigest: "a", Version: 1, CreatedAt: time.Now()})
	_ = repo.Create(ctx, &models.KeyVersion{ID: "kv-2", KeyDigest: "b", Version: 2, CreatedAt: time.Now()})

	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestKeyVersionRepositoryGetAllEncryptedData(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewKeyVersionRepository(db)
	ctx := context.Background()

	// Insert a host with encrypted credential
	hostRepo := NewHostRepository(db)
	_ = hostRepo.Create(ctx, &models.Host{
		Name: "h1", Address: "10.0.0.1", SSHUser: "root",
		SSHCredential: "encrypted-data", AgentPort: 9090,
	})

	records, err := repo.GetAllEncryptedData(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, records)
}

func TestKeyVersionRepositoryUpdateEncryptedValue(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewKeyVersionRepository(db)
	ctx := context.Background()

	// Insert a host
	hostRepo := NewHostRepository(db)
	_ = hostRepo.Create(ctx, &models.Host{
		Name: "h1", Address: "10.0.0.1", SSHUser: "root",
		SSHCredential: "old-encrypted", AgentPort: 9090,
	})
	list, _ := hostRepo.List(ctx, 10, 0)
	require.Len(t, list, 1)

	err := repo.UpdateEncryptedValue(ctx, "hosts", list[0].ID, "ssh_credential", "new-encrypted")
	require.NoError(t, err)

	host, _ := hostRepo.GetByID(ctx, list[0].ID)
	assert.Equal(t, "new-encrypted", host.SSHCredential)
}
