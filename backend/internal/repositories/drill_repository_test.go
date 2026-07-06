package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDrillRepositoryCreate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewDrillRepository(db)
	ctx := context.Background()

	drill := &models.HADrill{
		Name:      "Test Drill",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.Create(ctx, drill)
	require.NoError(t, err)
	assert.NotEmpty(t, drill.ID)
}

func TestDrillRepositoryGetByID(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewDrillRepository(db)
	ctx := context.Background()

	drill := &models.HADrill{Name: "Drill", Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.Create(ctx, drill))

	got, err := repo.GetByID(ctx, drill.ID)
	require.NoError(t, err)
	assert.Equal(t, "Drill", got.Name)
}

func TestDrillRepositoryGetByIDNotFound(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewDrillRepository(db)

	_, err := repo.GetByID(context.Background(), "missing")
	require.Error(t, err)
}

func TestDrillRepositoryList(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewDrillRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.HADrill{Name: "D1", Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	_ = repo.Create(ctx, &models.HADrill{Name: "D2", Status: "completed", CreatedAt: time.Now(), UpdatedAt: time.Now()})

	drills, err := repo.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, drills, 2)

	drills, err = repo.List(ctx, "pending")
	require.NoError(t, err)
	assert.Len(t, drills, 1)
}

func TestDrillRepositoryUpdate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewDrillRepository(db)
	ctx := context.Background()

	drill := &models.HADrill{Name: "Old", Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.Create(ctx, drill))

	got, _ := repo.GetByID(ctx, drill.ID)
	got.Name = "New"
	err := repo.Update(ctx, got)
	require.NoError(t, err)

	updated, _ := repo.GetByID(ctx, drill.ID)
	assert.Equal(t, "New", updated.Name)
}

func TestDrillRepositoryDelete(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewDrillRepository(db)
	ctx := context.Background()

	drill := &models.HADrill{Name: "D", Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.Create(ctx, drill))

	err := repo.Delete(ctx, drill.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, drill.ID)
	require.Error(t, err)
}

func TestDrillReportRepositoryCreate(t *testing.T) {
	db := newRepoTestDB(t)
	drillRepo := NewDrillRepository(db)
	reportRepo := NewDrillReportRepository(db)
	ctx := context.Background()

	// Create a drill first (FK constraint)
	drill := &models.HADrill{Name: "D", Status: "completed", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, drillRepo.Create(ctx, drill))

	report := &models.HADrillReport{
		DrillID:     drill.ID,
		Summary:     "Test summary",
		Score:       85,
		GeneratedAt: time.Now(),
		CreatedAt:   time.Now(),
	}
	err := reportRepo.Create(ctx, report)
	require.NoError(t, err)
	assert.NotEmpty(t, report.ID)
}

func TestDrillReportRepositoryGetByDrillID(t *testing.T) {
	db := newRepoTestDB(t)
	drillRepo := NewDrillRepository(db)
	reportRepo := NewDrillReportRepository(db)
	ctx := context.Background()

	drill := &models.HADrill{Name: "D", Status: "completed", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, drillRepo.Create(ctx, drill))

	report := &models.HADrillReport{
		DrillID: drill.ID, Summary: "Summary", Score: 85, GeneratedAt: time.Now(), CreatedAt: time.Now(),
	}
	require.NoError(t, reportRepo.Create(ctx, report))

	got, err := reportRepo.GetByDrillID(ctx, drill.ID)
	require.NoError(t, err)
	assert.Equal(t, 85, got.Score)
}

func TestDrillReportRepositoryGetByDrillIDNotFound(t *testing.T) {
	db := newRepoTestDB(t)
	reportRepo := NewDrillReportRepository(db)

	got, err := reportRepo.GetByDrillID(context.Background(), "missing-drill-id")
	require.NoError(t, err)
	assert.Nil(t, got)
}
