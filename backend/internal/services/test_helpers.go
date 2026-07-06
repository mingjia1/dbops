package services

import (
	"context"
	"errors"

	"github.com/jackcode/mysql-ops-platform/internal/models"
)

type TaskRepositoryInterface interface {
	Create(ctx context.Context, task *models.Task) error
	GetByID(ctx context.Context, id string) (*models.Task, error)
	Update(ctx context.Context, task *models.Task) error
}

type AlertRuleRepositoryInterface interface {
	CreateAlertRule(ctx context.Context, rule *models.AlertRule) error
	UpdateAlertRule(ctx context.Context, rule *models.AlertRule) error
	DeleteAlertRule(ctx context.Context, id string) error
	GetAlertRuleByID(ctx context.Context, id string) (*models.AlertRule, error)
	ListAlertRules(ctx context.Context, limit, offset int) ([]models.AlertRule, error)
}

type AlertNotificationRepositoryInterface interface {
	CreateAlertNotification(ctx context.Context, notification *models.NotificationChannel) error
	UpdateAlertNotification(ctx context.Context, notification *models.NotificationChannel) error
	DeleteAlertNotification(ctx context.Context, id string) error
	GetAlertNotificationByID(ctx context.Context, id string) (*models.NotificationChannel, error)
	ListAlertNotifications(ctx context.Context, limit, offset int) ([]models.NotificationChannel, error)
}

type MonitorServiceInterface interface {
	QueryMetrics(ctx context.Context, req MetricQueryRequest) ([]MetricData, error)
	CollectMetrics(ctx context.Context, instanceID string) error
}

type TestableUpgradeService struct {
	instanceRepo InstanceRepositoryInterface
	taskRepo     TaskRepositoryInterface
}

func NewTestableUpgradeService(instanceRepo InstanceRepositoryInterface, taskRepo TaskRepositoryInterface) *TestableUpgradeService {
	return &TestableUpgradeService{
		instanceRepo: instanceRepo,
		taskRepo:     taskRepo,
	}
}

func (s *TestableUpgradeService) PlanUpgradePath(ctx context.Context, req PlanUpgradePathRequest) (*PlanUpgradePathResponse, error) {
	instance, err := s.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, err
	}

	sourceVersion := "5.7.40"
	targetVersion := req.TargetVersion

	var steps []UpgradeStepInfo
	var estimatedTime int
	var riskLevel string

	switch req.Strategy {
	case "inplace":
		steps = s.planInPlaceUpgradeSteps(sourceVersion, targetVersion)
		estimatedTime = 30
		riskLevel = "medium"
	case "logical":
		steps = s.planLogicalMigrationSteps(sourceVersion, targetVersion)
		estimatedTime = 120
		riskLevel = "low"
	case "rolling":
		steps = s.planRollingUpgradeSteps(sourceVersion, targetVersion)
		estimatedTime = 60
		riskLevel = "medium"
	default:
		return nil, err
	}

	warnings := s.generatePreCheckWarnings(instance, req.Strategy)

	return &PlanUpgradePathResponse{
		PlanID:           "test-plan-id",
		SourceVersion:    sourceVersion,
		TargetVersion:    targetVersion,
		Strategy:         req.Strategy,
		UpgradePath:      steps,
		EstimatedTime:    estimatedTime,
		RiskLevel:        riskLevel,
		PreCheckWarnings: warnings,
	}, nil
}

func (s *TestableUpgradeService) planInPlaceUpgradeSteps(source, target string) []UpgradeStepInfo {
	return []UpgradeStepInfo{
		{Order: 1, Name: "Pre-upgrade Check", Type: "check", Description: "Validate system requirements"},
		{Order: 2, Name: "Create Backup", Type: "backup", Description: "Full backup of data directory"},
		{Order: 3, Name: "Stop Instance", Type: "operation", Description: "Stop MySQL service"},
		{Order: 4, Name: "Replace Binaries", Type: "operation", Description: "Replace MySQL binaries"},
		{Order: 5, Name: "Start Instance", Type: "operation", Description: "Start MySQL service"},
		{Order: 6, Name: "Run Upgrade", Type: "upgrade", Description: "Execute upgrade procedure"},
		{Order: 7, Name: "Post-upgrade Check", Type: "check", Description: "Verify upgrade completion"},
		{Order: 8, Name: "Cleanup", Type: "cleanup", Description: "Remove temporary files"},
	}
}

func (s *TestableUpgradeService) planLogicalMigrationSteps(source, target string) []UpgradeStepInfo {
	return []UpgradeStepInfo{
		{Order: 1, Name: "Pre-migration Check", Type: "check", Description: "Validate compatibility"},
		{Order: 2, Name: "Prepare Target", Type: "prepare", Description: "Configure target instance"},
		{Order: 3, Name: "Export Schema", Type: "export", Description: "Export schema from source"},
		{Order: 4, Name: "Import Schema", Type: "import", Description: "Import schema to target"},
		{Order: 5, Name: "Initial Data Sync", Type: "sync", Description: "Full data sync"},
		{Order: 6, Name: "Setup Replication", Type: "replication", Description: "Configure CDC"},
		{Order: 7, Name: "Catch-up Sync", Type: "sync", Description: "Sync remaining changes"},
		{Order: 8, Name: "Cutover", Type: "cutover", Description: "Switch to new instance"},
		{Order: 9, Name: "Verify Migration", Type: "verify", Description: "Verify integrity"},
		{Order: 10, Name: "Cleanup", Type: "cleanup", Description: "Remove artifacts"},
	}
}

func (s *TestableUpgradeService) planRollingUpgradeSteps(source, target string) []UpgradeStepInfo {
	return rollingUpgradeStepsForInstances("ha", nil, source, target)
}

func (s *TestableUpgradeService) generatePreCheckWarnings(instance *models.Instance, strategy string) []string {
	warnings := []string{}
	warnings = append(warnings, "Ensure sufficient disk space for backup")
	warnings = append(warnings, "Verify all applications are compatible with MySQL 8.0")

	if strategy == "inplace" {
		warnings = append(warnings, "In-place upgrade cannot be rolled back automatically")
	}

	return warnings
}

func (s *TestableUpgradeService) CheckCompatibility(ctx context.Context, req CheckCompatibilityRequest) (*CheckCompatibilityResponse, error) {
	_, err := s.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, err
	}

	incompatibilities := []IncompatibilityItem{
		{Type: "sql_mode", Level: "warning", Description: "NO_AUTO_CREATE_USER removed"},
		{Type: "feature", Level: "warning", Description: "Query cache removed"},
	}

	recommendations := []string{
		"Backup all data before upgrade",
		"Test upgrade in staging environment first",
	}

	return &CheckCompatibilityResponse{
		CheckID:           "test-check-id",
		InstanceID:        req.InstanceID,
		SourceVersion:     "5.7.40",
		TargetVersion:     req.TargetVersion,
		IsCompatible:      true,
		WarningCount:      2,
		ErrorCount:        0,
		Incompatibilities: incompatibilities,
		Recommendations:   recommendations,
	}, nil
}

type TestableTopologyService struct {
	repo InstanceRepositoryInterface
}

func NewTestableTopologyService(repo InstanceRepositoryInterface) *TestableTopologyService {
	return &TestableTopologyService{repo: repo}
}

func (s *TestableTopologyService) GetInstanceTopology(ctx context.Context, instanceID string) (*InstanceTopologyResponse, error) {
	instance, err := s.repo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	return &InstanceTopologyResponse{
		InstanceID:      instance.ID,
		ClusterID:       instance.ClusterID,
		MasterID:        "",
		SlaveIDs:        []string{},
		ReplicationMode: "async",
		Role:            normalizeTopologyRole(instance.Status.Role, "unknown"),
	}, nil
}

func (s *TestableTopologyService) GetClusterTopology(ctx context.Context, clusterID string) (*ClusterTopologyResponse, error) {
	instances, err := s.repo.List(ctx, 100, 0)
	if err != nil {
		return nil, err
	}

	var clusterInstances []models.Instance
	for _, inst := range instances {
		if inst.ClusterID == clusterID {
			clusterInstances = append(clusterInstances, inst)
		}
	}

	if len(clusterInstances) == 0 {
		return nil, errors.New("cluster not found or has no instances")
	}

	var topologyInstances []InstanceTopologyResponse
	for _, inst := range clusterInstances {
		topologyInstances = append(topologyInstances, InstanceTopologyResponse{
			InstanceID:      inst.ID,
			ClusterID:       inst.ClusterID,
			Role:            normalizeTopologyRole(inst.Status.Role, "unknown"),
			ReplicationMode: "async",
		})
	}
	inferClusterTopology(topologyInstances)

	return &ClusterTopologyResponse{
		ClusterID:       clusterID,
		ReplicationMode: "async",
		Instances:       topologyInstances,
	}, nil
}

type TestableAlertService struct {
	ruleRepo         AlertRuleRepositoryInterface
	notificationRepo AlertNotificationRepositoryInterface
	monitorService   MonitorServiceInterface
}

func NewTestableAlertService(ruleRepo AlertRuleRepositoryInterface, notificationRepo AlertNotificationRepositoryInterface, monitorService MonitorServiceInterface) *TestableAlertService {
	return &TestableAlertService{
		ruleRepo:         ruleRepo,
		notificationRepo: notificationRepo,
		monitorService:   monitorService,
	}
}

func (s *TestableAlertService) EvaluateAlertRule(ctx context.Context, req EvaluateAlertRuleRequest) (*AlertEvaluationResult, error) {
	rule, err := s.ruleRepo.GetAlertRuleByID(ctx, req.RuleID)
	if err != nil {
		return nil, err
	}

	metrics, err := s.monitorService.QueryMetrics(ctx, MetricQueryRequest{
		InstanceID: req.InstanceID,
		Metrics:    []string{rule.Metric},
	})
	if err != nil {
		return nil, err
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
		message = "Alert triggered"
	}

	return &AlertEvaluationResult{
		RuleID:     req.RuleID,
		InstanceID: req.InstanceID,
		Triggered:  triggered,
		Value:      currentValue,
		Threshold:  rule.Threshold,
		Message:    message,
	}, nil
}

func (s *TestableAlertService) evaluateCondition(condition string, value, threshold float64) bool {
	switch condition {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==", "=":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

func (s *TestableAlertService) TriggerAlert(ctx context.Context, req TriggerAlertRequest) (*AlertTriggerResult, error) {
	rule, err := s.ruleRepo.GetAlertRuleByID(ctx, req.RuleID)
	if err != nil {
		return nil, err
	}

	return &AlertTriggerResult{
		AlertID:    "test-alert-id",
		RuleID:     req.RuleID,
		InstanceID: req.InstanceID,
		Status:     "firing",
		Severity:   rule.Severity,
		Message:    req.Message,
	}, nil
}

func (s *TestableAlertService) CreateAlertRule(ctx context.Context, rule *models.AlertRule) error {
	return s.ruleRepo.CreateAlertRule(ctx, rule)
}

func (s *TestableAlertService) GetAlertRuleByID(ctx context.Context, id string) (*models.AlertRule, error) {
	return s.ruleRepo.GetAlertRuleByID(ctx, id)
}
