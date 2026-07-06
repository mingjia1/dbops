package repositories

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaskingRepositoryCreate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewMaskingRepository(db)
	ctx := context.Background()

	rule := &models.MaskingRule{
		ID:        "rule-1",
		Name:      "mask-email",
		Algorithm: models.MaskingMask,
		Enabled:   true,
	}
	err := repo.Create(ctx, rule)
	require.NoError(t, err)
}

func TestMaskingRepositoryGetByID(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewMaskingRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.MaskingRule{ID: "r1", Name: "rule1", Algorithm: models.MaskingMD5, Enabled: true})

	got, err := repo.GetByID(ctx, "r1")
	require.NoError(t, err)
	assert.Equal(t, "rule1", got.Name)
}

func TestMaskingRepositoryList(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewMaskingRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.MaskingRule{ID: "r1", Name: "r1", Algorithm: models.MaskingMD5, Enabled: true})
	_ = repo.Create(ctx, &models.MaskingRule{ID: "r2", Name: "r2", Algorithm: models.MaskingMask, Enabled: false})

	rules, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, rules, 2)
}

func TestMaskingRepositoryListEnabled(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewMaskingRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.MaskingRule{ID: "r1", Name: "r1", Algorithm: models.MaskingMD5, Enabled: true})
	_ = repo.Create(ctx, &models.MaskingRule{ID: "r2", Name: "r2", Algorithm: models.MaskingMask, Enabled: false})
	_ = repo.Create(ctx, &models.MaskingRule{ID: "r3", Name: "r3", Algorithm: models.MaskingReplace, Enabled: true})

	rules, err := repo.ListEnabled(ctx)
	require.NoError(t, err)
	assert.Len(t, rules, 2)
}

func TestMaskingRepositoryUpdate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewMaskingRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.MaskingRule{ID: "r1", Name: "old", Algorithm: models.MaskingMD5})

	rule, _ := repo.GetByID(ctx, "r1")
	rule.Name = "new"
	err := repo.Update(ctx, rule)
	require.NoError(t, err)

	got, _ := repo.GetByID(ctx, "r1")
	assert.Equal(t, "new", got.Name)
}

func TestMaskingRepositoryDelete(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewMaskingRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.MaskingRule{ID: "r1", Name: "rule1"})
	err := repo.Delete(ctx, "r1")
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, "r1")
	require.Error(t, err)
}
