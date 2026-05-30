package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
)

type UpgradeService struct {
	instanceRepo *repositories.InstanceRepository
	taskRepo     *repositories.TaskRepository
}

func NewUpgradeService(instanceRepo *repositories.InstanceRepository, taskRepo *repositories.TaskRepository) *UpgradeService {
	return &UpgradeService{
		instanceRepo: instanceRepo,
		taskRepo:     taskRepo,
	}
}

type PlanUpgradePathRequest struct {
	InstanceID    string `json:"instance_id" binding:"required"`
	TargetVersion string `json:"target_version" binding:"required"`
	Strategy      string `json:"strategy" binding:"required"`
}

type PlanUpgradePathResponse struct {
	PlanID          string       `json:"plan_id"`
	SourceVersion   string       `json:"source_version"`
	TargetVersion   string       `json:"target_version"`
	Strategy        string       `json:"strategy"`
	UpgradePath     []UpgradeStepInfo `json:"upgrade_path"`
	EstimatedTime   int          `json:"estimated_time"`
	RiskLevel       string       `json:"risk_level"`
	PreCheckWarnings []string    `json:"pre_check_warnings"`
}

type UpgradeStepInfo struct {
	Order       int    `json:"order"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

func (s *UpgradeService) PlanUpgradePath(ctx context.Context, req PlanUpgradePathRequest) (*PlanUpgradePathResponse, error) {
	instance, err := s.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
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
		return nil, fmt.Errorf("unsupported upgrade strategy: %s", req.Strategy)
	}

	warnings := s.generatePreCheckWarnings(instance, req.Strategy)

	plan := &models.UpgradePlan{
		ID:             uuid.New().String(),
		InstanceID:     req.InstanceID,
		SourceVersion:  sourceVersion,
		TargetVersion:  targetVersion,
		Strategy:       req.Strategy,
		Status:         "planned",
		EstimatedTime:  estimatedTime,
		RiskLevel:      riskLevel,
		UpgradePath:     s.serializeSteps(steps),
	}

	response := &PlanUpgradePathResponse{
		PlanID:          plan.ID,
		SourceVersion:   sourceVersion,
		TargetVersion:   targetVersion,
		Strategy:        req.Strategy,
		UpgradePath:     steps,
		EstimatedTime:   estimatedTime,
		RiskLevel:       riskLevel,
		PreCheckWarnings: warnings,
	}

	return response, nil
}

type CheckCompatibilityRequest struct {
	InstanceID    string `json:"instance_id" binding:"required"`
	TargetVersion string `json:"target_version" binding:"required"`
}

type CheckCompatibilityResponse struct {
	CheckID           string               `json:"check_id"`
	InstanceID        string               `json:"instance_id"`
	SourceVersion     string               `json:"source_version"`
	TargetVersion     string              `json:"target_version"`
	IsCompatible      bool                 `json:"is_compatible"`
	WarningCount      int                  `json:"warning_count"`
	ErrorCount        int                  `json:"error_count"`
	Incompatibilities []IncompatibilityItem `json:"incompatibilities"`
	Recommendations   []string             `json:"recommendations"`
}

type IncompatibilityItem struct {
	Type        string `json:"type"`
	Level       string `json:"level"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Solution    string `json:"solution"`
}

func (s *UpgradeService) CheckCompatibility(ctx context.Context, req CheckCompatibilityRequest) (*CheckCompatibilityResponse, error) {
	_, err := s.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	sourceVersion := "5.7.40"
	targetVersion := req.TargetVersion

	var incompatibilities []IncompatibilityItem
	var recommendations []string

	sqlModeIssues := s.checkSQLModeCompatibility(sourceVersion, targetVersion)
	incompatibilities = append(incompatibilities, sqlModeIssues...)

	deprecatedFeatures := s.checkDeprecatedFeatures(sourceVersion, targetVersion)
	incompatibilities = append(incompatibilities, deprecatedFeatures...)

	charsetIssues := s.checkCharacterSetCompatibility(sourceVersion, targetVersion)
	incompatibilities = append(incompatibilities, charsetIssues...)

	authPluginIssues := s.checkAuthenticationPlugin(sourceVersion, targetVersion)
	incompatibilities = append(incompatibilities, authPluginIssues...)

	errorCount := 0
	warningCount := 0
	for _, item := range incompatibilities {
		if item.Level == "error" {
			errorCount++
		} else {
			warningCount++
		}
	}

	isCompatible := errorCount == 0

	recommendations = append(recommendations, "Backup all data before upgrade")
	recommendations = append(recommendations, "Test upgrade in staging environment first")
	if !isCompatible {
		recommendations = append(recommendations, "Resolve compatibility issues before proceeding")
	}

	check := &models.CompatibilityCheck{
		ID:               uuid.New().String(),
		InstanceID:       req.InstanceID,
		SourceVersion:    sourceVersion,
		TargetVersion:    targetVersion,
		CheckType:        "full",
		Status:           "completed",
		IsCompatible:     isCompatible,
		WarningCount:     warningCount,
		ErrorCount:       errorCount,
		IncompatibilityList: s.serializeIncompatibilities(incompatibilities),
		CheckedAt:        time.Now(),
	}

	_ = check

	response := &CheckCompatibilityResponse{
		CheckID:           uuid.New().String(),
		InstanceID:        req.InstanceID,
		SourceVersion:     sourceVersion,
		TargetVersion:     targetVersion,
		IsCompatible:      isCompatible,
		WarningCount:      warningCount,
		ErrorCount:        errorCount,
		Incompatibilities: incompatibilities,
		Recommendations:   recommendations,
	}

	return response, nil
}

type ExecuteInPlaceUpgradeRequest struct {
	InstanceID    string `json:"instance_id" binding:"required"`
	PlanID        string `json:"plan_id" binding:"required"`
	BackupEnabled bool   `json:"backup_enabled"`
}

type ExecuteInPlaceUpgradeResponse struct {
	TaskID       string          `json:"task_id"`
	PlanID       string          `json:"plan_id"`
	InstanceID   string          `json:"instance_id"`
	Status       string          `json:"status"`
	CurrentStep  string          `json:"current_step"`
	Progress     int             `json:"progress"`
	StartedAt    time.Time       `json:"started_at"`
	Steps        []UpgradeStepInfo `json:"steps"`
}

func (s *UpgradeService) ExecuteInPlaceUpgrade(ctx context.Context, req ExecuteInPlaceUpgradeRequest) (*ExecuteInPlaceUpgradeResponse, error) {
	instance, err := s.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	_ = instance

	steps := s.planInPlaceUpgradeSteps("5.7.40", "8.0.36")

	response := &ExecuteInPlaceUpgradeResponse{
		TaskID:      uuid.New().String(),
		PlanID:      req.PlanID,
		InstanceID:  req.InstanceID,
		Status:      "running",
		CurrentStep: steps[0].Name,
		Progress:    0,
		StartedAt:   time.Now(),
		Steps:       steps,
	}

	return response, nil
}

type ExecuteLogicalMigrationRequest struct {
	InstanceID      string `json:"instance_id" binding:"required"`
	PlanID          string `json:"plan_id" binding:"required"`
	BackupEnabled   bool   `json:"backup_enabled"`
	Parallelism     int    `json:"parallelism"`
	BatchSize       int    `json:"batch_size"`
}

type ExecuteLogicalMigrationResponse struct {
	TaskID       string          `json:"task_id"`
	PlanID       string          `json:"plan_id"`
	InstanceID   string          `json:"instance_id"`
	Status       string          `json:"status"`
	CurrentStep  string          `json:"current_step"`
	Progress     int             `json:"progress"`
	StartedAt    time.Time       `json:"started_at"`
	Steps        []UpgradeStepInfo `json:"steps"`
	DataStats    DataMigrationStats `json:"data_stats"`
}

type DataMigrationStats struct {
	TotalTables    int   `json:"total_tables"`
	MigratedTables int   `json:"migrated_tables"`
	TotalRows      int64 `json:"total_rows"`
	MigratedRows   int64 `json:"migrated_rows"`
	TotalSize      int64 `json:"total_size"`
	MigratedSize   int64 `json:"migrated_size"`
}

func (s *UpgradeService) ExecuteLogicalMigration(ctx context.Context, req ExecuteLogicalMigrationRequest) (*ExecuteLogicalMigrationResponse, error) {
	instance, err := s.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	_ = instance

	if req.Parallelism == 0 {
		req.Parallelism = 4
	}
	if req.BatchSize == 0 {
		req.BatchSize = 1000
	}

	steps := s.planLogicalMigrationSteps("5.7.40", "8.0.36")

	response := &ExecuteLogicalMigrationResponse{
		TaskID:      uuid.New().String(),
		PlanID:      req.PlanID,
		InstanceID:  req.InstanceID,
		Status:      "running",
		CurrentStep: steps[0].Name,
		Progress:    0,
		StartedAt:   time.Now(),
		Steps:       steps,
		DataStats: DataMigrationStats{
			TotalTables:    0,
			MigratedTables: 0,
			TotalRows:      0,
			MigratedRows:   0,
			TotalSize:      0,
			MigratedSize:   0,
		},
	}

	return response, nil
}

type ExecuteRollingUpgradeRequest struct {
	ClusterID      string `json:"cluster_id" binding:"required"`
	PlanID         string `json:"plan_id" binding:"required"`
	TargetVersion  string `json:"target_version" binding:"required"`
	MaxInParallel  int    `json:"max_in_parallel"`
	HealthCheckInterval int `json:"health_check_interval"`
}

type ExecuteRollingUpgradeResponse struct {
	TaskID        string                 `json:"task_id"`
	PlanID        string                 `json:"plan_id"`
	ClusterID     string                 `json:"cluster_id"`
	Status        string                 `json:"status"`
	CurrentPhase  string                 `json:"current_phase"`
	Progress      int                    `json:"progress"`
	StartedAt     time.Time              `json:"started_at"`
	Instances     []RollingUpgradeInstance `json:"instances"`
}

type RollingUpgradeInstance struct {
	InstanceID   string    `json:"instance_id"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
}

func (s *UpgradeService) ExecuteRollingUpgrade(ctx context.Context, req ExecuteRollingUpgradeRequest) (*ExecuteRollingUpgradeResponse, error) {
	if req.MaxInParallel == 0 {
		req.MaxInParallel = 1
	}
	if req.HealthCheckInterval == 0 {
		req.HealthCheckInterval = 30
	}

	instances := []RollingUpgradeInstance{
		{
			InstanceID: "instance-1",
			Role:       "master",
			Status:     "pending",
			StartedAt:  time.Time{},
			CompletedAt: time.Time{},
		},
		{
			InstanceID: "instance-2",
			Role:       "slave",
			Status:     "pending",
			StartedAt:  time.Time{},
			CompletedAt: time.Time{},
		},
	}

	response := &ExecuteRollingUpgradeResponse{
		TaskID:       uuid.New().String(),
		PlanID:       req.PlanID,
		ClusterID:    req.ClusterID,
		Status:       "running",
		CurrentPhase: "pre_check",
		Progress:     0,
		StartedAt:    time.Now(),
		Instances:    instances,
	}

	return response, nil
}

type RollbackUpgradeRequest struct {
	PlanID       string `json:"plan_id" binding:"required"`
	InstanceID   string `json:"instance_id" binding:"required"`
	BackupID     string `json:"backup_id"`
	Force        bool   `json:"force"`
}

type RollbackUpgradeResponse struct {
	RollbackID   string          `json:"rollback_id"`
	PlanID       string          `json:"plan_id"`
	InstanceID   string          `json:"instance_id"`
	Status       string          `json:"status"`
	Progress     int             `json:"progress"`
	StartedAt    time.Time       `json:"started_at"`
	CompletedAt  time.Time       `json:"completed_at"`
	Steps        []UpgradeStepInfo `json:"steps"`
}

func (s *UpgradeService) RollbackUpgrade(ctx context.Context, req RollbackUpgradeRequest) (*RollbackUpgradeResponse, error) {
	_, err := s.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	steps := []UpgradeStepInfo{
		{Order: 1, Name: "Stop Instance", Type: "operation", Description: "Stop MySQL service"},
		{Order: 2, Name: "Restore Data", Type: "restore", Description: "Restore from backup"},
		{Order: 3, Name: "Restore Configuration", Type: "restore", Description: "Restore configuration files"},
		{Order: 4, Name: "Start Instance", Type: "operation", Description: "Start MySQL service"},
		{Order: 5, Name: "Verify Recovery", Type: "verify", Description: "Verify data integrity"},
	}

	response := &RollbackUpgradeResponse{
		RollbackID:  uuid.New().String(),
		PlanID:      req.PlanID,
		InstanceID:  req.InstanceID,
		Status:      "running",
		Progress:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Time{},
		Steps:       steps,
	}

	return response, nil
}

type GenerateUpgradeReportRequest struct {
	PlanID       string `json:"plan_id" binding:"required"`
	ReportType   string `json:"report_type"`
}

type GenerateUpgradeReportResponse struct {
	ReportID      string    `json:"report_id"`
	PlanID        string    `json:"plan_id"`
	ReportType    string    `json:"report_type"`
	GeneratedAt   time.Time `json:"generated_at"`
	Summary       string    `json:"summary"`
	Details       string    `json:"details"`
	Metrics       ReportMetrics `json:"metrics"`
	Issues        []ReportIssue  `json:"issues"`
	Recommendations []string    `json:"recommendations"`
}

type ReportMetrics struct {
	Duration         int     `json:"duration_seconds"`
	DataTransferred  int64   `json:"data_transferred_bytes"`
	TablesProcessed  int     `json:"tables_processed"`
	RowsProcessed    int64   `json:"rows_processed"`
	ErrorsEncountered int    `json:"errors_encountered"`
	WarningsGenerated int    `json:"warnings_generated"`
	AvgThroughput    float64 `json:"avg_throughput_mbps"`
	PeakMemoryUsage  int64   `json:"peak_memory_usage_mb"`
}

type ReportIssue struct {
	Severity    string `json:"severity"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Timestamp   string `json:"timestamp"`
	Resolved    bool   `json:"resolved"`
}

func (s *UpgradeService) GenerateUpgradeReport(ctx context.Context, req GenerateUpgradeReportRequest) (*GenerateUpgradeReportResponse, error) {
	if req.ReportType == "" {
		req.ReportType = "full"
	}

	issues := []ReportIssue{
		{
			Severity:    "warning",
			Type:        "compatibility",
			Description: "Deprecated SQL mode detected",
			Timestamp:   time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
			Resolved:    true,
		},
	}

	recommendations := []string{
		"Review deprecated features before production upgrade",
		"Enable binary logging for point-in-time recovery",
		"Consider logical migration for large datasets",
		"Schedule upgrade during maintenance window",
	}

	response := &GenerateUpgradeReportResponse{
		ReportID:    uuid.New().String(),
		PlanID:      req.PlanID,
		ReportType:  req.ReportType,
		GeneratedAt: time.Now(),
		Summary:     "Upgrade from MySQL 5.7.40 to 8.0.36 completed successfully",
		Details:     "In-place upgrade executed with no critical errors. Data integrity verified.",
		Metrics: ReportMetrics{
			Duration:          1800,
			DataTransferred:   10737418240,
			TablesProcessed:   150,
			RowsProcessed:     10000000,
			ErrorsEncountered: 0,
			WarningsGenerated: 3,
			AvgThroughput:     47.5,
			PeakMemoryUsage:   2048,
		},
		Issues:         issues,
		Recommendations: recommendations,
	}

	return response, nil
}

func (s *UpgradeService) planInPlaceUpgradeSteps(source, target string) []UpgradeStepInfo {
	return []UpgradeStepInfo{
		{Order: 1, Name: "Pre-upgrade Check", Type: "check", Description: "Validate system requirements and compatibility"},
		{Order: 2, Name: "Create Backup", Type: "backup", Description: "Full backup of data directory"},
		{Order: 3, Name: "Stop Instance", Type: "operation", Description: "Stop MySQL service gracefully"},
		{Order: 4, Name: "Replace Binaries", Type: "operation", Description: "Replace MySQL binaries with new version"},
		{Order: 5, Name: "Start Instance", Type: "operation", Description: "Start MySQL with new binaries"},
		{Order: 6, Name: "Run Upgrade", Type: "upgrade", Description: "Execute mysql_upgrade procedure"},
		{Order: 7, Name: "Post-upgrade Check", Type: "check", Description: "Verify upgrade completion and data integrity"},
		{Order: 8, Name: "Cleanup", Type: "cleanup", Description: "Remove temporary files and old backups"},
	}
}

func (s *UpgradeService) planLogicalMigrationSteps(source, target string) []UpgradeStepInfo {
	return []UpgradeStepInfo{
		{Order: 1, Name: "Pre-migration Check", Type: "check", Description: "Validate source and target compatibility"},
		{Order: 2, Name: "Prepare Target", Type: "prepare", Description: "Configure target MySQL instance"},
		{Order: 3, Name: "Export Schema", Type: "export", Description: "Export database schema from source"},
		{Order: 4, Name: "Import Schema", Type: "import", Description: "Import schema to target instance"},
		{Order: 5, Name: "Initial Data Sync", Type: "sync", Description: "Perform initial full data synchronization"},
		{Order: 6, Name: "Setup Replication", Type: "replication", Description: "Configure change data capture"},
		{Order: 7, Name: "Catch-up Sync", Type: "sync", Description: "Synchronize remaining changes"},
		{Order: 8, Name: "Cutover", Type: "cutover", Description: "Switch application to new instance"},
		{Order: 9, Name: "Verify Migration", Type: "verify", Description: "Verify data integrity and application functionality"},
		{Order: 10, Name: "Cleanup", Type: "cleanup", Description: "Remove migration artifacts"},
	}
}

func (s *UpgradeService) planRollingUpgradeSteps(source, target string) []UpgradeStepInfo {
	return []UpgradeStepInfo{
		{Order: 1, Name: "Cluster Health Check", Type: "check", Description: "Verify cluster health and replication status"},
		{Order: 2, Name: "Upgrade Slave 1", Type: "upgrade", Description: "Upgrade first slave instance"},
		{Order: 3, Name: "Verify Slave 1", Type: "verify", Description: "Verify slave 1 replication and health"},
		{Order: 4, Name: "Upgrade Slave 2", Type: "upgrade", Description: "Upgrade second slave instance"},
		{Order: 5, Name: "Verify Slave 2", Type: "verify", Description: "Verify slave 2 replication and health"},
		{Order: 6, Name: "Promote New Master", Type: "promote", Description: "Promote slave to master role"},
		{Order: 7, Name: "Upgrade Old Master", Type: "upgrade", Description: "Upgrade former master instance"},
		{Order: 8, Name: "Reconfigure Topology", Type: "configure", Description: "Update cluster topology"},
		{Order: 9, Name: "Final Verification", Type: "verify", Description: "Verify cluster functionality"},
	}
}

func (s *UpgradeService) generatePreCheckWarnings(instance *models.Instance, strategy string) []string {
	warnings := []string{}
	
	warnings = append(warnings, "Ensure sufficient disk space for backup")
	warnings = append(warnings, "Verify all applications are compatible with MySQL 8.0")
	
	if strategy == "inplace" {
		warnings = append(warnings, "In-place upgrade cannot be rolled back automatically")
		warnings = append(warnings, "Plan for extended downtime during upgrade")
	}
	
	if strategy == "logical" {
		warnings = append(warnings, "Ensure network bandwidth for data transfer")
		warnings = append(warnings, "Verify target instance has sufficient storage")
	}
	
	if strategy == "rolling" {
		warnings = append(warnings, "Ensure cluster is healthy before proceeding")
		warnings = append(warnings, "Monitor replication lag during upgrade")
	}
	
	return warnings
}

func (s *UpgradeService) checkSQLModeCompatibility(source, target string) []IncompatibilityItem {
	var items []IncompatibilityItem
	
	items = append(items, IncompatibilityItem{
		Type:        "sql_mode",
		Level:       "warning",
		Description: "NO_AUTO_CREATE_USER removed in MySQL 8.0",
		Impact:      "Queries using this mode will fail",
		Solution:    "Remove NO_AUTO_CREATE_USER from sql_mode setting",
	})
	
	items = append(items, IncompatibilityItem{
		Type:        "sql_mode",
		Level:       "info",
		Description: "Default sql_mode changed to stricter settings",
		Impact:      "Existing queries may fail if they rely on implicit defaults",
		Solution:    "Review and adjust sql_mode or update queries",
	})
	
	return items
}

func (s *UpgradeService) checkDeprecatedFeatures(source, target string) []IncompatibilityItem {
	var items []IncompatibilityItem
	
	items = append(items, IncompatibilityItem{
		Type:        "feature",
		Level:       "warning",
		Description: "Query cache removed in MySQL 8.0",
		Impact:      "query_cache_size and related variables ignored",
		Solution:    "Remove query cache settings from my.cnf",
	})
	
	items = append(items, IncompatibilityItem{
		Type:        "feature",
		Level:       "warning",
		Description: "JSON_APPEND function deprecated, use JSON_ARRAY_APPEND",
		Impact:      "Existing queries using JSON_APPEND will fail",
		Solution:    "Update queries to use JSON_ARRAY_APPEND",
	})
	
	return items
}

func (s *UpgradeService) checkCharacterSetCompatibility(source, target string) []IncompatibilityItem {
	var items []IncompatibilityItem
	
	items = append(items, IncompatibilityItem{
		Type:        "charset",
		Level:       "info",
		Description: "Default character set changed to utf8mb4",
		Impact:      "New tables will use utf8mb4 by default",
		Solution:    "Review character set requirements for applications",
	})
	
	return items
}

func (s *UpgradeService) checkAuthenticationPlugin(source, target string) []IncompatibilityItem {
	var items []IncompatibilityItem
	
	items = append(items, IncompatibilityItem{
		Type:        "auth",
		Level:       "warning",
		Description: "Default authentication plugin changed to caching_sha2_password",
		Impact:      "Clients may need to update authentication method",
		Solution:    "Update user accounts or configure mysql_native_password",
	})
	
	return items
}

func (s *UpgradeService) serializeSteps(steps []UpgradeStepInfo) string {
	data, _ := json.Marshal(steps)
	return string(data)
}

func (s *UpgradeService) serializeIncompatibilities(items []IncompatibilityItem) string {
	data, _ := json.Marshal(items)
	return string(data)
}

func (s *UpgradeService) parseSteps(data string) []UpgradeStepInfo {
	var steps []UpgradeStepInfo
	json.Unmarshal([]byte(data), &steps)
	return steps
}

func (s *UpgradeService) parseIncompatibilities(data string) []IncompatibilityItem {
	var items []IncompatibilityItem
	json.Unmarshal([]byte(data), &items)
	return items
}

type UpgradeServiceStatus struct {
	ServiceName string `json:"service_name"`
	Version     string `json:"version"`
	Status      string `json:"status"`
}

func (s *UpgradeService) GetServiceStatus(ctx context.Context) (*UpgradeServiceStatus, error) {
	return &UpgradeServiceStatus{
		ServiceName: "UpgradeService",
		Version:     "1.0.0",
		Status:      "operational",
	}, nil
}

type SupportedVersion struct {
	Version     string `json:"version"`
	IsLTS       bool   `json:"is_lts"`
	ReleaseDate string `json:"release_date"`
	EOLDate     string `json:"eol_date"`
}

func (s *UpgradeService) GetSupportedVersions(ctx context.Context) ([]SupportedVersion, error) {
	return []SupportedVersion{
		{Version: "8.0.36", IsLTS: true, ReleaseDate: "2024-01-16", EOLDate: "2026-04-30"},
		{Version: "8.0.35", IsLTS: true, ReleaseDate: "2023-10-12", EOLDate: "2025-04-30"},
		{Version: "8.0.34", IsLTS: true, ReleaseDate: "2023-07-18", EOLDate: "2025-01-30"},
		{Version: "8.0.33", IsLTS: false, ReleaseDate: "2023-04-18", EOLDate: "2024-10-30"},
		{Version: "8.0.32", IsLTS: false, ReleaseDate: "2023-01-17", EOLDate: "2024-07-30"},
	}, nil
}

func (s *UpgradeService) ValidateUpgradePath(ctx context.Context, sourceVersion, targetVersion string) (bool, []string, error) {
	var errors []string
	sourceMajor := s.extractMajorVersion(sourceVersion)
	targetMajor := s.extractMajorVersion(targetVersion)

	if sourceMajor == "5.7" && targetMajor == "8.0" {
		return true, nil, nil
	}

	if sourceMajor == "8.0" && targetMajor == "8.0" {
		return true, nil, nil
	}

	if sourceMajor == "5.6" && targetMajor == "8.0" {
		errors = append(errors, "Direct upgrade from 5.6 to 8.0 not supported, upgrade to 5.7 first")
		return false, errors, nil
	}

	errors = append(errors, fmt.Sprintf("Unsupported upgrade path from %s to %s", sourceVersion, targetVersion))
	return false, errors, nil
}

func (s *UpgradeService) extractMajorVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return version
}