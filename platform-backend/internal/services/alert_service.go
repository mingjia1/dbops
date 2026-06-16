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
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/notifier"
)

type AlertService struct {
	ruleRepo         *repositories.AlertRuleRepository
	notificationRepo *repositories.AlertNotificationRepository
	monitorService   *MonitorService
}

func NewAlertService(ruleRepo *repositories.AlertRuleRepository, notificationRepo *repositories.AlertNotificationRepository, monitorService *MonitorService) *AlertService {
	return &AlertService{
		ruleRepo:         ruleRepo,
		notificationRepo: notificationRepo,
		monitorService:   monitorService,
	}
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

	triggered := s.evaluateCondition(rule.Condition, currentValue, rule.Threshold)

	message := ""
	if triggered {
		message = fmt.Sprintf("Alert %s triggered: %s %.2f %s %.2f", rule.Name, rule.Metric, currentValue, rule.Condition, rule.Threshold)
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
