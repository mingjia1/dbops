package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLicenseRepositorySave(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewLicenseRepository(db)
	ctx := context.Background()

	lic := &models.License{
		ID:         "lic-1",
		Tier:       "ee",
		LicenseKey: "key-123",
		IssuedTo:   "test-org",
		IssuedAt:   time.Now(),
		ExpiresAt:  time.Now().Add(365 * 24 * time.Hour),
		MaxNodes:   50,
		Active:     true,
		CreatedAt:  time.Now(),
	}
	err := repo.Save(ctx, lic)
	require.NoError(t, err)
}

func TestLicenseRepositoryGetActive(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewLicenseRepository(db)
	ctx := context.Background()

	_ = repo.Save(ctx, &models.License{
		ID: "lic-1", Tier: "ce", LicenseKey: "k1", IssuedTo: "u1",
		IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), Active: true, CreatedAt: time.Now(),
	})

	got, err := repo.GetActive(ctx)
	require.NoError(t, err)
	assert.Equal(t, "ce", got.Tier)
}

func TestLicenseRepositoryGetActiveNone(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewLicenseRepository(db)

	got, err := repo.GetActive(context.Background())
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestLicenseRepositoryDeactivateAll(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewLicenseRepository(db)
	ctx := context.Background()

	_ = repo.Save(ctx, &models.License{
		ID: "lic-1", Tier: "ce", LicenseKey: "k1", IssuedTo: "u1",
		IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), Active: true, CreatedAt: time.Now(),
	})
	_ = repo.Save(ctx, &models.License{
		ID: "lic-2", Tier: "ee", LicenseKey: "k2", IssuedTo: "u2",
		IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), Active: true, CreatedAt: time.Now(),
	})

	err := repo.DeactivateAll(ctx)
	require.NoError(t, err)

	got, err := repo.GetActive(ctx)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestLicenseRepositoryList(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewLicenseRepository(db)
	ctx := context.Background()

	_ = repo.Save(ctx, &models.License{
		ID: "lic-1", Tier: "ce", LicenseKey: "k1", IssuedTo: "u1",
		IssuedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), Active: true, CreatedAt: time.Now(),
	})

	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
}
