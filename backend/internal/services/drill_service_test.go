package services

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDrillService(t *testing.T) *DrillService {
	db := newTestDB(t)
	return NewDrillService(
		repositories.NewDrillRepository(db),
		repositories.NewDrillReportRepository(db),
		repositories.NewFaultExecutionRepository(db),
	)
}

func TestDrillServiceCreate(t *testing.T) {
	svc := newTestDrillService(t)
	ctx := context.Background()

	drill := &models.HADrill{
		Name: "Test Drill",
	}
	err := svc.Create(ctx, drill)
	require.NoError(t, err)
	assert.NotEmpty(t, drill.ID)
	assert.NotZero(t, drill.CreatedAt)
}

func TestDrillServiceList(t *testing.T) {
	svc := newTestDrillService(t)
	ctx := context.Background()

	_ = svc.Create(ctx, &models.HADrill{Name: "Drill 1", Status: "pending"})
	_ = svc.Create(ctx, &models.HADrill{Name: "Drill 2", Status: "completed"})

	drills, err := svc.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, drills, 2)

	drills, err = svc.List(ctx, "pending")
	require.NoError(t, err)
	assert.Len(t, drills, 1)
}

func TestDrillServiceGet(t *testing.T) {
	svc := newTestDrillService(t)
	ctx := context.Background()

	d := &models.HADrill{Name: "Drill 1"}
	_ = svc.Create(ctx, d)

	drill, err := svc.Get(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "Drill 1", drill.Name)
}

func TestDrillServiceUpdate(t *testing.T) {
	svc := newTestDrillService(t)
	ctx := context.Background()

	d := &models.HADrill{Name: "Old"}
	_ = svc.Create(ctx, d)

	err := svc.Update(ctx, &models.HADrill{ID: d.ID, Name: "New"})
	require.NoError(t, err)

	drill, err := svc.Get(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "New", drill.Name)
}

func TestDrillServiceDelete(t *testing.T) {
	svc := newTestDrillService(t)
	ctx := context.Background()

	d := &models.HADrill{Name: "Drill"}
	_ = svc.Create(ctx, d)
	err := svc.Delete(ctx, d.ID)
	require.NoError(t, err)
}

func TestDrillServiceStartDrill(t *testing.T) {
	svc := newTestDrillService(t)
	ctx := context.Background()

	d := &models.HADrill{Name: "Drill", Status: "pending"}
	_ = svc.Create(ctx, d)

	drill, err := svc.StartDrill(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "running", drill.Status)
	require.NotNil(t, drill.StartedAt)
}

func TestDrillServiceCompleteDrillNoExecutions(t *testing.T) {
	svc := newTestDrillService(t)
	ctx := context.Background()

	d := &models.HADrill{Name: "Drill", Status: "running"}
	_ = svc.Create(ctx, d)

	drill, report, err := svc.CompleteDrill(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", drill.Status)
	assert.NotNil(t, drill.CompletedAt)
	assert.Equal(t, 0, drill.Score)
	require.NotNil(t, report)
	assert.Equal(t, d.ID, report.DrillID)
	assert.Equal(t, 0, report.Score)
}

func TestDrillServiceCompleteDrillWithExecutions(t *testing.T) {
	db := newTestDB(t)
	drillRepo := repositories.NewDrillRepository(db)
	reportRepo := repositories.NewDrillReportRepository(db)
	execRepo := repositories.NewFaultExecutionRepository(db)
	svc := NewDrillService(drillRepo, reportRepo, execRepo)
	ctx := context.Background()

	d := &models.HADrill{Name: "Drill", Status: "running"}
	_ = svc.Create(ctx, d)

	now := time.Now()
	_ = execRepo.Create(ctx, &models.FaultExecution{
		DrillID:    d.ID,
		FaultType:  "network_partition",
		Status:     "active",
		StartedAt:  now,
		CreatedAt:  now,
	})
	_ = execRepo.Create(ctx, &models.FaultExecution{
		DrillID:    d.ID,
		FaultType:  "node_crash",
		Status:     "rolled_back",
		StartedAt:  now,
		CreatedAt:  now,
	})
	_ = execRepo.Create(ctx, &models.FaultExecution{
		DrillID:    d.ID,
		FaultType:  "disk_full",
		Status:     "failed",
		StartedAt:  now,
		CreatedAt:  now,
	})

	drill, report, err := svc.CompleteDrill(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", drill.Status)
	assert.Equal(t, 66, drill.Score)
	assert.Equal(t, 66, report.Score)
	assert.NotEmpty(t, report.Timeline)
	assert.NotEmpty(t, report.Findings)
	assert.Equal(t, d.ID, report.DrillID)
}

func TestDrillServiceGetReport(t *testing.T) {
	svc := newTestDrillService(t)
	ctx := context.Background()

	d := &models.HADrill{Name: "Drill", Status: "running"}
	_ = svc.Create(ctx, d)
	drill, report, err := svc.CompleteDrill(ctx, d.ID)
	require.NoError(t, err)

	fetched, err := svc.GetReport(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, report.ID, fetched.ID)
	assert.Equal(t, drill.Score, fetched.Score)
}
