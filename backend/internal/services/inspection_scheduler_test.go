package services

import (
	"context"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
)

func TestParseCronExpr(t *testing.T) {
	// 每 30 分钟
	e, err := parseCronExpr("*/30 * * * *")
	assert.NoError(t, err)
	assert.True(t, e.minute[0] && e.minute[30])
	assert.False(t, e.minute[15])
	assert.False(t, e.minute[59])

	// 每天凌晨 2 点
	e, err = parseCronExpr("0 2 * * *")
	assert.NoError(t, err)
	assert.True(t, e.matches(time.Date(2026, 11, 11, 2, 0, 0, 0, time.Local)))
	assert.False(t, e.matches(time.Date(2026, 11, 11, 3, 0, 0, 0, time.Local)))
	assert.False(t, e.matches(time.Date(2026, 11, 11, 2, 0, 0, 0, time.Local).Add(-time.Minute)))

	// 逗号列表 + 范围起点
	e, err = parseCronExpr("5,15,25 8-9 * * *")
	assert.NoError(t, err)
	assert.True(t, e.matches(time.Date(2026, 11, 11, 9, 15, 0, 0, time.Local)))
	assert.False(t, e.matches(time.Date(2026, 11, 11, 10, 15, 0, 0, time.Local)))

	// 周一 09:00 (dow=1)
	e, err = parseCronExpr("0 9 * * 1")
	assert.NoError(t, err)
	monday := time.Date(2026, 11, 9, 9, 0, 0, 0, time.Local) // 2026-11-09 是周一
	assert.True(t, e.matches(monday))
	assert.False(t, e.matches(time.Date(2026, 11, 10, 9, 0, 0, 0, time.Local)))

	// 字段数错误 / 越界
	_, err = parseCronExpr("0 2 * *")
	assert.Error(t, err)
	_, err = parseCronExpr("99 * * * *")
	assert.Error(t, err)
	_, err = parseCronExpr("*/0 * * * *")
	assert.Error(t, err)
}

func TestCronDomDowOrSemantics(t *testing.T) {
	// dom 与 dow 都受限: 匹配任一即触发
	e, err := parseCronExpr("0 9 1 * 1")
	assert.NoError(t, err)
	// 2026-11-02 是周一 (dow 命中, dom 不命中)
	assert.True(t, e.matches(time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local)))
	// 2026-11-01 是月初 (dom 命中, dow 不命中)
	assert.True(t, e.matches(time.Date(2026, 11, 1, 9, 0, 0, 0, time.Local)))
	// 2026-11-04 周三, 都不命中
	assert.False(t, e.matches(time.Date(2026, 11, 4, 9, 0, 0, 0, time.Local)))
}

func TestTemplateInstanceIDs(t *testing.T) {
	assert.Equal(t, []string{"i1", "i2"}, templateInstanceIDs(`{"instance_ids":["i1","i2"]}`))
	assert.Nil(t, templateInstanceIDs(`{}`))
	assert.Nil(t, templateInstanceIDs(``))
	assert.Nil(t, templateInstanceIDs(`not-json`))
}

func TestSchedulerDedupPerMinute(t *testing.T) {
	s := NewInspectionScheduler(nil)
	s.lastRun["tpl-1"] = minuteKey(time.Date(2026, 11, 11, 8, 0, 0, 0, time.Local))
	s.mu.Lock()
	v := s.lastRun["tpl-1"]
	s.mu.Unlock()
	assert.Equal(t, "20261111-0800", v)
	assert.Equal(t, "20261111-0800", minuteKey(time.Date(2026, 11, 11, 8, 0, 30, 0, time.Local)))
	assert.NotEqual(t, "20261111-0800", minuteKey(time.Date(2026, 11, 11, 8, 1, 0, 0, time.Local)))
}

func TestSchedulerRunDueSkipsDisabled(t *testing.T) {
	// svc 为 nil 时 runDue 直接返回, 不 panic
	s := NewInspectionScheduler(nil)
	s.runDue(time.Now())
	assert.Equal(t, 0, len(s.lastRun))
}

func TestSchedulerRunDueExecutesDueTemplate(t *testing.T) {
	// 真实 sqlite: 到期模板 -> runDue -> 报告生成 + 分钟判重
	db := newTestDB(t)
	tplRepo := repositories.NewInspectionTemplateRepository(db)
	rptRepo := repositories.NewInspectionReportRepository(db)
	// monitorService 传 nil: runInspectionChecks 走 "监控指标服务不可用" 降级路径, 报告仍会生成
	svc := NewAlertService(nil, nil, NewMonitorService(nil)).WithInspectionRepos(tplRepo, rptRepo)

	ctx := context.Background()
	tpl := &models.InspectionTemplate{
		Name:     "sched-test",
		Category: "性能",
		Schedule: "* * * * *",
		Config:   `{"instance_ids":["inst-a","inst-b"]}`,
		Enabled:  true,
	}
	assert.NoError(t, svc.CreateInspectionTemplate(ctx, tpl))

	s := NewInspectionScheduler(svc)
	now := time.Date(2026, 11, 11, 8, 0, 0, 0, time.Local)
	s.runDue(now)

	reports, err := rptRepo.List(ctx, tpl.ID, 100, 0)
	assert.NoError(t, err)
	assert.Len(t, reports, 2, "两个 instance 各生成一份报告")
	assert.NotEmpty(t, reports[0].Summary)

	// 同一分钟再次 runDue: 判重, 不重复生成
	s.runDue(now)
	reports, err = rptRepo.List(ctx, tpl.ID, 100, 0)
	assert.NoError(t, err)
	assert.Len(t, reports, 2, "同一分钟内不应重复触发")

	// 下一分钟再触发: 重新生成
	s.runDue(now.Add(time.Minute))
	reports, err = rptRepo.List(ctx, tpl.ID, 100, 0)
	assert.NoError(t, err)
	assert.Len(t, reports, 4, "下一个到期分钟重新生成报告")
}

func TestSchedulerRunDueSkipsConfigWithoutInstances(t *testing.T) {
	db := newTestDB(t)
	tplRepo := repositories.NewInspectionTemplateRepository(db)
	rptRepo := repositories.NewInspectionReportRepository(db)
	svc := NewAlertService(nil, nil, NewMonitorService(nil)).WithInspectionRepos(tplRepo, rptRepo)

	ctx := context.Background()
	tpl := &models.InspectionTemplate{
		Name: "no-instances", Category: "安全", Schedule: "*/5 * * * *", Enabled: true,
	}
	assert.NoError(t, svc.CreateInspectionTemplate(ctx, tpl))

	s := NewInspectionScheduler(svc)
	now := time.Date(2026, 11, 11, 8, 0, 0, 0, time.Local)
	s.runDue(now) // config 无 instance_ids: 跳过不生成, 也不 panic

	reports, err := rptRepo.List(ctx, tpl.ID, 100, 0)
	assert.NoError(t, err)
	assert.Empty(t, reports)
}
