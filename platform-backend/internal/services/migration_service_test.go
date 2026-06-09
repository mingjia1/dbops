package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrationService_CreateTask(t *testing.T) {
	service := newTestMigrationService()

	ctx := context.Background()

	req := CreateMigrationTaskRequest{
		Name:             "Test Migration",
		SourceInstanceID: "source-001",
		TargetInstanceID: "target-001",
		Strategy:         models.MigrationStrategyPhysical,
		Config:           "{}",
	}

	taskID, err := service.CreateTask(ctx, req)

	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)
}

func TestMigrationService_ExecutePhysicalMigration(t *testing.T) {
	service := newTestMigrationService()

	ctx := context.Background()

	result, err := service.ExecutePhysicalMigration(ctx, "migration-001")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestMigrationService_TaskStatusTransition(t *testing.T) {
	service := newTestMigrationService()

	ctx := context.Background()

	req := CreateMigrationTaskRequest{
		Name:             "Status Test",
		SourceInstanceID: "src-001",
		TargetInstanceID: "tgt-001",
		Strategy:         models.MigrationStrategyGTID,
	}
	taskID, err := service.CreateTask(ctx, req)
	assert.NoError(t, err)
	assert.NotEmpty(t, taskID)
}

func TestMigrationService_SwitchWithoutTask(t *testing.T) {
	service := newTestMigrationService()
	ctx := context.Background()

	result, err := service.ExecuteSwitch(ctx, "nonexistent-task")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestMigrationService_ListTasks(t *testing.T) {
	service := newTestMigrationService()
	ctx := context.Background()

	tasks, err := service.ListTasks(ctx, "")
	assert.NoError(t, err)
	assert.NotNil(t, tasks)
}

func TestMigrationService_CancelTask(t *testing.T) {
	service := newTestMigrationService()
	ctx := context.Background()

	err := service.CancelTask(ctx, "nonexistent-task")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMigrationService_VerifyMigration(t *testing.T) {
	service := newTestMigrationService()
	ctx := context.Background()

	// 不存在的 task 应该返回 error 而不是静默成功.
	result, err := service.VerifyMigration(ctx, "migration-001")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveAgentHostDoesNotFallbackToLocalhost(t *testing.T) {
	host, port, err := resolveAgentHost(context.Background(), &models.Instance{ID: "inst-no-host"}, nil, nil, 9090)
	require.Error(t, err)
	assert.Empty(t, host)
	assert.Zero(t, port)
	assert.NotContains(t, err.Error(), "localhost")
}

func TestResolveAgentHostUsesConnectionHostWhenNoHostRepo(t *testing.T) {
	host, port, err := resolveAgentHost(context.Background(), &models.Instance{
		ID: "inst-conn",
		Connection: models.InstanceConnection{
			Host: "10.1.81.41",
		},
	}, nil, nil, 9090)
	require.NoError(t, err)
	assert.Equal(t, "10.1.81.41", host)
	assert.Equal(t, 9090, port)
}

func TestResolveAgentHostUsesManagedHostPort(t *testing.T) {
	ctx := context.Background()
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	host := &models.Host{
		ID:        "host-agent",
		Name:      "agent-host",
		Address:   "10.1.81.41",
		AgentPort: 19090,
		SSHPort:   22,
		SSHUser:   "root",
	}
	require.NoError(t, hostRepo.Create(ctx, host))
	hostID := host.ID

	resolvedHost, port, err := resolveAgentHost(ctx, &models.Instance{ID: "inst-host", HostID: &hostID}, nil, hostRepo, 9090)
	require.NoError(t, err)
	assert.Equal(t, "10.1.81.41", resolvedHost)
	assert.Equal(t, 19090, port)
}

func TestMigrationService_CreateTaskWritesAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "auditor-001")
	db := newTestDB()
	migrationRepo := repositories.NewMigrationRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	service := NewMigrationService(migrationRepo, repositories.NewInstanceRepository(db), repositories.NewHostRepository(db), newTestAgentClient(), NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db)))

	taskID, err := service.CreateTask(ctx, CreateMigrationTaskRequest{
		Name:             "audit migration",
		SourceInstanceID: "source-audit",
		TargetInstanceID: "target-audit",
		Strategy:         models.MigrationStrategyPhysical,
	})

	require.NoError(t, err)
	require.NotEmpty(t, taskID)
	logs, err := auditRepo.ListByResource(context.Background(), "migration_task", taskID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "auditor-001", logs[0].UserID)
	assert.Equal(t, "create_migration_task", logs[0].Operation)
	assert.Equal(t, "success", logs[0].Result)
	assert.Contains(t, logs[0].Details, "source_instance_id=source-audit")
}

func TestMigrationService_CancelTaskWritesAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "auditor-002")
	db := newTestDB()
	migrationRepo := repositories.NewMigrationRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	service := NewMigrationService(migrationRepo, repositories.NewInstanceRepository(db), repositories.NewHostRepository(db), newTestAgentClient(), NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db)))

	taskID, err := service.CreateTask(ctx, CreateMigrationTaskRequest{
		Name:             "cancel audit migration",
		SourceInstanceID: "source-cancel",
		TargetInstanceID: "target-cancel",
		Strategy:         models.MigrationStrategyGTID,
	})
	require.NoError(t, err)

	err = service.CancelTask(ctx, taskID)

	require.NoError(t, err)
	logs, err := auditRepo.ListByResource(context.Background(), "migration_task", taskID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "cancel_migration_task", logs[0].Operation)
	assert.Equal(t, "success", logs[0].Result)
}

func TestMigrationService_ExecuteFailureWritesAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "auditor-003")
	db := newTestDB()
	migrationRepo := repositories.NewMigrationRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	service := NewMigrationService(migrationRepo, instanceRepo, repositories.NewHostRepository(db), newTestAgentClient(), NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db)))

	taskID, err := service.CreateTask(ctx, CreateMigrationTaskRequest{
		Name:             "execute failure audit migration",
		SourceInstanceID: "source-missing-endpoint",
		TargetInstanceID: "target-missing-endpoint",
		Strategy:         models.MigrationStrategyPhysical,
	})
	require.NoError(t, err)
	require.NoError(t, instanceRepo.Create(ctx, &models.Instance{ID: "source-missing-endpoint"}))

	result, err := service.ExecutePhysicalMigration(ctx, taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, models.MigrationStatusFailed, result.Status)
	logs, err := auditRepo.ListByResource(context.Background(), "migration_task", taskID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "execute_migration", logs[0].Operation)
	assert.Equal(t, "failed", logs[0].Result)
	assert.Contains(t, logs[0].ErrorMsg, "cannot resolve agent endpoint")
}

func TestMigrationServiceExecuteWithoutAgentClientFailsTask(t *testing.T) {
	service, migrationRepo, taskID := newMigrationServiceWithoutAgent(t)

	result, err := service.ExecutePhysicalMigration(context.Background(), taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, models.MigrationStatusFailed, result.Status)
	assert.Contains(t, result.Message, "agent client not configured")
	task, err := migrationRepo.GetByID(context.Background(), taskID)
	require.NoError(t, err)
	assert.Equal(t, models.MigrationStatusFailed, task.Status)
	assert.Contains(t, task.Error, "agent client not configured")
}

func TestMigrationServiceMonitorWithoutAgentClientReturnsError(t *testing.T) {
	service, _, taskID := newMigrationServiceWithoutAgent(t)

	result, err := service.MonitorMigrationProgress(context.Background(), taskID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "agent client not configured")
}

func TestMigrationServiceMonitorFailedAgentStatusFailsTask(t *testing.T) {
	service, migrationRepo, taskID, cleanup := newMigrationMonitorTestService(t, "failed", 67, "replication stopped")
	defer cleanup()

	result, err := service.MonitorMigrationProgress(context.Background(), taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, models.MigrationStatusFailed, result.Status)
	assert.Equal(t, 67, result.Progress)
	task, err := migrationRepo.GetByID(context.Background(), taskID)
	require.NoError(t, err)
	assert.Equal(t, models.MigrationStatusFailed, task.Status)
	assert.Equal(t, 67, task.Progress)
	assert.Contains(t, task.Error, "replication stopped")
}

func TestMigrationServiceMonitorCompletedAgentStatusCompletesTask(t *testing.T) {
	service, migrationRepo, taskID, cleanup := newMigrationMonitorTestService(t, "completed", 100, "migration complete")
	defer cleanup()

	result, err := service.MonitorMigrationProgress(context.Background(), taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, models.MigrationStatusCompleted, result.Status)
	assert.Equal(t, 100, result.Progress)
	task, err := migrationRepo.GetByID(context.Background(), taskID)
	require.NoError(t, err)
	assert.Equal(t, models.MigrationStatusCompleted, task.Status)
	assert.Equal(t, 100, task.Progress)
}

func TestMigrationServiceVerifyWithoutAgentClientFailsTask(t *testing.T) {
	service, migrationRepo, taskID := newMigrationServiceWithoutAgent(t)

	result, err := service.VerifyMigration(context.Background(), taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0], "agent client not configured")
	task, err := migrationRepo.GetByID(context.Background(), taskID)
	require.NoError(t, err)
	assert.Equal(t, models.MigrationStatusFailed, task.Status)
	assert.Contains(t, task.Error, "agent client not configured")
}

func TestMigrationServiceSwitchWithoutAgentClientFailsTask(t *testing.T) {
	service, migrationRepo, taskID := newMigrationServiceWithoutAgent(t)

	result, err := service.ExecuteSwitch(context.Background(), taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	task, err := migrationRepo.GetByID(context.Background(), taskID)
	require.NoError(t, err)
	assert.Equal(t, models.MigrationStatusFailed, task.Status)
	assert.Contains(t, task.Error, "agent client not configured")
}

func TestMigrationService_SwitchFailureWritesAuditLog(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "auditor-004")
	db := newTestDB()
	migrationRepo := repositories.NewMigrationRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	service := NewMigrationService(migrationRepo, instanceRepo, repositories.NewHostRepository(db), newTestAgentClient(), NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db)))

	taskID, err := service.CreateTask(ctx, CreateMigrationTaskRequest{
		Name:             "switch failure audit migration",
		SourceInstanceID: "source-switch",
		TargetInstanceID: "target-missing-endpoint",
		Strategy:         models.MigrationStrategyReplication,
	})
	require.NoError(t, err)
	require.NoError(t, instanceRepo.Create(ctx, &models.Instance{ID: "target-missing-endpoint"}))

	result, err := service.ExecuteSwitch(ctx, taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	logs, err := auditRepo.ListByResource(context.Background(), "migration_task", taskID, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 2)
	assert.Equal(t, "switch_migration", logs[0].Operation)
	assert.Equal(t, "failed", logs[0].Result)
	assert.Contains(t, logs[0].ErrorMsg, "cannot resolve agent endpoint")
}

func TestMigrationService_ExecuteRunningAgentStatusStaysMigrating(t *testing.T) {
	service, migrationRepo, taskID, cleanup := newMigrationExecutionTestService(t, "running", 25)
	defer cleanup()

	result, err := service.ExecutePhysicalMigration(context.Background(), taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, models.MigrationStatusMigrating, result.Status)
	assert.Equal(t, 25, result.Progress)
	task, err := migrationRepo.GetByID(context.Background(), taskID)
	require.NoError(t, err)
	assert.Equal(t, models.MigrationStatusMigrating, task.Status)
	assert.Equal(t, 25, task.Progress)
}

func TestMigrationService_ExecuteCompletedAgentStatusCompletes(t *testing.T) {
	service, migrationRepo, taskID, cleanup := newMigrationExecutionTestService(t, "completed", 100)
	defer cleanup()

	result, err := service.ExecutePhysicalMigration(context.Background(), taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, models.MigrationStatusCompleted, result.Status)
	assert.Equal(t, 100, result.Progress)
	task, err := migrationRepo.GetByID(context.Background(), taskID)
	require.NoError(t, err)
	assert.Equal(t, models.MigrationStatusCompleted, task.Status)
}

func TestMigrationService_SwitchCompletedMarksApplicationUpdated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/migration-switch", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"task_id":  "switch-task",
				"status":   "completed",
				"progress": 100,
				"message":  "switch completed",
			},
		})
	}))
	defer server.Close()

	service, _, taskID := newMigrationServiceWithAgent(t, server.URL)

	result, err := service.ExecuteSwitch(context.Background(), taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "completed", result.Status)
	assert.True(t, result.ApplicationUpdated)
}

func TestMigrationService_VerifyAgentFailedStatusFailsTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/migration-verify", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"task_id":  "verify-task",
				"status":   "failed",
				"progress": 100,
				"message":  "checksum mismatch",
			},
		})
	}))
	defer server.Close()

	service, migrationRepo, taskID := newMigrationServiceWithAgent(t, server.URL)

	result, err := service.VerifyMigration(context.Background(), taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "checksum mismatch", result.Errors[0])
	task, err := migrationRepo.GetByID(context.Background(), taskID)
	require.NoError(t, err)
	assert.Equal(t, models.MigrationStatusFailed, task.Status)
	assert.Contains(t, task.Error, "checksum mismatch")
}

func TestMigrationService_SwitchNonCompletedStatusReturnsFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/migration-switch", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"task_id":  "switch-task",
				"status":   "running",
				"progress": 80,
				"message":  "switch not complete",
			},
		})
	}))
	defer server.Close()

	service, migrationRepo, taskID := newMigrationServiceWithAgent(t, server.URL)

	result, err := service.ExecuteSwitch(context.Background(), taskID)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.False(t, result.ApplicationUpdated)
	task, err := migrationRepo.GetByID(context.Background(), taskID)
	require.NoError(t, err)
	assert.Equal(t, models.MigrationStatusFailed, task.Status)
	assert.Contains(t, task.Error, "switch not complete")
}

func newMigrationExecutionTestService(t *testing.T, agentStatus string, progress int) (*MigrationService, *repositories.MigrationRepository, string, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/migration", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"task_id":  "migration-task",
				"status":   agentStatus,
				"progress": progress,
				"message":  "migration " + agentStatus,
			},
		})
	}))
	service, migrationRepo, taskID := newMigrationServiceWithAgent(t, server.URL)
	return service, migrationRepo, taskID, server.Close
}

func newMigrationMonitorTestService(t *testing.T, agentStatus string, progress int, message string) (*MigrationService, *repositories.MigrationRepository, string, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/migration", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data": map[string]interface{}{
				"task_id":  "migration-monitor-task",
				"status":   agentStatus,
				"progress": progress,
				"message":  message,
			},
		})
	}))
	service, migrationRepo, taskID := newMigrationServiceWithAgent(t, server.URL)
	require.NoError(t, migrationRepo.UpdateStatus(context.Background(), taskID, models.MigrationStatusMigrating, 25))
	return service, migrationRepo, taskID, server.Close
}

func newMigrationServiceWithAgent(t *testing.T, serverURL string) (*MigrationService, *repositories.MigrationRepository, string) {
	t.Helper()
	ctx := context.Background()
	u, err := url.Parse(serverURL)
	require.NoError(t, err)
	agentPort, err := strconv.Atoi(u.Port())
	require.NoError(t, err)

	db := newTestDB()
	migrationRepo := repositories.NewMigrationRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	hostRepo := repositories.NewHostRepository(db)
	hostID := "migration-agent-host"
	require.NoError(t, hostRepo.Create(ctx, &models.Host{
		ID:        hostID,
		Name:      "migration-agent-host",
		Address:   u.Hostname(),
		AgentPort: agentPort,
		SSHPort:   22,
		SSHUser:   "root",
	}))
	require.NoError(t, instanceRepo.Create(ctx, &models.Instance{
		ID:     "migration-source",
		Name:   "migration-source",
		HostID: &hostID,
	}))
	require.NoError(t, instanceRepo.Create(ctx, &models.Instance{
		ID:     "migration-target",
		Name:   "migration-target",
		HostID: &hostID,
	}))
	service := NewMigrationService(migrationRepo, instanceRepo, hostRepo, NewAgentClient(""))
	taskID, err := service.CreateTask(ctx, CreateMigrationTaskRequest{
		Name:             "migration execution",
		SourceInstanceID: "migration-source",
		TargetInstanceID: "migration-target",
		Strategy:         models.MigrationStrategyPhysical,
	})
	require.NoError(t, err)
	return service, migrationRepo, taskID
}

func newMigrationServiceWithoutAgent(t *testing.T) (*MigrationService, *repositories.MigrationRepository, string) {
	t.Helper()
	ctx := context.Background()
	db := newTestDB()
	migrationRepo := repositories.NewMigrationRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	hostRepo := repositories.NewHostRepository(db)
	hostID := "migration-no-agent-host"
	require.NoError(t, hostRepo.Create(ctx, &models.Host{
		ID:        hostID,
		Name:      "migration-no-agent-host",
		Address:   "127.0.0.1",
		AgentPort: 9090,
		SSHPort:   22,
		SSHUser:   "root",
	}))
	require.NoError(t, instanceRepo.Create(ctx, &models.Instance{
		ID:     "migration-no-agent-source",
		Name:   "migration-no-agent-source",
		HostID: &hostID,
	}))
	require.NoError(t, instanceRepo.Create(ctx, &models.Instance{
		ID:     "migration-no-agent-target",
		Name:   "migration-no-agent-target",
		HostID: &hostID,
	}))
	service := NewMigrationService(migrationRepo, instanceRepo, hostRepo, nil)
	taskID, err := service.CreateTask(ctx, CreateMigrationTaskRequest{
		Name:             "migration without agent",
		SourceInstanceID: "migration-no-agent-source",
		TargetInstanceID: "migration-no-agent-target",
		Strategy:         models.MigrationStrategyPhysical,
	})
	require.NoError(t, err)
	return service, migrationRepo, taskID
}
