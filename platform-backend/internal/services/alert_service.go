package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type AlertService struct {
	ruleRepo        *repositories.AlertRuleRepository
	notificationRepo *repositories.AlertNotificationRepository
	monitorService  *MonitorService
}

func NewAlertService(ruleRepo *repositories.AlertRuleRepository, notificationRepo *repositories.AlertNotificationRepository, monitorService *MonitorService) *AlertService {
	return &AlertService{
		ruleRepo:        ruleRepo,
		notificationRepo: notificationRepo,
		monitorService:  monitorService,
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
	AlertID    string    `json:"alert_id"`
	RuleID     string    `json:"rule_id"`
	InstanceID string    `json:"instance_id"`
	Status     string    `json:"status"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	TriggeredAt time.Time `json:"triggered_at"`
}

func (s *AlertService) TriggerAlert(ctx context.Context, req TriggerAlertRequest) (*AlertTriggerResult, error) {
	rule, err := s.ruleRepo.GetAlertRuleByID(ctx, req.RuleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get alert rule: %w", err)
	}

	alertID := uuid.New().String()
	alertRecord := &models.AlertRecord{
		ID:         alertID,
		RuleID:     req.RuleID,
		InstanceID: req.InstanceID,
		TriggeredAt: time.Now(),
		Status:     "firing",
		Severity:   rule.Severity,
		Value:      req.Value,
		Message:    req.Message,
		CreatedAt:  time.Now(),
	}

	fmt.Printf("Alert triggered: %s - Instance: %s - Value: %.2f\n", rule.Name, req.InstanceID, req.Value)

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
	AlertID  string `json:"alert_id" binding:"required"`
	Channels []string `json:"channels" binding:"required"`
	Message  string `json:"message" binding:"required"`
}

type NotificationResult struct {
	AlertID      string    `json:"alert_id"`
	Channels     []string  `json:"channels"`
	Status       string    `json:"status"`
	SentAt       time.Time `json:"sent_at"`
	SuccessCount int       `json:"success_count"`
}

func (s *AlertService) SendNotification(ctx context.Context, req SendNotificationRequest) (*NotificationResult, error) {
	sentAt := time.Now()
	successCount := 0

	for _, channel := range req.Channels {
		fmt.Printf("Sending notification via %s: %s\n", channel, req.Message)
		successCount++
	}

	status := "sent"
	if successCount == 0 {
		status = "failed"
	}

	return &NotificationResult{
		AlertID:      req.AlertID,
		Channels:     req.Channels,
		Status:       status,
		SentAt:       sentAt,
		SuccessCount: successCount,
	}, nil
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

func (s *AlertService) GetAlertHistory(ctx context.Context, filter AlertHistoryFilter) ([]AlertHistoryEntry, error) {
	resolvedTime := time.Now().Add(-30 * time.Minute)
	triggeredTime2 := time.Now().Add(-2 * time.Hour)

	history := []AlertHistoryEntry{
		{
			ID:          uuid.New().String(),
			RuleID:      "rule-001",
			RuleName:    "CPU High Alert",
			InstanceID:  "instance-001",
			Status:      "firing",
			Severity:    "critical",
			Value:       85.5,
			Message:     "CPU usage exceeded threshold",
			TriggeredAt: time.Now().Add(-1 * time.Hour),
			ResolvedAt:  nil,
		},
		{
			ID:          uuid.New().String(),
			RuleID:      "rule-002",
			RuleName:    "Memory Low Alert",
			InstanceID:  "instance-002",
			Status:      "resolved",
			Severity:    "warning",
			Value:       15.2,
			Message:     "Memory usage below threshold",
			TriggeredAt: triggeredTime2,
			ResolvedAt:  &resolvedTime,
		},
	}

	return history, nil
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