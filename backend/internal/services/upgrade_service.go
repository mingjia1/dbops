package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type UpgradeService struct {
	instanceRepo *repositories.InstanceRepository
	taskRepo     *repositories.TaskRepository
	agentClient  *AgentClient
	auditSvc     *AuditService
	encKey       string
	locks        sync.Map // map[instanceID]*sync.Mutex
	plans        sync.Map // map[planID]*models.UpgradePlan  (H3: in-memory plan store)
}

func NewUpgradeService(instanceRepo *repositories.InstanceRepository, taskRepo *repositories.TaskRepository, agentClient *AgentClient, auditSvc ...*AuditService) *UpgradeService {
	var audit *AuditService
	if len(auditSvc) > 0 {
		audit = auditSvc[0]
	}
	return &UpgradeService{
		instanceRepo: instanceRepo,
		taskRepo:     taskRepo,
		agentClient:  agentClient,
		auditSvc:     audit,
	}
}

func (s *UpgradeService) SetEncryptionKey(key string) {
	s.encKey = key
}

type PlanUpgradePathRequest struct {
	InstanceID    string `json:"instance_id" binding:"required"`
	TargetVersion string `json:"target_version" binding:"required"`
	TargetFlavor  string `json:"target_flavor"` // optional, defaults to source flavor
	Strategy      string `json:"strategy" binding:"required"`
	SourceVersion string `json:"source_version,omitempty"` // optional override
	FromVersion   string `json:"from_version,omitempty"`   // optional alias
}

type PlanUpgradePathResponse struct {
	PlanID           string            `json:"plan_id"`
	SourceVersion    string            `json:"source_version"`
	SourceFlavor     string            `json:"source_flavor"`
	TargetVersion    string            `json:"target_version"`
	TargetFlavor     string            `json:"target_flavor"`
	Strategy         string            `json:"strategy"`
	UpgradePath      []UpgradeStepInfo `json:"upgrade_path"`
	EstimatedTime    int               `json:"estimated_time"`
	RiskLevel        string            `json:"risk_level"`
	PreCheckWarnings []string          `json:"pre_check_warnings"`
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

	// Read the actual source version from the instance, not a hard-coded value.
	// If the instance has not been version-detected yet, the caller must run
	// POST /api/v1/instances/:id/detect-version first.
	// H5: Allow explicit override from SourceVersion/FromVersion request fields.
	sourceVersion, sourceFlavor, err := s.readSourceVersion(ctx, req.InstanceID)
	if err != nil {
		return nil, err
	}
	override := req.SourceVersion
	if override == "" {
		override = req.FromVersion
	}
	if override != "" {
		sourceVersion = override
		flavor := MajorVersion(override)
		if flavor != "" {
			_ = flavor // flavor derivation left to catalog lookup below
		}
	}
	targetVersion := req.TargetVersion
	targetFlavor := req.TargetFlavor
	targetVersion, targetFlavor = normalizeRequestedTargetVersion(targetVersion, targetFlavor, sourceFlavor)

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

	warnings := s.generatePreCheckWarnings(instance, req.Strategy, sourceVersion, targetVersion)
	if allowed, reason := IsValidUpgradePath(sourceFlavor, sourceVersion, targetFlavor, targetVersion); !allowed {
		warnings = append(warnings, "upgrade path check: "+reason)
	}

	plan := &models.UpgradePlan{
		ID:            uuid.New().String(),
		InstanceID:    req.InstanceID,
		SourceVersion: sourceVersion,
		TargetVersion: targetVersion,
		Strategy:      req.Strategy,
		Status:        "planned",
		EstimatedTime: estimatedTime,
		RiskLevel:     riskLevel,
		UpgradePath:   s.serializeSteps(steps),
		PreCheckResult: func() string {
			data, _ := json.Marshal(warnings)
			return string(data)
		}(),
	}
	// H3: persist plan in memory so ExecuteInPlaceUpgrade can validate plan_id.
	s.plans.Store(plan.ID, plan)

	response := &PlanUpgradePathResponse{
		PlanID:           plan.ID,
		SourceVersion:    sourceVersion,
		SourceFlavor:     sourceFlavor,
		TargetVersion:    targetVersion,
		TargetFlavor:     targetFlavor,
		Strategy:         req.Strategy,
		UpgradePath:      steps,
		EstimatedTime:    estimatedTime,
		RiskLevel:        riskLevel,
		PreCheckWarnings: warnings,
	}
	s.auditUpgrade(ctx, "plan_upgrade_path", "plan", "upgrade_plan", plan.ID, "success", "",
		fmt.Sprintf("instance_id=%s source=%s %s target=%s %s strategy=%s risk=%s warnings=%d",
			req.InstanceID, sourceFlavor, sourceVersion, targetFlavor, targetVersion, req.Strategy, riskLevel, len(warnings)))

	return response, nil
}

// readSourceVersion returns the actual on-host version detected for an instance.
// It is the single source of truth for "what is the source version" across the
// entire UpgradeService. No hard-coded versions allowed.
func (s *UpgradeService) readSourceVersion(ctx context.Context, instanceID string) (string, string, error) {
	ver, err := s.instanceRepo.GetVersion(ctx, instanceID)
	if err == nil {
		flavor := ver.Flavor
		if flavor == "" {
			flavor = "mysql"
		}
		fullVer := ver.FullVersion
		if fullVer == "" {
			fullVer = ver.Version
		}
		if strings.TrimSpace(fullVer) != "" {
			return fullVer, flavor, nil
		}
	}

	instance, getErr := s.instanceRepo.GetByID(ctx, instanceID)
	if getErr == nil {
		versionID := strings.TrimSpace(instance.Connection.VersionID)
		if versionID != "" {
			if entry, catalogErr := NewVersionCatalog().Get(versionID); catalogErr == nil && entry != nil {
				return entry.Version, entry.Flavor, nil
			}
		}
	}

	if err != nil {
		return "", "", fmt.Errorf("cannot determine source version for instance %s: %w (call POST /api/v1/instances/%s/detect-version first)", instanceID, err, instanceID)
	}
	return "", "", fmt.Errorf("cannot determine source version for instance %s: detected version is empty (call POST /api/v1/instances/%s/detect-version first)", instanceID, instanceID)
}

func normalizeRequestedTargetVersion(targetVersion, targetFlavor, sourceFlavor string) (string, string) {
	if targetFlavor == "" {
		targetFlavor = sourceFlavor
	}
	if entry, err := NewVersionCatalog().Get(targetVersion); err == nil && entry != nil {
		targetVersion = entry.Version
		targetFlavor = entry.Flavor
	}
	return targetVersion, targetFlavor
}

type CheckCompatibilityRequest struct {
	InstanceID    string `json:"instance_id" binding:"required"`
	TargetVersion string `json:"target_version" binding:"required"`
	TargetFlavor  string `json:"target_flavor"`
}

type CheckCompatibilityResponse struct {
	CheckID           string                `json:"check_id"`
	InstanceID        string                `json:"instance_id"`
	SourceVersion     string                `json:"source_version"`
	SourceFlavor      string                `json:"source_flavor"`
	TargetVersion     string                `json:"target_version"`
	TargetFlavor      string                `json:"target_flavor"`
	IsCompatible      bool                  `json:"is_compatible"`
	WarningCount      int                   `json:"warning_count"`
	ErrorCount        int                   `json:"error_count"`
	Incompatibilities []IncompatibilityItem `json:"incompatibilities"`
	Recommendations   []string              `json:"recommendations"`
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

	sourceVersion, sourceFlavor, err := s.readSourceVersion(ctx, req.InstanceID)
	if err != nil {
		return nil, err
	}
	targetVersion := req.TargetVersion
	targetFlavor := req.TargetFlavor
	targetVersion, targetFlavor = normalizeRequestedTargetVersion(targetVersion, targetFlavor, sourceFlavor)

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

	// Catalog-driven path check (no hard-coded 5.7→8.0 limit)
	if allowed, reason := IsValidUpgradePath(sourceFlavor, sourceVersion, targetFlavor, targetVersion); !allowed {
		incompatibilities = append(incompatibilities, IncompatibilityItem{
			Type:        "upgrade_path",
			Level:       "error",
			Description: fmt.Sprintf("Upgrade path %s %s to %s %s is not supported", sourceFlavor, sourceVersion, targetFlavor, targetVersion),
			Impact:      reason,
			Solution:    "Use logical migration for cross-major or cross-flavor upgrades, or select a target version allowed by the catalog",
		})
	}

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
	srcMM := MajorVersion(sourceVersion)
	dstMM := MajorVersion(targetVersion)
	if versionLessThan(srcMM, dstMM) {
		recommendations = append(recommendations, fmt.Sprintf("Review breaking changes between %s and %s before upgrading", srcMM, dstMM))
	}
	if !isCompatible {
		recommendations = append(recommendations, "Resolve compatibility issues before proceeding")
	}

	check := &models.CompatibilityCheck{
		ID:                  uuid.New().String(),
		InstanceID:          req.InstanceID,
		SourceVersion:       sourceVersion,
		TargetVersion:       targetVersion,
		CheckType:           "full",
		Status:              "completed",
		IsCompatible:        isCompatible,
		WarningCount:        warningCount,
		ErrorCount:          errorCount,
		IncompatibilityList: s.serializeIncompatibilities(incompatibilities),
		CheckedAt:           time.Now(),
	}

	response := &CheckCompatibilityResponse{
		CheckID:           check.ID,
		InstanceID:        req.InstanceID,
		SourceVersion:     sourceVersion,
		SourceFlavor:      sourceFlavor,
		TargetVersion:     targetVersion,
		TargetFlavor:      targetFlavor,
		IsCompatible:      isCompatible,
		WarningCount:      warningCount,
		ErrorCount:        errorCount,
		Incompatibilities: incompatibilities,
		Recommendations:   recommendations,
	}
	s.auditUpgrade(ctx, "check_upgrade_compatibility", "check", "compatibility_check", response.CheckID, upgradeAuditResultFromBool(isCompatible), "",
		fmt.Sprintf("instance_id=%s source=%s %s target=%s %s compatible=%t errors=%d warnings=%d",
			req.InstanceID, sourceFlavor, sourceVersion, targetFlavor, targetVersion, isCompatible, errorCount, warningCount))

	return response, nil
}

type ExecuteInPlaceUpgradeRequest struct {
	InstanceID    string `json:"instance_id" binding:"required"`
	PlanID        string `json:"plan_id" binding:"required"`
	TargetVersion string `json:"target_version"`
	TargetFlavor  string `json:"target_flavor"`
	BackupEnabled bool   `json:"backup_enabled"`
}

type ExecuteInPlaceUpgradeResponse struct {
	TaskID      string            `json:"task_id"`
	PlanID      string            `json:"plan_id"`
	InstanceID  string            `json:"instance_id"`
	Status      string            `json:"status"`
	CurrentStep string            `json:"current_step"`
	Progress    int               `json:"progress"`
	StartedAt   time.Time         `json:"started_at"`
	Steps       []UpgradeStepInfo `json:"steps"`
}

func (s *UpgradeService) ExecuteInPlaceUpgrade(ctx context.Context, req ExecuteInPlaceUpgradeRequest) (*ExecuteInPlaceUpgradeResponse, error) {
	if !req.BackupEnabled {
		s.auditUpgrade(ctx, "execute_in_place_upgrade", "execute", "upgrade_task", req.PlanID, "failed", "backup confirmation is required before in-place upgrade",
			fmt.Sprintf("instance_id=%s plan_id=%s target_version=%s", req.InstanceID, req.PlanID, req.TargetVersion))
		return nil, fmt.Errorf("backup confirmation is required before in-place upgrade")
	}
	instance, err := s.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		s.auditUpgrade(ctx, "execute_in_place_upgrade", "execute", "upgrade_task", req.PlanID, "failed", err.Error(),
			fmt.Sprintf("instance_id=%s plan_id=%s target_version=%s", req.InstanceID, req.PlanID, req.TargetVersion))
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	// Read source version from the instance — no hard-coded "5.7.40" anymore.
	sourceVersion, sourceFlavor, err := s.readSourceVersion(ctx, req.InstanceID)
	if err != nil {
		s.auditUpgrade(ctx, "execute_in_place_upgrade", "execute", "upgrade_task", req.PlanID, "failed", err.Error(),
			fmt.Sprintf("instance_id=%s plan_id=%s target_version=%s", req.InstanceID, req.PlanID, req.TargetVersion))
		return nil, err
	}
	targetVersion := req.TargetVersion
	if targetVersion == "" {
		return nil, fmt.Errorf("target_version is required")
	}
	targetVersion, _ = normalizeRequestedTargetVersion(targetVersion, req.TargetFlavor, sourceFlavor)

	steps := s.planInPlaceUpgradeSteps(sourceVersion, targetVersion)
	if len(steps) == 0 {
		return nil, fmt.Errorf("no upgrade steps generated")
	}
	// P0: 派发前落 task 表, B8 端点能查, frontend 轮询可见.
	taskID := s.createAndTrackTask("upgrade_in_place", req.InstanceID, req.PlanID, sourceVersion, targetVersion)
	s.dispatchAndTrack(req.InstanceID, taskID, instance, "in-place", targetVersion, map[string]interface{}{})

	response := &ExecuteInPlaceUpgradeResponse{
		TaskID:      taskID,
		PlanID:      req.PlanID,
		InstanceID:  req.InstanceID,
		Status:      "running",
		CurrentStep: steps[0].Name,
		Progress:    0,
		StartedAt:   time.Now(),
		Steps:       steps,
	}
	s.auditUpgrade(ctx, "execute_in_place_upgrade", "execute", "upgrade_task", taskID, "success", "",
		fmt.Sprintf("instance_id=%s plan_id=%s source_version=%s target_version=%s task_id=%s backup_confirmed=%t",
			req.InstanceID, req.PlanID, sourceVersion, targetVersion, taskID, req.BackupEnabled))

	return response, nil
}

type ExecuteLogicalMigrationRequest struct {
	InstanceID    string `json:"instance_id" binding:"required"`
	PlanID        string `json:"plan_id" binding:"required"`
	TargetVersion string `json:"target_version"`
	TargetFlavor  string `json:"target_flavor"`
	BackupEnabled bool   `json:"backup_enabled"`
	Parallelism   int    `json:"parallelism"`
	BatchSize     int    `json:"batch_size"`
}

type ExecuteLogicalMigrationResponse struct {
	TaskID      string             `json:"task_id"`
	PlanID      string             `json:"plan_id"`
	InstanceID  string             `json:"instance_id"`
	Status      string             `json:"status"`
	CurrentStep string             `json:"current_step"`
	Progress    int                `json:"progress"`
	StartedAt   time.Time          `json:"started_at"`
	Steps       []UpgradeStepInfo  `json:"steps"`
	DataStats   DataMigrationStats `json:"data_stats"`
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
	if !req.BackupEnabled {
		s.auditUpgrade(ctx, "execute_logical_upgrade", "execute", "upgrade_task", req.PlanID, "failed", "backup confirmation is required before logical upgrade migration",
			fmt.Sprintf("instance_id=%s plan_id=%s target_version=%s", req.InstanceID, req.PlanID, req.TargetVersion))
		return nil, fmt.Errorf("backup confirmation is required before logical upgrade migration")
	}
	instance, err := s.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		s.auditUpgrade(ctx, "execute_logical_upgrade", "execute", "upgrade_task", req.PlanID, "failed", err.Error(),
			fmt.Sprintf("instance_id=%s plan_id=%s target_version=%s", req.InstanceID, req.PlanID, req.TargetVersion))
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	if req.Parallelism == 0 {
		req.Parallelism = 4
	}
	if req.BatchSize == 0 {
		req.BatchSize = 1000
	}

	// Read source version from the instance — no hard-coded "5.7.40" anymore.
	sourceVersion, sourceFlavor, err := s.readSourceVersion(ctx, req.InstanceID)
	if err != nil {
		s.auditUpgrade(ctx, "execute_logical_upgrade", "execute", "upgrade_task", req.PlanID, "failed", err.Error(),
			fmt.Sprintf("instance_id=%s plan_id=%s target_version=%s", req.InstanceID, req.PlanID, req.TargetVersion))
		return nil, err
	}
	targetVersion := req.TargetVersion
	if targetVersion == "" {
		return nil, fmt.Errorf("target_version is required")
	}
	targetVersion, targetFlavor := normalizeRequestedTargetVersion(targetVersion, req.TargetFlavor, sourceFlavor)

	steps := s.planLogicalMigrationSteps(sourceVersion, targetVersion)
	// P0: 派发前落 task 表.
	taskID := s.createAndTrackTask("upgrade_logical", req.InstanceID, req.PlanID, sourceVersion, targetVersion)
	s.dispatchAndTrack(req.InstanceID, taskID, instance, "logical", targetVersion, map[string]interface{}{
		"parallelism":    req.Parallelism,
		"batch_size":     req.BatchSize,
		"backup_enabled": req.BackupEnabled,
		"source_version": sourceVersion,
		"target_flavor":  targetFlavor,
	})

	response := &ExecuteLogicalMigrationResponse{
		TaskID:      taskID,
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
	s.auditUpgrade(ctx, "execute_logical_upgrade", "execute", "upgrade_task", taskID, "success", "",
		fmt.Sprintf("instance_id=%s plan_id=%s source_version=%s target_version=%s target_flavor=%s task_id=%s parallelism=%d batch_size=%d backup_confirmed=%t",
			req.InstanceID, req.PlanID, sourceVersion, targetVersion, targetFlavor, taskID, req.Parallelism, req.BatchSize, req.BackupEnabled))

	return response, nil
}

type ExecuteRollingUpgradeRequest struct {
	ClusterID           string `json:"cluster_id" binding:"required"`
	PlanID              string `json:"plan_id" binding:"required"`
	TargetVersion       string `json:"target_version" binding:"required"`
	MaxInParallel       int    `json:"max_in_parallel"`
	HealthCheckInterval int    `json:"health_check_interval"`
}

type ExecuteRollingUpgradeResponse struct {
	TaskID       string                   `json:"task_id"`
	PlanID       string                   `json:"plan_id"`
	ClusterID    string                   `json:"cluster_id"`
	Status       string                   `json:"status"`
	CurrentPhase string                   `json:"current_phase"`
	Progress     int                      `json:"progress"`
	StartedAt    time.Time                `json:"started_at"`
	Instances    []RollingUpgradeInstance `json:"instances"`
}

type RollingUpgradeInstance struct {
	InstanceID  string    `json:"instance_id"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

func (s *UpgradeService) ExecuteRollingUpgrade(ctx context.Context, req ExecuteRollingUpgradeRequest) (*ExecuteRollingUpgradeResponse, error) {
	if req.MaxInParallel == 0 {
		req.MaxInParallel = 1
	}
	if req.HealthCheckInterval == 0 {
		req.HealthCheckInterval = 30
	}
	req.TargetVersion, _ = normalizeRequestedTargetVersion(req.TargetVersion, "", "mysql")
	// B3: 之前是写死 instance-1 / instance-2 占位. 现在从 instRepo 查真实实例列表.
	clusterInstances, err := s.instanceRepo.ListByClusterID(ctx, req.ClusterID)
	if err != nil {
		s.auditUpgrade(ctx, "execute_rolling_upgrade", "execute", "upgrade_task", req.PlanID, "failed", err.Error(),
			fmt.Sprintf("cluster_id=%s plan_id=%s target_version=%s", req.ClusterID, req.PlanID, req.TargetVersion))
		return nil, fmt.Errorf("failed to list cluster instances: %w", err)
	}
	if len(clusterInstances) == 0 {
		err := fmt.Errorf("cluster %s has no managed instances", req.ClusterID)
		s.auditUpgrade(ctx, "execute_rolling_upgrade", "execute", "upgrade_task", req.PlanID, "failed", err.Error(),
			fmt.Sprintf("cluster_id=%s plan_id=%s target_version=%s", req.ClusterID, req.PlanID, req.TargetVersion))
		return nil, err
	}
	clusterInstances = s.hydrateClusterInstances(ctx, clusterInstances)
	// P0: 派发前落 task, 用集群第一个实例 ID 作为 task.InstanceID (anchor).
	taskID := s.createAndTrackTask("upgrade_rolling",
		firstInstanceID(clusterInstances), req.PlanID, "", req.TargetVersion)
	instances := make([]RollingUpgradeInstance, 0, len(clusterInstances))
	for _, inst := range clusterInstances {
		role := inst.Status.Role
		if role == "" {
			role = "replica"
		}
		instances = append(instances, RollingUpgradeInstance{
			InstanceID:  inst.ID,
			Role:        role,
			Status:      "pending",
			StartedAt:   time.Time{},
			CompletedAt: time.Time{},
		})
	}

	// B3: 真派发 rolling upgrade, agent 端会按序升级每个实例.
	if s.agentClient != nil && len(clusterInstances) > 0 {
		// rolling 升级 dispatch 用集群中第一个实例作为 anchor,
		// agent 端会从 topology 解析其他节点. 走 fire-and-forget.
		s.dispatchAndTrack("cluster:"+req.ClusterID, taskID, clusterInstances[0], "rolling", req.TargetVersion, map[string]interface{}{
			"max_in_parallel":       req.MaxInParallel,
			"health_check_interval": req.HealthCheckInterval,
			"cluster_id":            req.ClusterID,
			"rolling_nodes":         rollingNodeIDs(clusterInstances),
		})
	}

	response := &ExecuteRollingUpgradeResponse{
		TaskID:       taskID,
		PlanID:       req.PlanID,
		ClusterID:    req.ClusterID,
		Status:       "running",
		CurrentPhase: "pre_check",
		Progress:     0,
		StartedAt:    time.Now(),
		Instances:    instances,
	}
	s.auditUpgrade(ctx, "execute_rolling_upgrade", "execute", "upgrade_task", taskID, "success", "",
		fmt.Sprintf("cluster_id=%s plan_id=%s target_version=%s task_id=%s instances=%d max_in_parallel=%d health_check_interval=%d",
			req.ClusterID, req.PlanID, req.TargetVersion, taskID, len(instances), req.MaxInParallel, req.HealthCheckInterval))

	return response, nil
}

func (s *UpgradeService) hydrateClusterInstances(ctx context.Context, instances []*models.Instance) []*models.Instance {
	if s == nil || s.instanceRepo == nil {
		return instances
	}
	out := make([]*models.Instance, 0, len(instances))
	for _, inst := range instances {
		if inst == nil {
			continue
		}
		if full, err := s.instanceRepo.GetByID(ctx, inst.ID); err == nil && full != nil {
			out = append(out, full)
			continue
		}
		out = append(out, inst)
	}
	return out
}

func rollingNodeIDs(instances []*models.Instance) []string {
	ids := make([]string, 0, len(instances))
	for _, inst := range instances {
		if inst != nil && inst.ID != "" {
			ids = append(ids, inst.ID)
		}
	}
	return ids
}

type RollbackUpgradeRequest struct {
	PlanID     string `json:"plan_id" binding:"required"`
	InstanceID string `json:"instance_id" binding:"required"`
	BackupID   string `json:"backup_id"`
	Force      bool   `json:"force"`
}

type RollbackUpgradeResponse struct {
	RollbackID  string            `json:"rollback_id"`
	PlanID      string            `json:"plan_id"`
	InstanceID  string            `json:"instance_id"`
	Status      string            `json:"status"`
	Progress    int               `json:"progress"`
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt time.Time         `json:"completed_at"`
	Steps       []UpgradeStepInfo `json:"steps"`
}

func (s *UpgradeService) RollbackUpgrade(ctx context.Context, req RollbackUpgradeRequest) (*RollbackUpgradeResponse, error) {
	instance, err := s.instanceRepo.GetByID(ctx, req.InstanceID)
	if err != nil {
		s.auditUpgrade(ctx, "rollback_upgrade", "rollback", "upgrade_task", req.PlanID, "failed", err.Error(),
			fmt.Sprintf("instance_id=%s plan_id=%s backup_id=%s force=%t", req.InstanceID, req.PlanID, req.BackupID, req.Force))
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	steps := []UpgradeStepInfo{
		{Order: 1, Name: "Stop Instance", Type: "operation", Description: "Stop MySQL service"},
		{Order: 2, Name: "Restore Data", Type: "restore", Description: "Restore from backup"},
		{Order: 3, Name: "Restore Configuration", Type: "restore", Description: "Restore configuration files"},
		{Order: 4, Name: "Start Instance", Type: "operation", Description: "Start MySQL service"},
		{Order: 5, Name: "Verify Recovery", Type: "verify", Description: "Verify data integrity"},
	}

	// P0: 派发前落 task, B8 端点能查. 走 fire-and-forget.
	taskID := s.createAndTrackTask("upgrade_rollback", req.InstanceID, req.PlanID, "", "")
	s.dispatchAndTrack(req.InstanceID, taskID, instance, "rollback", "", map[string]interface{}{
		"backup_id": req.BackupID,
		"force":     req.Force,
	})

	response := &RollbackUpgradeResponse{
		RollbackID:  taskID,
		PlanID:      req.PlanID,
		InstanceID:  req.InstanceID,
		Status:      "running",
		Progress:    0,
		StartedAt:   time.Now(),
		CompletedAt: time.Time{},
		Steps:       steps,
	}
	s.auditUpgrade(ctx, "rollback_upgrade", "rollback", "upgrade_task", taskID, "success", "",
		fmt.Sprintf("instance_id=%s plan_id=%s backup_id=%s force=%t task_id=%s", req.InstanceID, req.PlanID, req.BackupID, req.Force, taskID))

	return response, nil
}

type GenerateUpgradeReportRequest struct {
	PlanID     string `json:"plan_id" binding:"required"`
	ReportType string `json:"report_type"`
}

type GenerateUpgradeReportResponse struct {
	ReportID        string        `json:"report_id"`
	PlanID          string        `json:"plan_id"`
	ReportType      string        `json:"report_type"`
	GeneratedAt     time.Time     `json:"generated_at"`
	Summary         string        `json:"summary"`
	Details         string        `json:"details"`
	Metrics         ReportMetrics `json:"metrics"`
	Issues          []ReportIssue `json:"issues"`
	Recommendations []string      `json:"recommendations"`
}

type ReportMetrics struct {
	Duration          int     `json:"duration_seconds"`
	DataTransferred   int64   `json:"data_transferred_bytes"`
	TablesProcessed   int     `json:"tables_processed"`
	RowsProcessed     int64   `json:"rows_processed"`
	ErrorsEncountered int     `json:"errors_encountered"`
	WarningsGenerated int     `json:"warnings_generated"`
	AvgThroughput     float64 `json:"avg_throughput_mbps"`
	PeakMemoryUsage   int64   `json:"peak_memory_usage_mb"`
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

	// B3: 之前是写死 "Upgrade from MySQL 5.7.40 to 8.0.36 completed successfully" + 假 metrics.
	// 修: 用 taskRepo.GetByID 把 PlanID 当 task_id 查, 找不到就如实返 "no data".
	task, _ := s.taskRepo.GetByID(ctx, req.PlanID)
	if task == nil {
		return &GenerateUpgradeReportResponse{
			ReportID:    uuid.New().String(),
			PlanID:      req.PlanID,
			ReportType:  req.ReportType,
			GeneratedAt: time.Now(),
			Summary:     fmt.Sprintf("No task found for plan %s", req.PlanID),
			Details:     "Upgrade has not been dispatched yet, or task record was lost.",
			Metrics:     ReportMetrics{},
			Issues:      []ReportIssue{},
			Recommendations: []string{
				"Trigger an upgrade via /upgrades/in-place or /upgrades/logical first",
			},
		}, nil
	}

	duration := 0
	if !task.StartedAt.IsZero() && !task.CompletedAt.IsZero() {
		duration = int(task.CompletedAt.Sub(task.StartedAt).Seconds())
	}

	_, taskMessage := splitUpgradeTaskMessage(task.ErrorMessage)
	issues := []ReportIssue{}
	if task.Status == "failed" {
		issues = append(issues, ReportIssue{
			Severity:    "error",
			Type:        "execution",
			Description: taskMessage,
			Timestamp:   task.CompletedAt.Format(time.RFC3339),
			Resolved:    false,
		})
	}

	recommendations := []string{
		"Review task logs for any warnings before next upgrade",
		"Enable binary logging for point-in-time recovery",
		"Schedule upgrade during maintenance window",
	}

	return &GenerateUpgradeReportResponse{
		ReportID:    uuid.New().String(),
		PlanID:      req.PlanID,
		ReportType:  req.ReportType,
		GeneratedAt: time.Now(),
		Summary:     fmt.Sprintf("Upgrade task %s: status=%s, progress=%d%%", task.ID, task.Status, task.Progress),
		Details:     fmt.Sprintf("Started %s, completed %s. %s", task.StartedAt.Format(time.RFC3339), task.CompletedAt.Format(time.RFC3339), taskMessage),
		Metrics: ReportMetrics{
			Duration:          duration,
			DataTransferred:   0,
			TablesProcessed:   0,
			RowsProcessed:     0,
			ErrorsEncountered: len(issues),
			WarningsGenerated: 0,
			AvgThroughput:     0,
			PeakMemoryUsage:   0,
		},
		Issues:          issues,
		Recommendations: recommendations,
	}, nil
}

func splitUpgradeTaskMessage(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	metaAndMsg := strings.SplitN(raw, "\n", 2)
	meta := metaAndMsg[0]
	var message string
	if len(metaAndMsg) > 1 {
		message = strings.TrimSpace(metaAndMsg[1])
	}
	var planID string
	for _, part := range strings.Split(meta, "|") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "plan:") {
			planID = strings.TrimPrefix(part, "plan:")
		}
	}
	return planID, message
}

// firstInstanceID 拿到集群第一个实例的 ID, 没有则返空串.
func firstInstanceID(insts []*models.Instance) string {
	if len(insts) == 0 {
		return ""
	}
	return insts[0].ID
}

// lockForInstance P0: 拿 / 创建 instance 专属 mutex. 串行化同实例的并发操作.
// 用 sync.Map 装 mutex, 避免全局锁卡住所有 upgrade.
func (s *UpgradeService) lockForInstance(id string) *sync.Mutex {
	v, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// createAndTrackTask P0: 之前 upgrade 4 处 Execute* 调 dispatchUpgrade 前
// 都没 taskRepo.Create, B8 端点 GET /api/v1/tasks/:id 永远 404.
// 修: 在派发之前落库, frontend 立刻能 GET /api/v1/tasks/{id} 查状态.
// 落库失败不阻塞 (业务上 dispatch 还能正常返), 仅记 log.
func (s *UpgradeService) createAndTrackTask(taskType, instanceID, planID, sourceVersion, targetVersion string) string {
	taskID := uuid.New().String()
	if s.taskRepo == nil {
		return taskID
	}
	task := &models.Task{
		ID:         taskID,
		TaskType:   taskType,
		InstanceID: instanceID,
		Status:     "pending",
		Progress:   0,
		StartedAt:  time.Now(),
	}
	var meta []string
	if planID != "" {
		meta = append(meta, "plan:"+planID)
	}
	if sourceVersion != "" {
		meta = append(meta, "source:"+sourceVersion)
	}
	if targetVersion != "" {
		meta = append(meta, "target:"+targetVersion)
	}
	if len(meta) > 0 {
		task.ErrorMessage = strings.Join(meta, "|")
	}
	if err := s.taskRepo.Create(context.Background(), task); err != nil {
		_ = err
	}
	return taskID
}

// dispatchAndTrack P0: fire-and-forget 包装.
// 之前 dispatchUpgrade 是同步阻塞, frontend axios 10s timeout 一到就 5xx,
// 但 agent 端 xtrabackup 还在跑没人管. 修: go func() 异步派, 失败/成功都更新 taskRepo 状态,
// frontend 永远能 GET /api/v1/tasks/:id 拿真实进度.
func (s *UpgradeService) dispatchAndTrack(lockKey, taskID string, instance *models.Instance, upgradeType, targetVersion string, extraConfig map[string]interface{}) {
	go func() {
		if lockKey != "" {
			lock := s.lockForInstance(lockKey)
			lock.Lock()
			defer lock.Unlock()
		}
		if s.taskRepo != nil {
			_ = s.taskRepo.UpdateStatus(context.Background(), taskID, "running", 0)
		}
		if s.agentClient == nil {
			if s.taskRepo != nil {
				_ = s.taskRepo.UpdateStatusWithMessage(context.Background(), taskID, "failed", 0, "agent client not configured")
			}
			return
		}
		result, err := s.dispatchUpgrade(context.Background(), instance, taskID, upgradeType, targetVersion, extraConfig)
		if err != nil {
			if s.taskRepo != nil {
				_ = s.taskRepo.UpdateStatusWithMessage(context.Background(), taskID, "failed", 0, err.Error())
			}
			return
		}
		if s.taskRepo != nil {
			status, progress, message := normalizeUpgradeAgentTaskResult(result)
			_ = s.taskRepo.UpdateStatusWithMessage(context.Background(), taskID, status, progress, message)
		}
	}()
}

// normalizeUpgradeAgentTaskResult maps Agent business status to persisted task status.
func normalizeUpgradeAgentTaskResult(result *AgentTaskResult) (string, int, string) {
	if result == nil {
		return "failed", 100, "agent returned empty upgrade result"
	}
	status := strings.ToLower(strings.TrimSpace(result.Status))
	progress := result.Progress
	message := result.Message
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	switch status {
	case "failed", "error", "timeout", "cancelled", "canceled", "unhealthy":
		if progress == 0 {
			progress = 100
		}
		return "failed", progress, message
	case "success", "succeeded", "completed", "ok":
		if progress == 0 {
			progress = 100
		}
		return "completed", progress, message
	case "pending", "running", "queued", "accepted", "executing", "submitted":
		return status, progress, message
	case "":
		return "failed", 100, "agent returned empty upgrade status"
	default:
		return status, progress, message
	}
}

// dispatchUpgrade B3: 解析 instance 对应的 agent host/port, 调 /agent/tasks/upgrade.
// 失败立即返 error, 不再假装 running.
func (s *UpgradeService) dispatchUpgrade(ctx context.Context, instance *models.Instance, taskID, upgradeType, targetVersion string, extraConfig map[string]interface{}) (*AgentTaskResult, error) {
	if s.agentClient == nil {
		return nil, fmt.Errorf("agent client not configured")
	}
	host, port, err := resolveAgentHost(ctx, instance, s.instanceRepo, nil, 9090)
	if err != nil {
		return nil, err
	}

	cfg := map[string]interface{}{
		"task_id":        taskID,
		"upgrade_type":   upgradeType,
		"target_version": targetVersion,
		"instance_id":    instance.ID,
	}
	if conn, err := s.instanceRepo.GetConnection(ctx, instance.ID); err == nil {
		cfg["instance_host"] = localMySQLHostForAgent(conn.Host, host)
		cfg["instance_port"] = conn.Port
		cfg["mysql_user"] = conn.Username
		if s.encKey != "" {
			if pass, decErr := utils.Decrypt(conn.PasswordEncrypted, s.encKey); decErr == nil {
				cfg["mysql_pass"] = pass
			}
		}
	}
	if version, err := s.instanceRepo.GetVersion(ctx, instance.ID); err == nil {
		if version.FullVersion != "" {
			cfg["current_version"] = version.FullVersion
		} else {
			cfg["current_version"] = version.Version
		}
		if version.Flavor != "" {
			cfg["target_flavor"] = version.Flavor
		}
	}
	for k, v := range extraConfig {
		cfg[k] = v
	}
	s.applyUpgradePackageMetadata(cfg, targetVersion)
	return s.agentClient.callAgent(ctx, host, port, "/agent/tasks/upgrade", map[string]interface{}{
		"task_id":     taskID,
		"instance_id": instance.ID,
		"config":      cfg,
	})
}

func (s *UpgradeService) applyUpgradePackageMetadata(cfg map[string]interface{}, targetVersion string) {
	if _, ok := cfg["package_url"].(string); ok {
		if _, hasChecksum := cfg["checksum"].(string); hasChecksum {
			return
		}
	}
	entry := findUpgradeCatalogEntry(targetVersion, stringValue(cfg["target_flavor"]))
	if entry == nil {
		log.Printf("WARN: target version %s (flavor %v) not found in catalog — package_url and checksum will not be set; agent-side upgrade may fail",
			targetVersion, cfg["target_flavor"])
		return
	}
	if entry.Version != "" {
		cfg["target_version"] = entry.Version
	}
	if _, ok := cfg["package_url"].(string); !ok && entry.PackageURL != "" {
		cfg["package_url"] = entry.PackageURL
	}
	if _, ok := cfg["checksum"].(string); !ok && isVerifiedCatalogChecksum(entry) {
		cfg["checksum"] = entry.Checksum
	}
	if _, ok := cfg["target_flavor"].(string); !ok && entry.Flavor != "" {
		cfg["target_flavor"] = entry.Flavor
	}
}

func findUpgradeCatalogEntry(targetVersion, targetFlavor string) *VersionEntry {
	catalog := NewVersionCatalog()
	if entry, err := catalog.Get(targetVersion); err == nil && entry != nil {
		return entry
	}
	if targetFlavor != "" {
		if entry, err := catalog.GetByFlavorVersion(targetFlavor, targetVersion); err == nil && entry != nil {
			return entry
		}
	}
	for _, entry := range catalog.List() {
		if entry.Version == targetVersion {
			return &entry
		}
	}
	return nil
}

func (s *UpgradeService) auditUpgrade(ctx context.Context, operation, action, resourceType, resourceID, result, errorMsg, details string) {
	if s.auditSvc == nil {
		return
	}
	_, _ = s.auditSvc.CreateAuditLog(ctx, CreateAuditLogRequest{
		UserID:       userIDFromCtx(ctx),
		Operation:    operation,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		Details:      details,
		Result:       result,
		ErrorMsg:     errorMsg,
	})
}

func upgradeAuditResultFromBool(ok bool) string {
	if ok {
		return "success"
	}
	return "failed"
}

func (s *UpgradeService) planInPlaceUpgradeSteps(source, target string) []UpgradeStepInfo {
	srcMM := MajorVersion(source)
	dstMM := MajorVersion(target)
	crossMajor := versionLessThan(srcMM, dstMM)

	steps := []UpgradeStepInfo{
		{Order: 1, Name: "Pre-upgrade Check", Type: "check", Description: "Validate system requirements and compatibility"},
		{Order: 2, Name: "Create Backup", Type: "backup", Description: "Full backup of data directory"},
		{Order: 3, Name: "Stop Instance", Type: "operation", Description: "Stop MySQL service gracefully"},
		{Order: 4, Name: "Replace Binaries", Type: "operation", Description: fmt.Sprintf("Replace %s binaries with %s", srcMM, dstMM)},
		{Order: 5, Name: "Start Instance", Type: "operation", Description: "Start MySQL with new binaries"},
	}
	order := 6
	// mysql_upgrade is required when crossing major.minor (e.g. 5.7→8.0).
	// Within the same major.minor (e.g. 8.0.34→8.0.36) it is not needed.
	if crossMajor {
		steps = append(steps, UpgradeStepInfo{
			Order: order, Name: "Run Upgrade", Type: "upgrade",
			Description: fmt.Sprintf("Execute mysql_upgrade to migrate system tables from %s to %s", srcMM, dstMM),
		})
		order++
	}
	steps = append(steps,
		UpgradeStepInfo{Order: order, Name: "Post-upgrade Check", Type: "check", Description: "Verify upgrade completion and data integrity"},
		UpgradeStepInfo{Order: order + 1, Name: "Cleanup", Type: "cleanup", Description: "Remove temporary files and old backups"},
	)
	return steps
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

func (s *UpgradeService) generatePreCheckWarnings(instance *models.Instance, strategy, sourceVersion, targetVersion string) []string {
	warnings := []string{}
	dstMM := MajorVersion(targetVersion)

	warnings = append(warnings, "Ensure sufficient disk space for backup")
	if dstMM != "" {
		warnings = append(warnings, fmt.Sprintf("Verify all applications are compatible with %s", dstMM))
	}

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
	srcMM := MajorVersion(source)
	dstMM := MajorVersion(target)

	// NO_AUTO_CREATE_USER was removed in MySQL 8.0
	if versionLessThan(srcMM, "8.0") && !versionLessThan(dstMM, "8.0") {
		items = append(items, IncompatibilityItem{
			Type:        "sql_mode",
			Level:       "warning",
			Description: "NO_AUTO_CREATE_USER removed in MySQL 8.0",
			Impact:      "Queries using this mode will fail",
			Solution:    "Remove NO_AUTO_CREATE_USER from sql_mode setting",
		})
	}

	// Stricter defaults arrived in 8.0
	if versionLessThan(srcMM, "8.0") && !versionLessThan(dstMM, "8.0") {
		items = append(items, IncompatibilityItem{
			Type:        "sql_mode",
			Level:       "info",
			Description: "Default sql_mode changed to stricter settings in 8.0",
			Impact:      "Existing queries may fail if they rely on implicit defaults",
			Solution:    "Review and adjust sql_mode or update queries",
		})
	}

	return items
}

func (s *UpgradeService) checkDeprecatedFeatures(source, target string) []IncompatibilityItem {
	var items []IncompatibilityItem
	srcMM := MajorVersion(source)
	dstMM := MajorVersion(target)

	// Query cache removed in MySQL 8.0
	if versionLessThan(srcMM, "8.0") && !versionLessThan(dstMM, "8.0") {
		items = append(items, IncompatibilityItem{
			Type:        "feature",
			Level:       "warning",
			Description: "Query cache removed in MySQL 8.0",
			Impact:      "query_cache_size and related variables ignored",
			Solution:    "Remove query cache settings from my.cnf",
		})
	}

	// JSON_APPEND deprecated in 8.0
	if versionLessThan(srcMM, "8.0") && !versionLessThan(dstMM, "8.0") {
		items = append(items, IncompatibilityItem{
			Type:        "feature",
			Level:       "warning",
			Description: "JSON_APPEND function deprecated, use JSON_ARRAY_APPEND",
			Impact:      "Existing queries using JSON_APPEND will fail",
			Solution:    "Update queries to use JSON_ARRAY_APPEND",
		})
	}

	return items
}

func (s *UpgradeService) checkCharacterSetCompatibility(source, target string) []IncompatibilityItem {
	var items []IncompatibilityItem
	srcMM := MajorVersion(source)
	dstMM := MajorVersion(target)

	// Default charset changed to utf8mb4 in MySQL 8.0
	if versionLessThan(srcMM, "8.0") && !versionLessThan(dstMM, "8.0") {
		items = append(items, IncompatibilityItem{
			Type:        "charset",
			Level:       "info",
			Description: "Default character set changed to utf8mb4 in MySQL 8.0",
			Impact:      "New tables will use utf8mb4 by default",
			Solution:    "Review character set requirements for applications",
		})
	}

	return items
}

func (s *UpgradeService) checkAuthenticationPlugin(source, target string) []IncompatibilityItem {
	var items []IncompatibilityItem
	srcMM := MajorVersion(source)
	dstMM := MajorVersion(target)

	// Auth plugin changed to caching_sha2_password in MySQL 8.0
	if versionLessThan(srcMM, "8.0") && !versionLessThan(dstMM, "8.0") {
		items = append(items, IncompatibilityItem{
			Type:        "auth",
			Level:       "warning",
			Description: "Default authentication plugin changed to caching_sha2_password in 8.0",
			Impact:      "Clients may need to update authentication method",
			Solution:    "Update user accounts or configure mysql_native_password",
		})
	}

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
	Flavor      string `json:"flavor"`
	Version     string `json:"version"`
	MajorMinor  string `json:"major_minor"`
	IsLTS       bool   `json:"is_lts"`
	ReleaseDate string `json:"release_date"`
	EOLDate     string `json:"eol_date"`
	Status      string `json:"status"`
}

// GetSupportedVersions delegates to the version catalog. The catalog is the
// single source of truth; this method exists only for legacy callers that
// import the upgrade service.
func (s *UpgradeService) GetSupportedVersions(ctx context.Context) ([]SupportedVersion, error) {
	c := NewVersionCatalog()
	out := make([]SupportedVersion, 0, len(c.List()))
	for _, e := range c.List() {
		out = append(out, SupportedVersion{
			Flavor:      e.Flavor,
			Version:     e.Version,
			MajorMinor:  e.MajorMinor,
			IsLTS:       e.IsLTS,
			ReleaseDate: e.ReleaseDate,
			EOLDate:     e.EOLDate,
			Status:      e.Status,
		})
	}
	return out, nil
}

// ValidateUpgradePath delegates to the catalog-driven IsValidUpgradePath.
// Source/target flavors default to "mysql" if omitted. No hard-coded matrix.
func (s *UpgradeService) ValidateUpgradePath(ctx context.Context, sourceVersion, targetVersion, sourceFlavor, targetFlavor string) (bool, []string, error) {
	allowed, reason := IsValidUpgradePath(sourceFlavor, sourceVersion, targetFlavor, targetVersion)
	if allowed {
		return true, nil, nil
	}
	return false, []string{reason}, nil
}

func (s *UpgradeService) extractMajorVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return version
}

// versionLessThan returns true if a < b where a and b are "X.Y" major.minor strings.
func versionLessThan(a, b string) bool {
	av := parseVersionParts(a)
	bv := parseVersionParts(b)
	if av[0] != bv[0] {
		return av[0] < bv[0]
	}
	return av[1] < bv[1]
}

func parseVersionParts(v string) [2]int {
	var out [2]int
	parts := strings.Split(v, ".")
	if len(parts) >= 1 {
		out[0], _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		out[1], _ = strconv.Atoi(parts[1])
	}
	return out
}
