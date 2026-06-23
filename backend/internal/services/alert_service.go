package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/alertexpr"
	"github.com/jackcode/mysql-ops-platform/pkg/notifier"
)

type AlertService struct {
	ruleRepo         *repositories.AlertRuleRepository
	notificationRepo *repositories.AlertNotificationRepository
	monitorService   *MonitorService
	templateRepo     *repositories.AlertTemplateRepository
	escRepo          *repositories.EscalationRepository
	silenceRepo      *repositories.SilenceRepository
	inspectionTplRepo *repositories.InspectionTemplateRepository
	inspectionRptRepo *repositories.InspectionReportRepository
}

func NewAlertService(ruleRepo *repositories.AlertRuleRepository, notificationRepo *repositories.AlertNotificationRepository, monitorService *MonitorService) *AlertService {
	return &AlertService{
		ruleRepo:         ruleRepo,
		notificationRepo: notificationRepo,
		monitorService:   monitorService,
	}
}

func (s *AlertService) WithTemplateRepo(repo *repositories.AlertTemplateRepository) *AlertService {
	s.templateRepo = repo
	return s
}

func (s *AlertService) WithEscalationRepo(repo *repositories.EscalationRepository) *AlertService {
	s.escRepo = repo
	return s
}

func (s *AlertService) WithSilenceRepo(repo *repositories.SilenceRepository) *AlertService {
	s.silenceRepo = repo
	return s
}

func (s *AlertService) WithInspectionRepos(tplRepo *repositories.InspectionTemplateRepository, rptRepo *repositories.InspectionReportRepository) *AlertService {
	s.inspectionTplRepo = tplRepo
	s.inspectionRptRepo = rptRepo
	return s
}

// --- AlertTemplate methods ---

func (s *AlertService) CreateAlertTemplate(ctx context.Context, tpl *models.AlertTemplate) error {
	tpl.CreatedAt = time.Now()
	return s.templateRepo.Create(ctx, tpl)
}

func (s *AlertService) ListAlertTemplates(ctx context.Context, category string) ([]models.AlertTemplate, error) {
	return s.templateRepo.List(ctx, category)
}

func (s *AlertService) GetAlertTemplate(ctx context.Context, id string) (*models.AlertTemplate, error) {
	return s.templateRepo.GetByID(ctx, id)
}

func (s *AlertService) DeleteAlertTemplate(ctx context.Context, id string) error {
	return s.templateRepo.Delete(ctx, id)
}

type EvaluateAlertRuleRequest struct {
	RuleID     string `json:"rule_id" binding:"required"`
	InstanceID string `json:"instance_id" binding:"required"`
}

type AlertEvaluationResult struct {
	RuleID     string    `json:"rule_id"`
	InstanceID string    `json:"instance_id"`
	Triggered  bool      `json:"triggered"`
	Value      float64   `json:"value"`
	Threshold  float64   `json:"threshold"`
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
}

func (s *AlertService) EvaluateAlertRule(ctx context.Context, req EvaluateAlertRuleRequest) (*AlertEvaluationResult, error) {
	rule, err := s.ruleRepo.GetAlertRuleByID(ctx, req.RuleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get alert rule: %w", err)
	}

	metrics, err := s.monitorService.QueryMetrics(ctx, MetricQueryRequest{
		InstanceID: req.InstanceID,
		Metrics:    []string{rule.Metric},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}

	var currentValue float64
	for _, metric := range metrics {
		if metric.Name == rule.Metric {
			currentValue = metric.Value
			break
		}
	}

	var triggered bool
	if rule.Expression != "" {
		metricMap := make(map[string]float64)
		for _, m := range metrics {
			metricMap[m.Name] = m.Value
		}
		e := alertexpr.NewEvaluator(metricMap)
		triggered, err = e.Eval(rule.Expression)
		if err != nil {
			return nil, fmt.Errorf("expression evaluation failed: %w", err)
		}
	} else {
		triggered = s.evaluateCondition(rule.Condition, currentValue, rule.Threshold)
	}

	message := ""
	if triggered {
		if rule.Expression != "" {
			message = fmt.Sprintf("Alert %s triggered: expression(%s)", rule.Name, rule.Expression)
		} else {
			message = fmt.Sprintf("Alert %s triggered: %s %.2f %s %.2f", rule.Name, rule.Metric, currentValue, rule.Condition, rule.Threshold)
		}
	}

	return &AlertEvaluationResult{
		RuleID:     req.RuleID,
		InstanceID: req.InstanceID,
		Triggered:  triggered,
		Value:      currentValue,
		Threshold:  rule.Threshold,
		Message:    message,
		Timestamp:  time.Now(),
	}, nil
}

func (s *AlertService) evaluateCondition(condition string, value, threshold float64) bool {
	switch condition {
	case ">", ">=":
		return value > threshold
	case "<", "<=":
		return value < threshold
	case "==", "=":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

type TriggerAlertRequest struct {
	RuleID     string  `json:"rule_id" binding:"required"`
	InstanceID string  `json:"instance_id" binding:"required"`
	Value      float64 `json:"value" binding:"required"`
	Message    string  `json:"message"`
}

type AlertTriggerResult struct {
	AlertID     string    `json:"alert_id"`
	RuleID      string    `json:"rule_id"`
	InstanceID  string    `json:"instance_id"`
	Status      string    `json:"status"`
	Severity    string    `json:"severity"`
	Message     string    `json:"message"`
	TriggeredAt time.Time `json:"triggered_at"`
}

func (s *AlertService) TriggerAlert(ctx context.Context, req TriggerAlertRequest) (*AlertTriggerResult, error) {
	rule, err := s.ruleRepo.GetAlertRuleByID(ctx, req.RuleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get alert rule: %w", err)
	}

	alertID := uuid.New().String()
	now := time.Now()
	alertRecord := &models.AlertRecord{
		ID:          alertID,
		RuleID:      req.RuleID,
		InstanceID:  req.InstanceID,
		TriggeredAt: now,
		Status:      "firing",
		Severity:    rule.Severity,
		Value:       req.Value,
		Message:     req.Message,
		CreatedAt:   now,
	}

	// P0-4: 真实写库, 不再是 no-op.
	if err := s.ruleRepo.CreateAlertRecord(ctx, alertRecord); err != nil {
		return nil, fmt.Errorf("failed to persist alert record: %w", err)
	}

	return &AlertTriggerResult{
		AlertID:     alertID,
		RuleID:      req.RuleID,
		InstanceID:  req.InstanceID,
		Status:      alertRecord.Status,
		Severity:    alertRecord.Severity,
		Message:     alertRecord.Message,
		TriggeredAt: alertRecord.TriggeredAt,
	}, nil
}

type SendNotificationRequest struct {
	AlertID  string   `json:"alert_id" binding:"required"`
	Channels []string `json:"channels" binding:"required"`
	Message  string   `json:"message" binding:"required"`
}

type NotificationResult struct {
	AlertID      string    `json:"alert_id"`
	Channels     []string  `json:"channels"`
	Status       string    `json:"status"`
	SentAt       time.Time `json:"sent_at"`
	SuccessCount int       `json:"success_count"`
}

// SendNotification P0-4: 真实按 channel.webhook_url 调 HTTP POST, 或 SMTP 发送邮件.
// "channels" 接收 channel id 列表; service 用 notificationRepo 拉取 channel 配置.
func (s *AlertService) SendNotification(ctx context.Context, req SendNotificationRequest) (*NotificationResult, error) {
	sentAt := time.Now()
	successCount := 0
	failedChannels := []string{}

	for _, channelID := range req.Channels {
		ch, err := s.notificationRepo.GetAlertNotificationByID(ctx, channelID)
		if err != nil || ch == nil {
			failedChannels = append(failedChannels, channelID+" (not found)")
			continue
		}
		// 解析 channel_config: {"webhook_url":"...","smtp_host":"...","smtp_user":"...","to":["a@x"]}
		cfg := parseChannelConfig(ch.ChannelConfig)
		switch ch.ChannelType {
		case "webhook":
			url, _ := cfg["webhook_url"].(string)
			if url == "" {
				failedChannels = append(failedChannels, channelID+" (no webhook_url)")
				continue
			}
			payload, _ := json.Marshal(map[string]string{
				"alert_id": req.AlertID,
				"message":  req.Message,
			})
			httpClient := &http.Client{Timeout: 5 * time.Second}
			resp, postErr := httpClient.Post(url, "application/json", bytes.NewReader(payload))
			if postErr != nil || resp.StatusCode >= 400 {
				failedChannels = append(failedChannels, channelID+" (webhook failed)")
				continue
			}
			_ = resp.Body.Close()
			successCount++
		case "email":
			smtpHost, _ := cfg["smtp_host"].(string)
			smtpUser, _ := cfg["smtp_user"].(string)
			smtpPass, _ := cfg["smtp_pass"].(string)
			toList, _ := cfg["to"].([]interface{})
			if smtpHost == "" || smtpUser == "" || len(toList) == 0 {
				failedChannels = append(failedChannels, channelID+" (smtp misconfigured)")
				continue
			}
			from := smtpUser
			addrs := make([]string, 0, len(toList))
			for _, a := range toList {
				if s, ok := a.(string); ok {
					addrs = append(addrs, s)
				}
			}
			msg := []byte("From: " + from + "\r\n" +
				"To: " + addrs[0] + "\r\n" +
				"Subject: [DBOps Alert] " + req.AlertID + "\r\n\r\n" + req.Message)
			auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
			if smtpErr := smtp.SendMail(smtpHost+":25", auth, from, addrs, msg); smtpErr != nil {
				failedChannels = append(failedChannels, channelID+" (smtp send failed)")
				continue
			}
			successCount++
		case "dingtalk":
			webhookURL, _ := cfg["webhook_url"].(string)
			if webhookURL == "" {
				failedChannels = append(failedChannels, channelID+" (webhook_url not configured)")
				continue
			}
			if err := notifier.Send("dingtalk", webhookURL, req.AlertID, req.Message); err != nil {
				failedChannels = append(failedChannels, channelID+" ("+err.Error()+")")
				continue
			}
			successCount++
		case "wecom":
			webhookURL, _ := cfg["webhook_url"].(string)
			if webhookURL == "" {
				failedChannels = append(failedChannels, channelID+" (webhook_url not configured)")
				continue
			}
			if err := notifier.Send("wecom", webhookURL, req.AlertID, req.Message); err != nil {
				failedChannels = append(failedChannels, channelID+" ("+err.Error()+")")
				continue
			}
			successCount++
		default:
			failedChannels = append(failedChannels, channelID+" (unsupported channel_type)")
		}
	}

	status := "sent"
	if successCount == 0 {
		status = "failed"
	} else if successCount < len(req.Channels) {
		status = "partial"
	}

	// 注: 发送明细 (AlertNotification) 由 Agent 端在投递 webhook/smtp 后写回, 这里只返回汇总.

	return &NotificationResult{
		AlertID:      req.AlertID,
		Channels:     req.Channels,
		Status:       status,
		SentAt:       sentAt,
		SuccessCount: successCount,
	}, nil
}

func parseChannelConfig(raw string) map[string]interface{} {
	out := map[string]interface{}{}
	if raw == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

type AlertHistoryFilter struct {
	InstanceID string     `json:"instance_id"`
	RuleID     string     `json:"rule_id"`
	Status     string     `json:"status"`
	StartTime  *time.Time `json:"start_time"`
	EndTime    *time.Time `json:"end_time"`
	Limit      int        `json:"limit"`
	Offset     int        `json:"offset"`
}

type AlertHistoryEntry struct {
	ID          string     `json:"id"`
	RuleID      string     `json:"rule_id"`
	RuleName    string     `json:"rule_name"`
	InstanceID  string     `json:"instance_id"`
	Status      string     `json:"status"`
	Severity    string     `json:"severity"`
	Value       float64    `json:"value"`
	Message     string     `json:"message"`
	TriggeredAt time.Time  `json:"triggered_at"`
	ResolvedAt  *time.Time `json:"resolved_at"`
}

// GetAlertHistory P0-4: 真实读 alert_records, 不再是写死的 2 条.
func (s *AlertService) GetAlertHistory(ctx context.Context, filter AlertHistoryFilter) ([]AlertHistoryEntry, error) {
	records, err := s.ruleRepo.ListAlertHistory(ctx, repositories.AlertHistoryFilter{
		InstanceID: filter.InstanceID,
		RuleID:     filter.RuleID,
		Status:     filter.Status,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	})
	if err != nil {
		return nil, err
	}
	ruleNames := make(map[string]string)
	out := make([]AlertHistoryEntry, 0, len(records))
	for _, r := range records {
		ruleName, ok := ruleNames[r.RuleID]
		if !ok {
			ruleName = r.RuleID
			if rule, err := s.ruleRepo.GetAlertRuleByID(ctx, r.RuleID); err == nil && rule != nil && rule.Name != "" {
				ruleName = rule.Name
			}
			ruleNames[r.RuleID] = ruleName
		}
		out = append(out, AlertHistoryEntry{
			ID:          r.ID,
			RuleID:      r.RuleID,
			RuleName:    ruleName,
			InstanceID:  r.InstanceID,
			Status:      r.Status,
			Severity:    r.Severity,
			Value:       r.Value,
			Message:     r.Message,
			TriggeredAt: r.TriggeredAt,
			ResolvedAt:  r.ResolvedAt,
		})
	}
	return out, nil
}

func (s *AlertService) CreateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	return s.ruleRepo.CreateAlertRule(ctx, rule)
}

func (s *AlertService) UpdateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	return s.ruleRepo.UpdateAlertRule(ctx, rule)
}

func (s *AlertService) DeleteAlertRule(ctx context.Context, id string) error {
	return s.ruleRepo.DeleteAlertRule(ctx, id)
}

func (s *AlertService) GetAlertRuleByID(ctx context.Context, id string) (*models.AlertRule, error) {
	return s.ruleRepo.GetAlertRuleByID(ctx, id)
}

func (s *AlertService) ListAlertRules(ctx context.Context, limit, offset int) ([]models.AlertRule, error) {
	return s.ruleRepo.ListAlertRules(ctx, limit, offset)
}

func (s *AlertService) CreateNotificationChannel(ctx context.Context, channel *models.NotificationChannel) error {
	return s.notificationRepo.CreateAlertNotification(ctx, channel)
}

func (s *AlertService) UpdateNotificationChannel(ctx context.Context, channel *models.NotificationChannel) error {
	return s.notificationRepo.UpdateAlertNotification(ctx, channel)
}

func (s *AlertService) DeleteNotificationChannel(ctx context.Context, id string) error {
	return s.notificationRepo.DeleteAlertNotification(ctx, id)
}

func (s *AlertService) GetNotificationChannelByID(ctx context.Context, id string) (*models.NotificationChannel, error) {
	return s.notificationRepo.GetAlertNotificationByID(ctx, id)
}

func (s *AlertService) ListNotificationChannels(ctx context.Context, limit, offset int) ([]models.NotificationChannel, error) {
	return s.notificationRepo.ListAlertNotifications(ctx, limit, offset)
}

// --- Escalation ---

func (s *AlertService) CreateEscalation(ctx context.Context, e *models.AlertEscalation) error {
	e.CreatedAt = time.Now()
	e.UpdatedAt = time.Now()
	return s.escRepo.Create(ctx, e)
}

func (s *AlertService) ListEscalations(ctx context.Context, ruleID string) ([]models.AlertEscalation, error) {
	return s.escRepo.ListByRuleID(ctx, ruleID)
}

func (s *AlertService) GetEscalation(ctx context.Context, id string) (*models.AlertEscalation, error) {
	return s.escRepo.GetByID(ctx, id)
}

func (s *AlertService) DeleteEscalation(ctx context.Context, id string) error {
	return s.escRepo.Delete(ctx, id)
}

// --- Silence ---

func (s *AlertService) CreateSilence(ctx context.Context, sl *models.AlertSilence) error {
	sl.CreatedAt = time.Now()
	sl.UpdatedAt = time.Now()
	return s.silenceRepo.Create(ctx, sl)
}

func (s *AlertService) ListSilences(ctx context.Context) ([]models.AlertSilence, error) {
	return s.silenceRepo.List(ctx)
}

func (s *AlertService) GetSilence(ctx context.Context, id string) (*models.AlertSilence, error) {
	return s.silenceRepo.GetByID(ctx, id)
}

func (s *AlertService) UpdateSilence(ctx context.Context, sl *models.AlertSilence) error {
	sl.UpdatedAt = time.Now()
	return s.silenceRepo.Update(ctx, sl)
}

func (s *AlertService) DeleteSilence(ctx context.Context, id string) error {
	return s.silenceRepo.Delete(ctx, id)
}

// --- Inspection Templates ---

func (s *AlertService) CreateInspectionTemplate(ctx context.Context, t *models.InspectionTemplate) error {
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	return s.inspectionTplRepo.Create(ctx, t)
}

func (s *AlertService) ListInspectionTemplates(ctx context.Context, category string) ([]models.InspectionTemplate, error) {
	return s.inspectionTplRepo.List(ctx, category)
}

func (s *AlertService) GetInspectionTemplate(ctx context.Context, id string) (*models.InspectionTemplate, error) {
	return s.inspectionTplRepo.GetByID(ctx, id)
}

func (s *AlertService) UpdateInspectionTemplate(ctx context.Context, t *models.InspectionTemplate) error {
	t.UpdatedAt = time.Now()
	return s.inspectionTplRepo.Update(ctx, t)
}

func (s *AlertService) DeleteInspectionTemplate(ctx context.Context, id string) error {
	return s.inspectionTplRepo.Delete(ctx, id)
}

// --- Inspection Reports ---

func (s *AlertService) GenerateInspectionReport(ctx context.Context, templateID, instanceID string) (*models.InspectionReport, error) {
	tpl, err := s.inspectionTplRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("template not found: %w", err)
	}

	now := time.Now()
	report := &models.InspectionReport{
		TemplateID:  templateID,
		InstanceID:  instanceID,
		Status:      "generating",
		GeneratedAt: now,
		CreatedAt:   now,
	}

	if err := s.inspectionRptRepo.Create(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to create report record: %w", err)
	}

	// Run checks based on template config
	summary, details, score := s.runInspectionChecks(ctx, tpl, instanceID)
	report.Status = "completed"
	report.Summary = summary
	report.Details = details
	report.Score = score

	if err := s.inspectionRptRepo.Update(ctx, report); err != nil {
		return nil, fmt.Errorf("failed to update report results: %w", err)
	}

	return report, nil
}

func (s *AlertService) runInspectionChecks(ctx context.Context, tpl *models.InspectionTemplate, instanceID string) (summary, details string, score int) {
	baseScore := 100
	var issues []string

	// 1. Check instance health
	healthMetrics := []string{"cpu", "mem", "disk", "connections", "qps"}
	metrics, err := s.monitorService.QueryMetrics(ctx, MetricQueryRequest{
		InstanceID: instanceID,
		Metrics:    healthMetrics,
	})
	if err == nil {
		metricMap := make(map[string]float64)
		for _, m := range metrics {
			metricMap[m.Name] = m.Value
		}
		if v, ok := metricMap["cpu"]; ok && v > 90 {
			issues = append(issues, "CPU usage high (> 90%)")
			baseScore -= 10
		}
		if v, ok := metricMap["mem"]; ok && v > 90 {
			issues = append(issues, "Memory usage high (> 90%)")
			baseScore -= 10
		}
		if v, ok := metricMap["disk"]; ok && v > 85 {
			issues = append(issues, "Disk usage high (> 85%)")
			baseScore -= 15
		}
		if v, ok := metricMap["connections"]; ok && v > 500 {
			issues = append(issues, fmt.Sprintf("High connection count (%.0f)", v))
			baseScore -= 5
		}
		if v, ok := metricMap["qps"]; ok && v > 10000 {
			issues = append(issues, fmt.Sprintf("High QPS (%.0f)", v))
			baseScore -= 5
		}
	}

	// 2. Check active alert rules for this instance
	alerts, _ := s.ruleRepo.ListAlertHistory(ctx, repositories.AlertHistoryFilter{
		InstanceID: instanceID,
		Status:     "firing",
		Limit:      10,
	})
	if len(alerts) > 0 {
		issues = append(issues, fmt.Sprintf("%d active alerts firing", len(alerts)))
		baseScore -= len(alerts) * 5
	}

	if baseScore < 0 {
		baseScore = 0
	}

	summary = fmt.Sprintf("巡检得分: %d/100", baseScore)
	if len(issues) > 0 {
		summary += " — 发现 " + fmt.Sprintf("%d", len(issues)) + " 项问题"
	} else {
		summary += " — 运行状态良好"
	}

	detailBytes, _ := json.Marshal(map[string]interface{}{
		"score":    baseScore,
		"issues":   issues,
		"template": tpl.Name,
		"at":       time.Now().Format(time.RFC3339),
	})
	details = string(detailBytes)

	return summary, details, baseScore
}

func (s *AlertService) ListInspectionReports(ctx context.Context, templateID string, limit, offset int) ([]models.InspectionReport, error) {
	return s.inspectionRptRepo.List(ctx, templateID, limit, offset)
}

func (s *AlertService) GetInspectionReport(ctx context.Context, id string) (*models.InspectionReport, error) {
	return s.inspectionRptRepo.GetByID(ctx, id)
}

func (s *AlertService) DeleteInspectionReport(ctx context.Context, id string) error {
	return s.inspectionRptRepo.Delete(ctx, id)
}
