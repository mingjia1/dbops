package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFaultService(t *testing.T) *FaultService {
	db := newTestDB(t)
	return NewFaultService(
		repositories.NewFaultTemplateRepository(db),
		repositories.NewFaultExecutionRepository(db),
	)
}

func TestFaultServiceCreateTemplate(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{
		Name:        "Network Partition",
		Category:    "network",
		FaultType:   "network_partition",
		DurationSec: 60,
		Severity:    "high",
	}
	err := svc.CreateTemplate(ctx, tpl)
	require.NoError(t, err)
	assert.NotEmpty(t, tpl.ID)
	assert.NotZero(t, tpl.CreatedAt)
}

func TestFaultServiceListTemplates(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	_ = svc.CreateTemplate(ctx, &models.FaultTemplate{Name: "Net", Category: "network", FaultType: "network_partition"})
	_ = svc.CreateTemplate(ctx, &models.FaultTemplate{Name: "Disk", Category: "disk", FaultType: "disk_full"})

	tpls, err := svc.ListTemplates(ctx, "")
	require.NoError(t, err)
	assert.Len(t, tpls, 2)

	tpls, err = svc.ListTemplates(ctx, "network")
	require.NoError(t, err)
	assert.Len(t, tpls, 1)
}

func TestFaultServiceGetTemplate(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{Name: "Test", FaultType: "network_partition"}
	_ = svc.CreateTemplate(ctx, tpl)

	got, err := svc.GetTemplate(ctx, tpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test", got.Name)
}

func TestFaultServiceDeleteTemplate(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{Name: "Test"}
	_ = svc.CreateTemplate(ctx, tpl)
	err := svc.DeleteTemplate(ctx, tpl.ID)
	require.NoError(t, err)
}

func TestFaultServiceExecuteFault(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{
		Name:      "Network Partition",
		FaultType: "network_partition",
		Params:    `{"block_inbound": true}`,
	}
	_ = svc.CreateTemplate(ctx, tpl)

	exec, err := svc.ExecuteFault(ctx, ExecuteFaultRequest{
		TemplateID: tpl.ID,
		DrillID:    "drill-1",
		TargetType: "instance",
		TargetID:   "inst-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "active", exec.Status)
	assert.Equal(t, "network_partition", exec.FaultType)
	assert.Equal(t, tpl.ID, exec.TemplateID)
	assert.Equal(t, "drill-1", exec.DrillID)
	assert.Equal(t, "instance", exec.TargetType)
	assert.Equal(t, "inst-1", exec.TargetID)
}

func TestFaultServiceExecuteFaultNetworkPartition(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{FaultType: "network_partition", Params: `{"block_inbound": true}`}
	_ = svc.CreateTemplate(ctx, tpl)

	exec, err := svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID})
	require.NoError(t, err)
	assert.Equal(t, tpl.Params, exec.Params)
}

func TestFaultServiceExecuteFaultNodeCrash(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{FaultType: "node_crash", Params: `{"kill_mysql_process": true}`}
	_ = svc.CreateTemplate(ctx, tpl)

	exec, err := svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID})
	require.NoError(t, err)
	assert.Equal(t, tpl.Params, exec.Params)
}

func TestFaultServiceExecuteFaultDiskFull(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{FaultType: "disk_full", Params: `{"fill_percent": 90}`}
	_ = svc.CreateTemplate(ctx, tpl)

	exec, err := svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID})
	require.NoError(t, err)
	assert.Equal(t, tpl.Params, exec.Params)
}

func TestFaultServiceExecuteFaultIOHang(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{FaultType: "io_hang", Params: `{"delay_ms": 1000}`}
	_ = svc.CreateTemplate(ctx, tpl)

	exec, err := svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID})
	require.NoError(t, err)
	assert.Equal(t, tpl.Params, exec.Params)
}

func TestFaultServiceExecuteFaultConnectionExhaust(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{FaultType: "connection_exhaust", Params: `{"max_connections": 100}`}
	_ = svc.CreateTemplate(ctx, tpl)

	exec, err := svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID})
	require.NoError(t, err)
	assert.Equal(t, tpl.Params, exec.Params)
}

func TestFaultServiceExecuteFaultSlowQueryFlood(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{FaultType: "slow_query_flood", Params: `{"concurrent_queries": 50}`}
	_ = svc.CreateTemplate(ctx, tpl)

	exec, err := svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID})
	require.NoError(t, err)
	assert.Equal(t, tpl.Params, exec.Params)
}

func TestFaultServiceExecuteFaultUnknownType(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{FaultType: "unknown_type"}
	_ = svc.CreateTemplate(ctx, tpl)

	exec, err := svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID})
	require.NoError(t, err)
	assert.Equal(t, "active", exec.Status)
}

func TestFaultServiceExecuteFaultTemplateNotFound(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	_, err := svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: "missing"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFaultServiceRollbackFault(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{FaultType: "network_partition"}
	_ = svc.CreateTemplate(ctx, tpl)
	exec, err := svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID})
	require.NoError(t, err)

	rolled, err := svc.RollbackFault(ctx, exec.ID)
	require.NoError(t, err)
	assert.Equal(t, "rolled_back", rolled.Status)
	assert.NotNil(t, rolled.RollbackAt)
}

func TestFaultServiceListExecutions(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{FaultType: "network_partition"}
	_ = svc.CreateTemplate(ctx, tpl)
	_, _ = svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID, DrillID: "d1"})
	_, _ = svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID, DrillID: "d1"})
	_, _ = svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID, DrillID: "d2"})

	execs, err := svc.ListExecutions(ctx, "d1")
	require.NoError(t, err)
	assert.Len(t, execs, 2)
}

func TestFaultServiceGetExecution(t *testing.T) {
	svc := newTestFaultService(t)
	ctx := context.Background()

	tpl := &models.FaultTemplate{FaultType: "network_partition"}
	_ = svc.CreateTemplate(ctx, tpl)
	exec, _ := svc.ExecuteFault(ctx, ExecuteFaultRequest{TemplateID: tpl.ID})

	fetched, err := svc.GetExecution(ctx, exec.ID)
	require.NoError(t, err)
	assert.Equal(t, exec.ID, fetched.ID)
}
