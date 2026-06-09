package services

import (
	"context"
	"errors"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockInstanceRepo struct {
	mock.Mock
}

func (m *MockInstanceRepo) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Instance), args.Error(1)
}

func (m *MockInstanceRepo) List(ctx context.Context, limit, offset int) ([]models.Instance, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Instance), args.Error(1)
}

func (m *MockInstanceRepo) Create(ctx context.Context, instance *models.Instance) error {
	args := m.Called(ctx, instance)
	return args.Error(0)
}

func (m *MockInstanceRepo) Update(ctx context.Context, instance *models.Instance) error {
	args := m.Called(ctx, instance)
	return args.Error(0)
}

func (m *MockInstanceRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockTaskRepo struct {
	mock.Mock
}

func (m *MockTaskRepo) Create(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepo) GetByID(ctx context.Context, id string) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskRepo) Update(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func TestUpgradeService_PlanUpgradePath(t *testing.T) {
	mockInstanceRepo := new(MockInstanceRepo)
	mockTaskRepo := new(MockTaskRepo)
	service := NewTestableUpgradeService(mockInstanceRepo, mockTaskRepo)

	ctx := context.Background()

	testInstance := &models.Instance{
		ID:        "instance-001",
		Name:      "test-instance",
		ClusterID: "cluster-001",
	}

	mockInstanceRepo.On("GetByID", ctx, "instance-001").Return(testInstance, nil)

	req := PlanUpgradePathRequest{
		InstanceID:    "instance-001",
		TargetVersion: "8.0.36",
		Strategy:      "inplace",
	}

	resp, err := service.PlanUpgradePath(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.PlanID)
	assert.Equal(t, "5.7.40", resp.SourceVersion)
	assert.Equal(t, "8.0.36", resp.TargetVersion)
	assert.Equal(t, "inplace", resp.Strategy)
	assert.Equal(t, 30, resp.EstimatedTime)
	assert.Equal(t, "medium", resp.RiskLevel)
	assert.NotEmpty(t, resp.UpgradePath)
	assert.NotEmpty(t, resp.PreCheckWarnings)

	mockInstanceRepo.AssertExpectations(t)
}

func TestUpgradeService_PlanUpgradePath_LogicalStrategy(t *testing.T) {
	mockInstanceRepo := new(MockInstanceRepo)
	mockTaskRepo := new(MockTaskRepo)
	service := NewTestableUpgradeService(mockInstanceRepo, mockTaskRepo)

	ctx := context.Background()

	testInstance := &models.Instance{
		ID:        "instance-002",
		Name:      "test-instance",
		ClusterID: "cluster-001",
	}

	mockInstanceRepo.On("GetByID", ctx, "instance-002").Return(testInstance, nil)

	req := PlanUpgradePathRequest{
		InstanceID:    "instance-002",
		TargetVersion: "8.0.36",
		Strategy:      "logical",
	}

	resp, err := service.PlanUpgradePath(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "logical", resp.Strategy)
	assert.Equal(t, 120, resp.EstimatedTime)
	assert.Equal(t, "low", resp.RiskLevel)
	assert.Len(t, resp.UpgradePath, 10)

	mockInstanceRepo.AssertExpectations(t)
}

func TestUpgradeService_PlanUpgradePath_RollingStrategy(t *testing.T) {
	mockInstanceRepo := new(MockInstanceRepo)
	mockTaskRepo := new(MockTaskRepo)
	service := NewTestableUpgradeService(mockInstanceRepo, mockTaskRepo)

	ctx := context.Background()

	testInstance := &models.Instance{
		ID:        "instance-003",
		Name:      "test-instance",
		ClusterID: "cluster-001",
	}

	mockInstanceRepo.On("GetByID", ctx, "instance-003").Return(testInstance, nil)

	req := PlanUpgradePathRequest{
		InstanceID:    "instance-003",
		TargetVersion: "8.0.36",
		Strategy:      "rolling",
	}

	resp, err := service.PlanUpgradePath(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "rolling", resp.Strategy)
	assert.Equal(t, 60, resp.EstimatedTime)
	assert.Equal(t, "medium", resp.RiskLevel)
	assert.Len(t, resp.UpgradePath, 9)

	mockInstanceRepo.AssertExpectations(t)
}

func TestUpgradeService_PlanUpgradePath_InstanceNotFound(t *testing.T) {
	mockInstanceRepo := new(MockInstanceRepo)
	mockTaskRepo := new(MockTaskRepo)
	service := NewTestableUpgradeService(mockInstanceRepo, mockTaskRepo)

	ctx := context.Background()

	mockInstanceRepo.On("GetByID", ctx, "non-existent").Return(nil, errors.New("instance not found"))

	req := PlanUpgradePathRequest{
		InstanceID:    "non-existent",
		TargetVersion: "8.0.36",
		Strategy:      "inplace",
	}

	resp, err := service.PlanUpgradePath(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	mockInstanceRepo.AssertExpectations(t)
}

func TestUpgradeService_CheckCompatibility(t *testing.T) {
	mockInstanceRepo := new(MockInstanceRepo)
	mockTaskRepo := new(MockTaskRepo)
	service := NewTestableUpgradeService(mockInstanceRepo, mockTaskRepo)

	ctx := context.Background()

	testInstance := &models.Instance{
		ID:        "instance-001",
		Name:      "test-instance",
		ClusterID: "cluster-001",
	}

	mockInstanceRepo.On("GetByID", ctx, "instance-001").Return(testInstance, nil)

	req := CheckCompatibilityRequest{
		InstanceID:    "instance-001",
		TargetVersion: "8.0.36",
	}

	resp, err := service.CheckCompatibility(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.CheckID)
	assert.Equal(t, "instance-001", resp.InstanceID)
	assert.NotEmpty(t, resp.Incompatibilities)
	assert.NotEmpty(t, resp.Recommendations)

	mockInstanceRepo.AssertExpectations(t)
}

func TestUpgradeService_CheckCompatibility_InstanceNotFound(t *testing.T) {
	mockInstanceRepo := new(MockInstanceRepo)
	mockTaskRepo := new(MockTaskRepo)
	service := NewTestableUpgradeService(mockInstanceRepo, mockTaskRepo)

	ctx := context.Background()

	mockInstanceRepo.On("GetByID", ctx, "non-existent").Return(nil, errors.New("instance not found"))

	req := CheckCompatibilityRequest{
		InstanceID:    "non-existent",
		TargetVersion: "8.0.36",
	}

	resp, err := service.CheckCompatibility(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)

	mockInstanceRepo.AssertExpectations(t)
}

func TestUpgradeService_Original_GetServiceStatus(t *testing.T) {
	service := NewUpgradeService(nil, nil, nil)

	ctx := context.Background()

	resp, err := service.GetServiceStatus(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "UpgradeService", resp.ServiceName)
	assert.Equal(t, "1.0.0", resp.Version)
	assert.Equal(t, "operational", resp.Status)
}

func TestUpgradeService_Original_GetSupportedVersions(t *testing.T) {
	service := NewUpgradeService(nil, nil, nil)

	ctx := context.Background()

	resp, err := service.GetSupportedVersions(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp)

	for _, v := range resp {
		assert.NotEmpty(t, v.Version)
	}
}

func TestUpgradeService_Original_ValidateUpgradePath(t *testing.T) {
	service := NewUpgradeService(nil, nil, nil)

	ctx := context.Background()

	valid, errors, err := service.ValidateUpgradePath(ctx, "5.7.40", "8.0.36", "mysql", "mysql")

	assert.NoError(t, err)
	assert.True(t, valid)
	assert.Empty(t, errors)
}

func TestUpgradeService_Original_ValidateUpgradePath_Invalid(t *testing.T) {
	service := NewUpgradeService(nil, nil, nil)

	ctx := context.Background()

	valid, errors, err := service.ValidateUpgradePath(ctx, "5.6.40", "8.0.36", "mysql", "mysql")

	assert.NoError(t, err)
	assert.False(t, valid)
	assert.NotEmpty(t, errors)
}

func TestUpgradeService_ExecuteRollingUpgradeEmptyClusterFailsWithoutTask(t *testing.T) {
	db := newTestDB()
	defer db.Close()
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil)

	resp, err := service.ExecuteRollingUpgrade(context.Background(), ExecuteRollingUpgradeRequest{
		ClusterID:     "cluster-001",
		PlanID:        "plan-001",
		TargetVersion: "8.0.36",
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "has no managed instances")
	tasks, listErr := taskRepo.ListByTypes(context.Background(), []string{"upgrade_rolling"}, 10, 0)
	assert.NoError(t, listErr)
	assert.Empty(t, tasks)
}

func TestUpgradeService_ExecuteRollingUpgrade_TaskIDIsPersisted(t *testing.T) {
	db := newTestDB()
	defer db.Close()
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil)
	createUpgradeTestInstance(t, context.Background(), instanceRepo, "cluster-history-inst-1", "cluster-history")

	resp, err := service.ExecuteRollingUpgrade(context.Background(), ExecuteRollingUpgradeRequest{
		ClusterID:     "cluster-history",
		PlanID:        "plan-history",
		TargetVersion: "8.0.36",
	})

	assert.NoError(t, err)
	task, err := taskRepo.GetByID(context.Background(), resp.TaskID)
	assert.NoError(t, err)
	assert.Equal(t, resp.TaskID, task.ID)
	assert.Equal(t, "upgrade_rolling", task.TaskType)

	tasks, err := taskRepo.ListByTypes(context.Background(), []string{"upgrade_rolling"}, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, resp.TaskID, tasks[0].ID)
}

func TestUpgradeService_ExecuteInPlaceRequiresBackupConfirmation(t *testing.T) {
	db := newTestDB()
	defer db.Close()
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil)

	resp, err := service.ExecuteInPlaceUpgrade(context.Background(), ExecuteInPlaceUpgradeRequest{
		InstanceID:    "instance-001",
		PlanID:        "plan-001",
		TargetVersion: "8.0.36",
		BackupEnabled: false,
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "backup confirmation")
	tasks, listErr := taskRepo.ListByTypes(context.Background(), []string{"upgrade_in_place"}, 10, 0)
	assert.NoError(t, listErr)
	assert.Empty(t, tasks)
}

func TestUpgradeService_ExecuteLogicalRequiresBackupConfirmation(t *testing.T) {
	db := newTestDB()
	defer db.Close()
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil)

	resp, err := service.ExecuteLogicalMigration(context.Background(), ExecuteLogicalMigrationRequest{
		InstanceID:    "instance-001",
		PlanID:        "plan-001",
		TargetVersion: "8.0.36",
		BackupEnabled: false,
	})

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "backup confirmation")
	tasks, listErr := taskRepo.ListByTypes(context.Background(), []string{"upgrade_logical"}, 10, 0)
	assert.NoError(t, listErr)
	assert.Empty(t, tasks)
}

func TestUpgradeService_Original_GenerateUpgradeReport(t *testing.T) {
	// B3: GenerateUpgradeReport 现在真用 taskRepo.GetByID. 共享 sqlite test DB.
	db := newTestDB()
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil)

	ctx := context.Background()

	req := GenerateUpgradeReportRequest{
		PlanID:     "plan-001",
		ReportType: "full",
	}

	resp, err := service.GenerateUpgradeReport(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.ReportID)
	assert.Equal(t, "plan-001", resp.PlanID)
	assert.NotEmpty(t, resp.Summary)
}

func TestUpgradeService_Original_extractMajorVersion(t *testing.T) {
	service := NewUpgradeService(nil, nil, nil)

	assert.Equal(t, "5.7", service.extractMajorVersion("5.7.40"))
	assert.Equal(t, "8.0", service.extractMajorVersion("8.0.36"))
}

func TestUpgradeService_PlanUpgradePathWritesAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "upgrade-auditor-001")
	db := newTestDB()
	instanceRepo, taskRepo, auditRepo, auditSvc := newUpgradeAuditTestRepos(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil, auditSvc)
	createUpgradeTestInstance(t, ctx, instanceRepo, "upgrade-inst-plan", "")

	resp, err := service.PlanUpgradePath(ctx, PlanUpgradePathRequest{
		InstanceID:    "upgrade-inst-plan",
		TargetVersion: "8.0.36",
		Strategy:      "inplace",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	logs, err := auditRepo.ListByResource(context.Background(), "upgrade_plan", resp.PlanID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "upgrade-auditor-001", logs[0].UserID)
	assert.Equal(t, "plan_upgrade_path", logs[0].Operation)
	assert.Equal(t, "success", logs[0].Result)
	assert.Contains(t, logs[0].Details, "instance_id=upgrade-inst-plan")
}

func TestUpgradeService_CheckCompatibilityWritesAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "upgrade-auditor-002")
	db := newTestDB()
	instanceRepo, taskRepo, auditRepo, auditSvc := newUpgradeAuditTestRepos(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil, auditSvc)
	createUpgradeTestInstance(t, ctx, instanceRepo, "upgrade-inst-check", "")

	resp, err := service.CheckCompatibility(ctx, CheckCompatibilityRequest{
		InstanceID:    "upgrade-inst-check",
		TargetVersion: "8.0.36",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	logs, err := auditRepo.ListByResource(context.Background(), "compatibility_check", resp.CheckID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "check_upgrade_compatibility", logs[0].Operation)
	assert.Contains(t, logs[0].Details, "instance_id=upgrade-inst-check")
}

func TestUpgradeService_ExecuteInPlaceWithoutBackupWritesFailedAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "upgrade-auditor-003")
	db := newTestDB()
	instanceRepo, taskRepo, auditRepo, auditSvc := newUpgradeAuditTestRepos(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil, auditSvc)

	resp, err := service.ExecuteInPlaceUpgrade(ctx, ExecuteInPlaceUpgradeRequest{
		InstanceID:    "upgrade-inst-denied",
		PlanID:        "plan-denied",
		TargetVersion: "8.0.36",
		BackupEnabled: false,
	})

	require.Error(t, err)
	require.Nil(t, resp)
	logs, err := auditRepo.ListByResource(context.Background(), "upgrade_task", "plan-denied", 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "execute_in_place_upgrade", logs[0].Operation)
	assert.Equal(t, "failed", logs[0].Result)
	assert.Contains(t, logs[0].ErrorMsg, "backup confirmation")
}

func TestUpgradeService_ExecuteInPlaceWritesAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "upgrade-auditor-004")
	db := newTestDB()
	instanceRepo, taskRepo, auditRepo, auditSvc := newUpgradeAuditTestRepos(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil, auditSvc)
	createUpgradeTestInstance(t, ctx, instanceRepo, "upgrade-inst-execute", "")

	resp, err := service.ExecuteInPlaceUpgrade(ctx, ExecuteInPlaceUpgradeRequest{
		InstanceID:    "upgrade-inst-execute",
		PlanID:        "plan-execute",
		TargetVersion: "8.0.36",
		BackupEnabled: true,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	logs, err := auditRepo.ListByResource(context.Background(), "upgrade_task", resp.TaskID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "execute_in_place_upgrade", logs[0].Operation)
	assert.Equal(t, "success", logs[0].Result)
	assert.Contains(t, logs[0].Details, "plan_id=plan-execute")
}

func TestUpgradeService_ExecuteLogicalWritesAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "upgrade-auditor-005")
	db := newTestDB()
	instanceRepo, taskRepo, auditRepo, auditSvc := newUpgradeAuditTestRepos(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil, auditSvc)
	createUpgradeTestInstance(t, ctx, instanceRepo, "upgrade-inst-logical", "")

	resp, err := service.ExecuteLogicalMigration(ctx, ExecuteLogicalMigrationRequest{
		InstanceID:    "upgrade-inst-logical",
		PlanID:        "plan-logical",
		TargetVersion: "8.0.36",
		BackupEnabled: true,
		Parallelism:   2,
		BatchSize:     500,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	logs, err := auditRepo.ListByResource(context.Background(), "upgrade_task", resp.TaskID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "execute_logical_upgrade", logs[0].Operation)
	assert.Equal(t, "success", logs[0].Result)
	assert.Contains(t, logs[0].Details, "plan_id=plan-logical")
	assert.Contains(t, logs[0].Details, "parallelism=2")
}

func TestUpgradeService_RollbackWritesAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "upgrade-auditor-006")
	db := newTestDB()
	instanceRepo, taskRepo, auditRepo, auditSvc := newUpgradeAuditTestRepos(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil, auditSvc)
	createUpgradeTestInstance(t, ctx, instanceRepo, "upgrade-inst-rollback", "")

	resp, err := service.RollbackUpgrade(ctx, RollbackUpgradeRequest{
		InstanceID: "upgrade-inst-rollback",
		PlanID:     "plan-rollback",
		BackupID:   "backup-001",
		Force:      true,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	logs, err := auditRepo.ListByResource(context.Background(), "upgrade_task", resp.RollbackID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "rollback_upgrade", logs[0].Operation)
	assert.Equal(t, "success", logs[0].Result)
	assert.Contains(t, logs[0].Details, "backup_id=backup-001")
}

func TestUpgradeService_ExecuteRollingWritesAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "upgrade-auditor-007")
	db := newTestDB()
	instanceRepo, taskRepo, auditRepo, auditSvc := newUpgradeAuditTestRepos(db)
	service := NewUpgradeService(instanceRepo, taskRepo, nil, auditSvc)
	createUpgradeTestInstance(t, ctx, instanceRepo, "upgrade-cluster-inst-1", "upgrade-cluster-audit")

	resp, err := service.ExecuteRollingUpgrade(ctx, ExecuteRollingUpgradeRequest{
		ClusterID:           "upgrade-cluster-audit",
		PlanID:              "plan-rolling",
		TargetVersion:       "8.0.36",
		MaxInParallel:       1,
		HealthCheckInterval: 15,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	logs, err := auditRepo.ListByResource(context.Background(), "upgrade_task", resp.TaskID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "execute_rolling_upgrade", logs[0].Operation)
	assert.Equal(t, "success", logs[0].Result)
	assert.Contains(t, logs[0].Details, "cluster_id=upgrade-cluster-audit")
	assert.Contains(t, logs[0].Details, "instances=1")
}

func TestUpgradeDispatchAddsCatalogPackageMetadata(t *testing.T) {
	db := newTestDB()
	service := NewUpgradeService(repositories.NewInstanceRepository(db), repositories.NewTaskRepository(db), nil)
	config := map[string]interface{}{}

	service.applyUpgradePackageMetadata(config, "mysql-8.0.36")

	assert.Equal(t, "8.0.36", config["target_version"])
	assert.Equal(t, "mysql", config["target_flavor"])
	assert.Contains(t, config["package_url"], "mysql-8.0.36")
	assert.NotContains(t, config, "checksum")
}

func TestUpgradeDispatchKeepsExplicitPackageURL(t *testing.T) {
	db := newTestDB()
	service := NewUpgradeService(repositories.NewInstanceRepository(db), repositories.NewTaskRepository(db), nil)
	explicitURL := "https://mirror.example/mysql.tar.gz"
	explicitChecksum := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	config := map[string]interface{}{
		"package_url": explicitURL,
		"checksum":    explicitChecksum,
	}

	service.applyUpgradePackageMetadata(config, "mysql-8.0.36")

	assert.Equal(t, explicitURL, config["package_url"])
	assert.Equal(t, explicitChecksum, config["checksum"])
}

func TestNormalizeUpgradeAgentTaskResultFailed(t *testing.T) {
	status, progress, message := normalizeUpgradeAgentTaskResult(&AgentTaskResult{
		Status:   "failed",
		Progress: 100,
		Message:  "upgrade precheck failed",
	})

	assert.Equal(t, "failed", status)
	assert.Equal(t, 100, progress)
	assert.Equal(t, "upgrade precheck failed", message)
}

func TestNormalizeUpgradeAgentTaskResultRunning(t *testing.T) {
	status, progress, message := normalizeUpgradeAgentTaskResult(&AgentTaskResult{
		Status:   "running",
		Progress: 35,
		Message:  "copying files",
	})

	assert.Equal(t, "running", status)
	assert.Equal(t, 35, progress)
	assert.Equal(t, "copying files", message)
}

func TestNormalizeUpgradeAgentTaskResultCompleted(t *testing.T) {
	status, progress, message := normalizeUpgradeAgentTaskResult(&AgentTaskResult{
		Status:  "success",
		Message: "upgrade completed",
	})

	assert.Equal(t, "completed", status)
	assert.Equal(t, 100, progress)
	assert.Equal(t, "upgrade completed", message)
}

func TestNormalizeUpgradeAgentTaskResultEmptyStatusFails(t *testing.T) {
	status, progress, message := normalizeUpgradeAgentTaskResult(&AgentTaskResult{})

	assert.Equal(t, "failed", status)
	assert.Equal(t, 100, progress)
	assert.Contains(t, message, "empty upgrade status")
}

func newUpgradeAuditTestRepos(db *repositories.Database) (*repositories.InstanceRepository, *repositories.TaskRepository, *repositories.AuditLogRepository, *AuditService) {
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	auditSvc := NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db))
	return instanceRepo, taskRepo, auditRepo, auditSvc
}

func createUpgradeTestInstance(t *testing.T, ctx context.Context, repo *repositories.InstanceRepository, id, clusterID string) {
	t.Helper()
	instance := &models.Instance{
		ID:        id,
		Name:      id,
		ClusterID: clusterID,
	}
	require.NoError(t, repo.Create(ctx, instance))
	require.NoError(t, repo.CreateVersion(ctx, &models.InstanceVersion{
		InstanceID:  id,
		Flavor:      "mysql",
		Version:     "5.7.40",
		FullVersion: "5.7.40",
	}))
}
