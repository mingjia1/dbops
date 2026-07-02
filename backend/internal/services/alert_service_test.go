package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockAlertRuleRepo struct {
	mock.Mock
}

func (m *MockAlertRuleRepo) CreateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *MockAlertRuleRepo) UpdateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *MockAlertRuleRepo) DeleteAlertRule(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAlertRuleRepo) GetAlertRuleByID(ctx context.Context, id string) (*models.AlertRule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AlertRule), args.Error(1)
}

func (m *MockAlertRuleRepo) ListAlertRules(ctx context.Context, limit, offset int) ([]models.AlertRule, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AlertRule), args.Error(1)
}

type MockAlertNotificationRepo struct {
	mock.Mock
}

func (m *MockAlertNotificationRepo) CreateAlertNotification(ctx context.Context, notification *models.NotificationChannel) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockAlertNotificationRepo) UpdateAlertNotification(ctx context.Context, notification *models.NotificationChannel) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockAlertNotificationRepo) DeleteAlertNotification(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAlertNotificationRepo) GetAlertNotificationByID(ctx context.Context, id string) (*models.NotificationChannel, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.NotificationChannel), args.Error(1)
}

func (m *MockAlertNotificationRepo) ListAlertNotifications(ctx context.Context, limit, offset int) ([]models.NotificationChannel, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.NotificationChannel), args.Error(1)
}

type MockMonitorSvc struct {
	mock.Mock
}

func (m *MockMonitorSvc) QueryMetrics(ctx context.Context, req MetricQueryRequest) ([]MetricData, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]MetricData), args.Error(1)
}

func (m *MockMonitorSvc) CollectMetrics(ctx context.Context, instanceID string) error {
	args := m.Called(ctx, instanceID)
	return args.Error(0)
}

func TestAlertService_EvaluateAlertRule(t *testing.T) {
	mockRuleRepo := new(MockAlertRuleRepo)
	mockNotificationRepo := new(MockAlertNotificationRepo)
	mockMonitorService := new(MockMonitorSvc)
	service := NewTestableAlertService(mockRuleRepo, mockNotificationRepo, mockMonitorService)

	ctx := context.Background()

	testRule := &models.AlertRule{
		ID:        "rule-001",
		Name:      "CPU High",
		Metric:    "cpu_usage",
		Condition: ">",
		Threshold: 80.0,
		Severity:  "critical",
	}

	mockRuleRepo.On("GetAlertRuleByID", ctx, "rule-001").Return(testRule, nil)

	metrics := []MetricData{
		{Name: "cpu_usage", Value: 85.5},
	}

	mockMonitorService.On("QueryMetrics", ctx, mock.Anything).Return(metrics, nil)

	req := EvaluateAlertRuleRequest{
		RuleID:     "rule-001",
		InstanceID: "instance-001",
	}

	result, err := service.EvaluateAlertRule(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "rule-001", result.RuleID)
	assert.Equal(t, "instance-001", result.InstanceID)
	assert.True(t, result.Triggered)
	assert.Equal(t, 85.5, result.Value)
	assert.Equal(t, 80.0, result.Threshold)
	assert.NotEmpty(t, result.Message)

	mockRuleRepo.AssertExpectations(t)
	mockMonitorService.AssertExpectations(t)
}

func TestAlertService_EvaluateAlertRule_NotTriggered(t *testing.T) {
	mockRuleRepo := new(MockAlertRuleRepo)
	mockNotificationRepo := new(MockAlertNotificationRepo)
	mockMonitorService := new(MockMonitorSvc)
	service := NewTestableAlertService(mockRuleRepo, mockNotificationRepo, mockMonitorService)

	ctx := context.Background()

	testRule := &models.AlertRule{
		ID:        "rule-002",
		Name:      "CPU High",
		Metric:    "cpu_usage",
		Condition: ">",
		Threshold: 90.0,
		Severity:  "critical",
	}

	mockRuleRepo.On("GetAlertRuleByID", ctx, "rule-002").Return(testRule, nil)

	metrics := []MetricData{
		{Name: "cpu_usage", Value: 50.0},
	}

	mockMonitorService.On("QueryMetrics", ctx, mock.Anything).Return(metrics, nil)

	req := EvaluateAlertRuleRequest{
		RuleID:     "rule-002",
		InstanceID: "instance-002",
	}

	result, err := service.EvaluateAlertRule(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.Triggered)
	assert.Equal(t, 50.0, result.Value)
	assert.Empty(t, result.Message)

	mockRuleRepo.AssertExpectations(t)
	mockMonitorService.AssertExpectations(t)
}

func TestAlertService_EvaluateAlertRule_RuleNotFound(t *testing.T) {
	mockRuleRepo := new(MockAlertRuleRepo)
	mockNotificationRepo := new(MockAlertNotificationRepo)
	mockMonitorService := new(MockMonitorSvc)
	service := NewTestableAlertService(mockRuleRepo, mockNotificationRepo, mockMonitorService)

	ctx := context.Background()

	mockRuleRepo.On("GetAlertRuleByID", ctx, "non-existent").Return(nil, errors.New("rule not found"))

	req := EvaluateAlertRuleRequest{
		RuleID:     "non-existent",
		InstanceID: "instance-001",
	}

	result, err := service.EvaluateAlertRule(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)

	mockRuleRepo.AssertExpectations(t)
}

func TestAlertService_TriggerAlert(t *testing.T) {
	mockRuleRepo := new(MockAlertRuleRepo)
	mockNotificationRepo := new(MockAlertNotificationRepo)
	mockMonitorService := new(MockMonitorSvc)
	service := NewTestableAlertService(mockRuleRepo, mockNotificationRepo, mockMonitorService)

	ctx := context.Background()

	testRule := &models.AlertRule{
		ID:        "rule-001",
		Name:      "CPU High",
		Metric:    "cpu_usage",
		Condition: ">",
		Threshold: 80.0,
		Severity:  "critical",
	}

	mockRuleRepo.On("GetAlertRuleByID", ctx, "rule-001").Return(testRule, nil)

	req := TriggerAlertRequest{
		RuleID:     "rule-001",
		InstanceID: "instance-001",
		Value:      85.5,
		Message:    "CPU usage exceeded threshold",
	}

	result, err := service.TriggerAlert(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.AlertID)
	assert.Equal(t, "rule-001", result.RuleID)
	assert.Equal(t, "instance-001", result.InstanceID)
	assert.Equal(t, "firing", result.Status)
	assert.Equal(t, "critical", result.Severity)

	mockRuleRepo.AssertExpectations(t)
}

func TestAlertService_CreateAlertRule(t *testing.T) {
	mockRuleRepo := new(MockAlertRuleRepo)
	mockNotificationRepo := new(MockAlertNotificationRepo)
	mockMonitorService := new(MockMonitorSvc)
	service := NewTestableAlertService(mockRuleRepo, mockNotificationRepo, mockMonitorService)

	ctx := context.Background()

	rule := &models.AlertRule{
		Name:      "New Alert Rule",
		Metric:    "memory_usage",
		Condition: ">",
		Threshold: 90.0,
		Severity:  "warning",
	}

	mockRuleRepo.On("CreateAlertRule", ctx, rule).Return(nil)

	err := service.CreateAlertRule(ctx, rule)

	assert.NoError(t, err)

	mockRuleRepo.AssertExpectations(t)
}

func TestAlertService_GetAlertRuleByID(t *testing.T) {
	mockRuleRepo := new(MockAlertRuleRepo)
	mockNotificationRepo := new(MockAlertNotificationRepo)
	mockMonitorService := new(MockMonitorSvc)
	service := NewTestableAlertService(mockRuleRepo, mockNotificationRepo, mockMonitorService)

	ctx := context.Background()

	testRule := &models.AlertRule{
		ID:        "rule-001",
		Name:      "CPU High",
		Metric:    "cpu_usage",
		Condition: ">",
		Threshold: 80.0,
		Severity:  "critical",
	}

	mockRuleRepo.On("GetAlertRuleByID", ctx, "rule-001").Return(testRule, nil)

	rule, err := service.GetAlertRuleByID(ctx, "rule-001")

	assert.NoError(t, err)
	assert.NotNil(t, rule)
	assert.Equal(t, "rule-001", rule.ID)

	mockRuleRepo.AssertExpectations(t)
}

func TestAlertService_Original_SendNotification(t *testing.T) {
	db := newTestDB(t)
	ruleRepo := repositories.NewAlertRuleRepository(db)
	notifRepo := repositories.NewAlertNotificationRepository(db)
	service := NewAlertService(ruleRepo, notifRepo, nil)

	ctx := context.Background()

	req := SendNotificationRequest{
		AlertID:  "alert-001",
		Channels: []string{"email", "slack"},
		Message:  "Alert triggered",
	}

	result, err := service.SendNotification(ctx, req)

	// 没有真实 channel 配置时, 不应该 panic; result 应该有记录
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "alert-001", result.AlertID)
	// 两个未知 channel 都会失败, SuccessCount 为 0
	assert.Equal(t, 0, result.SuccessCount)
}

func TestAlertService_Original_GetAlertHistory(t *testing.T) {
	db := newTestDB(t)
	ruleRepo := repositories.NewAlertRuleRepository(db)
	notifRepo := repositories.NewAlertNotificationRepository(db)
	service := NewAlertService(ruleRepo, notifRepo, nil)

	ctx := context.Background()

	filter := AlertHistoryFilter{
		InstanceID: "instance-001",
		Limit:      10,
	}

	history, err := service.GetAlertHistory(ctx, filter)

	// 真实仓库空时返回空 list, 不应该是 nil
	assert.NoError(t, err)
	assert.NotNil(t, history)
	assert.Empty(t, history)
}

func TestAlertService_GetAlertHistoryReturnsRuleName(t *testing.T) {
	db := newTestDB(t)
	ruleRepo := repositories.NewAlertRuleRepository(db)
	notifRepo := repositories.NewAlertNotificationRepository(db)
	service := NewAlertService(ruleRepo, notifRepo, nil)
	ctx := context.Background()

	rule := &models.AlertRule{
		Name:            "CPU High",
		Metric:          "cpu_usage",
		Condition:       ">",
		Threshold:       80,
		Severity:        "warning",
		DurationSeconds: 60,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	require.NoError(t, ruleRepo.CreateAlertRule(ctx, rule))
	require.NoError(t, ruleRepo.CreateAlertRecord(ctx, &models.AlertRecord{
		ID:          "alert-cpu-high",
		RuleID:      rule.ID,
		InstanceID:  "instance-001",
		TriggeredAt: time.Now(),
		Status:      "firing",
		Severity:    "warning",
		Value:       91.5,
		Message:     "CPU usage is high",
		CreatedAt:   time.Now(),
	}))

	history, err := service.GetAlertHistory(ctx, AlertHistoryFilter{InstanceID: "instance-001", Limit: 10})

	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, rule.ID, history[0].RuleID)
	assert.Equal(t, "CPU High", history[0].RuleName)
}

func TestAlertService_Original_evaluateCondition(t *testing.T) {
	testableService := NewTestableAlertService(nil, nil, nil)

	assert.True(t, testableService.evaluateCondition(">", 10.0, 5.0))
	assert.True(t, testableService.evaluateCondition(">=", 10.0, 10.0))
	assert.True(t, testableService.evaluateCondition("<", 5.0, 10.0))
	assert.True(t, testableService.evaluateCondition("<=", 10.0, 10.0))
	assert.True(t, testableService.evaluateCondition("==", 10.0, 10.0))
	assert.True(t, testableService.evaluateCondition("!=", 10.0, 5.0))
	assert.False(t, testableService.evaluateCondition("invalid", 10.0, 5.0))
}
