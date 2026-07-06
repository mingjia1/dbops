package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryCreate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &models.User{
		ID:        "user-1",
		Username:  "alice",
		Password:  "hashed-pw",
		Email:     "alice@test.com",
		Role:      "dba",
		Status:    "active",
		CreatedAt: time.Now(),
	}
	err := repo.Create(ctx, user)
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, "user-1")
	require.NoError(t, err)
	assert.Equal(t, "alice", got.Username)
	assert.Equal(t, "alice@test.com", got.Email)
}

func TestUserRepositoryGetByUsername(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.User{ID: "u1", Username: "alice", Password: "hash", Status: "active", CreatedAt: time.Now()})

	got, err := repo.GetByUsername(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, "u1", got.ID)
}

func TestUserRepositoryGetByUsernameNotFound(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewUserRepository(db)

	got, err := repo.GetByUsername(context.Background(), "missing")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUserRepositoryList(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.User{ID: "u1", Username: "a", Password: "h", Email: "a@test.com", Status: "active", CreatedAt: time.Now()})
	_ = repo.Create(ctx, &models.User{ID: "u2", Username: "b", Password: "h", Email: "b@test.com", Status: "active", CreatedAt: time.Now()})

	users, err := repo.List(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestUserRepositoryUpdate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.User{ID: "u1", Username: "old", Password: "h", Status: "active", CreatedAt: time.Now()})

	user, _ := repo.GetByID(ctx, "u1")
	user.Username = "new"
	err := repo.Update(ctx, user)
	require.NoError(t, err)

	got, _ := repo.GetByID(ctx, "u1")
	assert.Equal(t, "new", got.Username)
}

func TestUserRepositoryUpdatePassword(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.User{ID: "u1", Username: "alice", Password: "old-hash", Status: "active", CreatedAt: time.Now()})

	err := repo.UpdatePassword(ctx, "u1", "new-hash")
	require.NoError(t, err)

	got, _ := repo.GetByID(ctx, "u1")
	assert.Equal(t, "new-hash", got.Password)
}

func TestUserRepositoryUpdatePasswordNotFound(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewUserRepository(db)

	err := repo.UpdatePassword(context.Background(), "missing", "hash")
	require.Error(t, err)
}

func TestUserRepositoryUpdateAllPasswords(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.User{ID: "u1", Username: "a", Password: "h1", Email: "a@test.com", Status: "active", CreatedAt: time.Now()})
	_ = repo.Create(ctx, &models.User{ID: "u2", Username: "b", Password: "h2", Email: "b@test.com", Status: "active", CreatedAt: time.Now()})

	count, err := repo.UpdateAllPasswords(ctx, "new-hash")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestUserRepositoryDelete(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.User{ID: "u1", Username: "alice", Password: "h", Email: "alice@test.com", Status: "active", CreatedAt: time.Now()})

	err := repo.Delete(ctx, "u1")
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, "u1")
	require.NoError(t, err)
	assert.Nil(t, got)
}
