package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFaultTemplateRepositoryCreate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewFaultTemplateRepository(db)
	ctx := context.Background()

	tpl := &models.FaultTemplate{
		Name:        "Network Partition",
		Category:    "network",
		FaultType:   "network_partition",
		DurationSec: 60,
		Severity:    "high",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	err := repo.Create(ctx, tpl)
	require.NoError(t, err)
	assert.NotEmpty(t, tpl.ID)
}

func TestFaultTemplateRepositoryGetByID(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewFaultTemplateRepository(db)
	ctx := context.Background()

	tpl := &models.FaultTemplate{Name: "Test", Category: "net", FaultType: "network_partition", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.Create(ctx, tpl))

	got, err := repo.GetByID(ctx, tpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test", got.Name)
}

func TestFaultTemplateRepositoryGetByIDNotFound(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewFaultTemplateRepository(db)

	_, err := repo.GetByID(context.Background(), "missing")
	require.Error(t, err)
}

func TestFaultTemplateRepositoryList(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewFaultTemplateRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.FaultTemplate{Name: "T1", Category: "network", FaultType: "f1", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	_ = repo.Create(ctx, &models.FaultTemplate{Name: "T2", Category: "disk", FaultType: "f2", CreatedAt: time.Now(), UpdatedAt: time.Now()})

	tpls, err := repo.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, tpls, 2)

	tpls, err = repo.List(ctx, "network")
	require.NoError(t, err)
	assert.Len(t, tpls, 1)
}

func TestFaultTemplateRepositoryDelete(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewFaultTemplateRepository(db)
	ctx := context.Background()

	tpl := &models.FaultTemplate{Name: "T", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.Create(ctx, tpl))

	err := repo.Delete(ctx, tpl.ID)
	require.NoError(t, err)
}

func TestFaultExecutionRepositoryCreate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewFaultExecutionRepository(db)
	ctx := context.Background()

	exec := &models.FaultExecution{
		TemplateID: "t1",
		DrillID:    "d1",
		Status:     "active",
		StartedAt:  time.Now(),
		CreatedAt:  time.Now(),
	}
	err := repo.Create(ctx, exec)
	require.NoError(t, err)
	assert.NotEmpty(t, exec.ID)
}

func TestFaultExecutionRepositoryGetByID(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewFaultExecutionRepository(db)
	ctx := context.Background()

	exec := &models.FaultExecution{TemplateID: "t1", Status: "active", StartedAt: time.Now(), CreatedAt: time.Now()}
	require.NoError(t, repo.Create(ctx, exec))

	got, err := repo.GetByID(ctx, exec.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", got.Status)
}

func TestFaultExecutionRepositoryGetByIDNotFound(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewFaultExecutionRepository(db)

	_, err := repo.GetByID(context.Background(), "missing")
	require.Error(t, err)
}

func TestFaultExecutionRepositoryUpdate(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewFaultExecutionRepository(db)
	ctx := context.Background()

	exec := &models.FaultExecution{TemplateID: "t1", Status: "injecting", StartedAt: time.Now(), CreatedAt: time.Now()}
	require.NoError(t, repo.Create(ctx, exec))

	got, _ := repo.GetByID(ctx, exec.ID)
	got.Status = "active"
	err := repo.Update(ctx, got)
	require.NoError(t, err)

	updated, _ := repo.GetByID(ctx, exec.ID)
	assert.Equal(t, "active", updated.Status)
}

func TestFaultExecutionRepositoryList(t *testing.T) {
	db := newRepoTestDB(t)
	repo := NewFaultExecutionRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.FaultExecution{DrillID: "d1", Status: "active", StartedAt: time.Now(), CreatedAt: time.Now()})
	_ = repo.Create(ctx, &models.FaultExecution{DrillID: "d1", Status: "active", StartedAt: time.Now(), CreatedAt: time.Now()})
	_ = repo.Create(ctx, &models.FaultExecution{DrillID: "d2", Status: "active", StartedAt: time.Now(), CreatedAt: time.Now()})

	execs, err := repo.List(ctx, "d1")
	require.NoError(t, err)
	assert.Len(t, execs, 2)
}
