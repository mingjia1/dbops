package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiagnosisRepositoryCreate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewDiagnosisRepository(db)
	ctx := context.Background()

	d := &models.DiagnosisRecord{
		InstanceID: "inst-1",
		Status:     "healthy",
		Summary:    "All good",
		Score:      100,
		CreatedAt:  time.Now(),
	}
	err := repo.Create(ctx, d)
	require.NoError(t, err)
	assert.NotEmpty(t, d.ID)
}

func TestDiagnosisRepositoryGetByID(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewDiagnosisRepository(db)
	ctx := context.Background()

	d := &models.DiagnosisRecord{
		InstanceID: "inst-1", Status: "ok", Score: 100, CreatedAt: time.Now(),
	}
	require.NoError(t, repo.Create(ctx, d))

	got, err := repo.GetByID(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, 100, got.Score)
}

func TestDiagnosisRepositoryGetByIDNotFound(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewDiagnosisRepository(db)

	_, err := repo.GetByID(context.Background(), "missing")
	require.Error(t, err)
}

func TestDiagnosisRepositoryList(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewDiagnosisRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.DiagnosisRecord{InstanceID: "inst-1", Score: 100, CreatedAt: time.Now()})
	_ = repo.Create(ctx, &models.DiagnosisRecord{InstanceID: "inst-1", Score: 80, CreatedAt: time.Now()})
	_ = repo.Create(ctx, &models.DiagnosisRecord{InstanceID: "inst-2", Score: 60, CreatedAt: time.Now()})

	records, err := repo.List(ctx, "inst-1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, records, 2)
}

func TestSQLAdviceRepositoryCreate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewSQLAdviceRepository(db)
	ctx := context.Background()

	a := &models.SQLAdvice{
		SQLText:   "SELECT * FROM users",
		Advice:    "Add index",
		Score:     85,
		CreatedAt: time.Now(),
	}
	err := repo.Create(ctx, a)
	require.NoError(t, err)
	assert.NotEmpty(t, a.ID)
}

func TestSQLAdviceRepositoryGetByID(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewSQLAdviceRepository(db)
	ctx := context.Background()

	a := &models.SQLAdvice{
		SQLText: "SELECT 1", Advice: "ok", Score: 100, CreatedAt: time.Now(),
	}
	require.NoError(t, repo.Create(ctx, a))

	got, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, 100, got.Score)
}

func TestSQLAdviceRepositoryGetByIDNotFound(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewSQLAdviceRepository(db)

	_, err := repo.GetByID(context.Background(), "missing")
	require.Error(t, err)
}

func TestSQLAdviceRepositoryList(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewSQLAdviceRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.SQLAdvice{SQLText: "s1", Score: 100, CreatedAt: time.Now()})
	_ = repo.Create(ctx, &models.SQLAdvice{SQLText: "s2", Score: 80, CreatedAt: time.Now()})

	list, err := repo.List(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
