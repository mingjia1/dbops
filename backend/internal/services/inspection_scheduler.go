package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
)

// InspectionScheduler: 轻量化巡检后台调度器。
//
// 周期扫描启用中且 schedule(cron) 非空的巡检模板, 到期时对模板 config 中的
// instance_ids 逐个调用 GenerateInspectionReport (含 recipients 通知发送)。
//
// - cron 解析: 5 字段标准 cron (分 时 日 月 周), 支持 * 数字 逗号列表 与 */n 步进
// - 防重: 同一模板同一分钟只触发一次 (内存 map, 进程重启后从当前分钟重新计时)
// - config.instance_ids 未配置的定时模板跳过并记日志, 不影响手动执行
// - 单 worker 串行执行, 报告失败仅记日志不中断调度循环

type InspectionScheduler struct {
	svc  *AlertService
	stop chan struct{}
	stopOnce sync.Once

	mu      sync.Mutex
	lastRun map[string]string // templateID -> "20060102-1504" 上次触发分钟键

	tickInterval time.Duration
}

func NewInspectionScheduler(svc *AlertService) *InspectionScheduler {
	return &InspectionScheduler{
		svc:          svc,
		stop:         make(chan struct{}),
		lastRun:      make(map[string]string),
		tickInterval: 30 * time.Second,
	}
}

func (s *InspectionScheduler) Start() {
	go func() {
		ticker := time.NewTicker(s.tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				return
			case now := <-ticker.C:
				s.runDue(now)
			}
		}
	}()
	log.Println("INFO: inspection scheduler started (tick every 30s)")
}

func (s *InspectionScheduler) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
}

// minuteKey: 调度按分钟粒度判重, 同一模板同一分钟只跑一次
func minuteKey(t time.Time) string {
	return t.Format("20060102-1504")
}

func (s *InspectionScheduler) runDue(now time.Time) {
	if s.svc == nil || s.svc.inspectionTplRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	templates, err := s.svc.ListInspectionTemplates(ctx, "")
	if err != nil {
		log.Printf("WARN: inspection scheduler list templates failed: %v", err)
		return
	}
	for i := range templates {
		tpl := templates[i]
		if !tpl.Enabled || strings.TrimSpace(tpl.Schedule) == "" {
			continue
		}
		expr, err := parseCronExpr(tpl.Schedule)
		if err != nil {
			log.Printf("WARN: inspection scheduler skip template %s: invalid cron %q: %v", tpl.Name, tpl.Schedule, err)
			continue
		}
		if !expr.matches(now) {
			continue
		}
		key := minuteKey(now)
		s.mu.Lock()
		if s.lastRun[tpl.ID] == key {
			s.mu.Unlock()
			continue
		}
		s.lastRun[tpl.ID] = key
		s.mu.Unlock()

		s.runTemplate(ctx, &tpl)
	}
}

func (s *InspectionScheduler) runTemplate(ctx context.Context, tpl *models.InspectionTemplate) {
	instanceIDs := templateInstanceIDs(tpl.Config)
	if len(instanceIDs) == 0 {
		log.Printf("INFO: inspection scheduler: template %q has cron but no config.instance_ids, skipped", tpl.Name)
		return
	}
	for _, instID := range instanceIDs {
		report, err := s.svc.GenerateInspectionReport(ctx, tpl.ID, instID)
		if err != nil {
			log.Printf("WARN: inspection scheduler: generate report for template %q instance %s failed: %v", tpl.Name, instID, err)
			continue
		}
		log.Printf("INFO: inspection scheduler: template %q instance %s -> %s (score %d)", tpl.Name, instID, report.Summary, report.Score)
	}
}

// templateInstanceIDs: 从模板 config JSON 提取 instance_ids 数组; 未配置返回空
func templateInstanceIDs(configJSON string) []string {
	raw := strings.TrimSpace(configJSON)
	if raw == "" {
		return nil
	}
	var cfg struct {
		InstanceIDs []string `json:"instance_ids"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil
	}
	return cfg.InstanceIDs
}

// --- 轻量 5 字段 cron 解析 ---

type cronExpr struct {
	minute, hour, dom, month, dow map[int]bool
	anyDom, anyDow                bool
}

func parseCronExpr(spec string) (*cronExpr, error) {
	fields := strings.Fields(strings.TrimSpace(spec))
	if len(fields) != 5 {
		return nil, fmt.Errorf("expect 5 fields (min hour dom month dow), got %d", len(fields))
	}
	minute, err := parseCronField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseCronField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	dom, err := parseCronField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day-of-month: %w", err)
	}
	month, err := parseCronField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	dow, err := parseCronField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("day-of-week: %w", err)
	}
	return &cronExpr{
		minute: minute, hour: hour, dom: dom, month: month, dow: dow,
		anyDom: fields[2] == "*", anyDow: fields[4] == "*",
	}, nil
}

// parseCronField: 支持 "*", 纯数字, 逗号列表, "*/n" 与 "a/n" 起点步进
func parseCronField(field string, min, max int) (map[int]bool, error) {
	out := map[int]bool{}
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty list item")
		}
		step := 1
		base := min
		body := part
		if idx := strings.Index(part, "/"); idx >= 0 {
			body = part[:idx]
			var err error
			step, err = strconv.Atoi(part[idx+1:])
			if err != nil || step < 1 {
				return nil, fmt.Errorf("invalid step in %q", part)
			}
			if body != "*" {
				lo, hi, err := parseCronRange(body, min, max)
				if err != nil {
					return nil, err
				}
				for v := lo; v <= hi; v += step {
					out[v] = true
				}
				continue
			}
			for v := base; v <= max; v += step {
				out[v] = true
			}
			continue
		}
		if body == "*" {
			// 裸 "*": 全范围
			for v := min; v <= max; v++ {
				out[v] = true
			}
			continue
		}
		if strings.Contains(part, "-") {
			// "a-b" 范围
			lo, hi, err := parseCronRange(part, min, max)
			if err != nil {
				return nil, err
			}
			for v := lo; v <= hi; v++ {
				out[v] = true
			}
			continue
		}
		{
			v, err := strconv.Atoi(body)
			if err != nil {
				return nil, fmt.Errorf("invalid value %q", body)
			}
			if v < min || v > max {
				return nil, fmt.Errorf("value %d out of range [%d,%d]", v, min, max)
			}
			out[v] = true
			continue
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("field matches nothing")
	}
	return out, nil
}

// parseCronRange: 解析 "a-b" 范围, 校验边界
func parseCronRange(body string, min, max int) (int, int, error) {
	parts := strings.SplitN(body, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range %q", body)
	}
	lo, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range start %q", parts[0])
	}
	hi, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range end %q", parts[1])
	}
	if lo > hi || lo < min || hi > max {
		return 0, 0, fmt.Errorf("range %q out of order or out of range [%d,%d]", body, min, max)
	}
	return lo, hi, nil
}

// matches: 标准 cron dom/dow 语义 — 两者都受限时匹配任一
func (e *cronExpr) matches(t time.Time) bool {
	if !e.minute[t.Minute()] || !e.hour[t.Hour()] || !e.month[int(t.Month())] {
		return false
	}
	domHit := e.anyDom || e.dom[t.Day()]
	dowHit := e.anyDow || e.dow[int(t.Weekday())]
	if e.anyDom && e.anyDow {
		return true
	}
	if e.anyDom {
		return dowHit
	}
	if e.anyDow {
		return domHit
	}
	return domHit || dowHit
}
