package services

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestBackupService 创建一个共享 db 的 BackupService — hostRepo / instRepo /
// backupRepo 都连同一 Database, 这样 backup_policies 外键能正确指向 instances 行.
func newTestBackupService() *BackupService {
	service, _ := newTestBackupServiceWithAudit()
	return service
}

func newTestBackupServiceWithAudit() (*BackupService, *repositories.AuditLogRepository) {
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	backupRepo := repositories.NewBackupRepository(db)
	auditRepo := repositories.NewAuditLogRepository(db)
	auditSvc := NewAuditService(auditRepo, repositories.NewApprovalRequestRepository(db))
	hostID := "host-001"
	_ = hostRepo.Create(context.Background(), &models.Host{ID: hostID, Name: "test-host", Address: "192.168.1.100"})
	_ = instRepo.Create(context.Background(), &models.Instance{ID: "instance-001", Name: "instance-001", HostID: &hostID})
	password, _ := utils.Encrypt("rootpass", "test-encryption-key")
	_ = instRepo.CreateConnection(context.Background(), &models.InstanceConnection{
		InstanceID:        "instance-001",
		Host:              "192.168.1.100",
		Port:              3306,
		Username:          "root",
		PasswordEncrypted: password,
	})
	return NewBackupService(hostRepo, instRepo, backupRepo, newTestAgentClient(), "test-encryption-key", auditSvc), auditRepo
}

func TestNewBackupService(t *testing.T) {
	service := newTestBackupService()
	assert.NotNil(t, service)
}

func TestCreatePolicy(t *testing.T) {
	service := newTestBackupService()

	req := CreateBackupPolicyRequest{
		InstanceID:    "instance-001",
		BackupType:    "full",
		Schedule:      "0 2 * * *",
		RetentionDays: 7,
		StorageType:   "local",
		StoragePath:   "/backup",
		Enabled:       true,
	}

	ctx := context.Background()
	policyID, err := service.CreatePolicy(ctx, req)

	assert.NoError(t, err)
	assert.NotEmpty(t, policyID)
}

func TestCreatePolicyWritesAuditLog(t *testing.T) {
	service, auditRepo := newTestBackupServiceWithAudit()
	ctx := context.WithValue(context.Background(), "user_id", "backup-user")

	policyID, err := service.CreatePolicy(ctx, CreateBackupPolicyRequest{
		InstanceID:    "instance-001",
		BackupType:    "full",
		Schedule:      "0 2 * * *",
		RetentionDays: 7,
		StorageType:   "local",
		StoragePath:   "/backup",
		Enabled:       true,
	})

	assert.NoError(t, err)
	logs, err := auditRepo.ListByResource(context.Background(), "backup_policy", policyID, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "backup-user", logs[0].UserID)
	assert.Equal(t, "create_backup_policy", logs[0].Operation)
	assert.Equal(t, "success", logs[0].Result)
	assert.Contains(t, logs[0].Details, "backup_type=full")
}

func TestExecuteBackup(t *testing.T) {
	service := newTestBackupService()

	req := ExecuteBackupRequest{
		InstanceID: "instance-001",
		BackupType: "full",
	}

	// 测试用 2s 短 timeout — 真实 agent 不可达时, 立即失败而不是等 TCP 默认 timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := service.ExecuteBackup(ctx, req)

	// 没有真实 agent, 整体流程仍返回 (status=failed), 而非 err.
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.TaskID)
	assert.NotZero(t, result.StartedAt)
	assert.Equal(t, "failed", result.Status)

	backups, err := service.ListBackups(context.Background(), "instance-001")
	assert.NoError(t, err)
	assert.Len(t, backups, 1)
	assert.Equal(t, "failed", backups[0].Status)
	assert.Equal(t, "full", backups[0].BackupType)
	assert.Equal(t, result.TaskID, backups[0].TaskID)
}

func TestExecuteIncrementalBackupWithoutFullBaseCreatesFailedRecord(t *testing.T) {
	service := newTestBackupService()

	result, err := service.ExecuteBackup(context.Background(), ExecuteBackupRequest{
		InstanceID: "instance-001",
		BackupType: "incremental",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.NotEmpty(t, result.TaskID)
	assert.Contains(t, result.Message, "completed full backup")

	backups, err := service.ListBackups(context.Background(), "instance-001")
	assert.NoError(t, err)
	assert.Len(t, backups, 1)
	assert.Equal(t, "failed", backups[0].Status)
	assert.Equal(t, "incremental", backups[0].BackupType)
	assert.Equal(t, result.TaskID, backups[0].TaskID)
	assert.Contains(t, backups[0].Message, "completed full backup")
}

func TestExecuteBackupWritesAuditLogForFailedIncremental(t *testing.T) {
	service, auditRepo := newTestBackupServiceWithAudit()
	ctx := context.WithValue(context.Background(), "user_id", "backup-user")

	result, err := service.ExecuteBackup(ctx, ExecuteBackupRequest{
		InstanceID: "instance-001",
		BackupType: "incremental",
	})

	assert.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
	logs, err := auditRepo.ListByResource(context.Background(), "backup_record", result.TaskID, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "backup-user", logs[0].UserID)
	assert.Equal(t, "execute_backup", logs[0].Operation)
	assert.Equal(t, "failed", logs[0].Result)
	assert.Contains(t, logs[0].Details, "backup_type=incremental")
	assert.Contains(t, logs[0].ErrorMsg, "completed full backup")
}

func TestExecuteBackup_AgentRunningStatusCreatesRunningRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/agent/tasks/backup", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"success","data":{"task_id":"agent-backup-001","status":"running","progress":10,"message":"backup accepted","data":{"backup_path":"/backup/mysql/full.xbstream"}}}`))
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	assert.NoError(t, err)
	hostAddr, portText, err := net.SplitHostPort(u.Host)
	assert.NoError(t, err)
	agentPort, err := strconv.Atoi(portText)
	assert.NoError(t, err)

	db := newTestDB()
	defer db.Close()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	backupRepo := repositories.NewBackupRepository(db)
	hostID := "host-running"
	assert.NoError(t, hostRepo.Create(context.Background(), &models.Host{ID: hostID, Name: "backup-agent", Address: hostAddr, AgentPort: agentPort}))
	assert.NoError(t, instRepo.Create(context.Background(), &models.Instance{ID: "instance-running", Name: "instance-running", HostID: &hostID}))
	password, _ := utils.Encrypt("rootpass", "test-encryption-key")
	assert.NoError(t, instRepo.CreateConnection(context.Background(), &models.InstanceConnection{
		InstanceID:        "instance-running",
		Host:              "127.0.0.1",
		Port:              3306,
		Username:          "root",
		PasswordEncrypted: password,
	}))
	service := NewBackupService(hostRepo, instRepo, backupRepo, NewAgentClient(""), "test-encryption-key")

	result, err := service.ExecuteBackup(context.Background(), ExecuteBackupRequest{
		InstanceID: "instance-running",
		BackupType: "full",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "running", result.Status)
	assert.Equal(t, "/backup/mysql/full.xbstream", result.FilePath)
	backups, err := service.ListBackups(context.Background(), "instance-running")
	assert.NoError(t, err)
	assert.Len(t, backups, 1)
	assert.Equal(t, "running", backups[0].Status)
	assert.Equal(t, result.TaskID, backups[0].TaskID)
	assert.Equal(t, "/backup/mysql/full.xbstream", backups[0].FilePath)
	assert.True(t, result.CompletedAt.IsZero())
	assert.True(t, backups[0].CompletedAt.IsZero())
}

func TestScanBackupsRegistersDiscoveredRecordsAndAvoidsDuplicates(t *testing.T) {
	scanPayload := `{"code":200,"message":"success","data":{"task_id":"scan-001","status":"completed","progress":100,"message":"scan done","data":{"backups":[{"file_name":"full-001","file_path":"/backup/mysql/full-001","size_bytes":2048,"backup_type":"full","detected_at":"2026-06-09T10:00:00Z","mtime":"2026-06-09T10:00:00Z"},{"file_name":"full-001","file_path":"/backup/mysql/full-001","size_bytes":2048,"backup_type":"full","detected_at":"2026-06-09T10:00:00Z","mtime":"2026-06-09T10:00:00Z"}]}}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/agent/tasks/backup-scan", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(scanPayload))
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	assert.NoError(t, err)
	hostAddr, portText, err := net.SplitHostPort(u.Host)
	assert.NoError(t, err)
	agentPort, err := strconv.Atoi(portText)
	assert.NoError(t, err)

	db := newTestDB()
	defer db.Close()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	backupRepo := repositories.NewBackupRepository(db)
	hostID := "host-scan"
	assert.NoError(t, hostRepo.Create(context.Background(), &models.Host{ID: hostID, Name: "scan-agent", Address: hostAddr, AgentPort: agentPort}))
	assert.NoError(t, instRepo.Create(context.Background(), &models.Instance{ID: "instance-scan", Name: "instance-scan", HostID: &hostID}))
	password, _ := utils.Encrypt("rootpass", "test-encryption-key")
	assert.NoError(t, instRepo.CreateConnection(context.Background(), &models.InstanceConnection{
		InstanceID:        "instance-scan",
		Host:              "127.0.0.1",
		Port:              3306,
		Username:          "root",
		PasswordEncrypted: password,
	}))
	service := NewBackupService(hostRepo, instRepo, backupRepo, NewAgentClient(""), "test-encryption-key")

	result, err := service.ScanBackups(context.Background(), "instance-scan")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Backups, 1)
	assert.True(t, result.Backups[0].AlreadyManaged)
	assert.NotEmpty(t, result.Backups[0].ManagedBackupID)
	records, err := service.ListBackups(context.Background(), "instance-scan")
	assert.NoError(t, err)
	assert.Len(t, records, 1)
	assert.Equal(t, "completed", records[0].Status)
	assert.Equal(t, "full", records[0].BackupType)
	assert.Equal(t, "/backup/mysql/full-001", records[0].FilePath)

	second, err := service.ScanBackups(context.Background(), "instance-scan")

	assert.NoError(t, err)
	assert.Len(t, second.Backups, 1)
	assert.True(t, second.Backups[0].AlreadyManaged)
	records, err = service.ListBackups(context.Background(), "instance-scan")
	assert.NoError(t, err)
	assert.Len(t, records, 1)
}

func TestRestoreBackupDispatchesAgentAndWritesRestoreRecord(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/agent/tasks/restore", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"success","data":{"task_id":"agent-restore-001","status":"completed","progress":100,"message":"restore done"}}`))
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	require.NoError(t, err)
	hostAddr, portText, err := net.SplitHostPort(u.Host)
	require.NoError(t, err)
	agentPort, err := strconv.Atoi(portText)
	require.NoError(t, err)

	db := newTestDB()
	defer db.Close()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	backupRepo := repositories.NewBackupRepository(db)
	hostID := "restore-host"
	require.NoError(t, hostRepo.Create(context.Background(), &models.Host{ID: hostID, Name: "restore-host", Address: hostAddr, AgentPort: agentPort}))
	require.NoError(t, instRepo.Create(context.Background(), &models.Instance{ID: "restore-instance", Name: "restore-instance", HostID: &hostID}))
	password, _ := utils.Encrypt("rootpass", "test-encryption-key")
	require.NoError(t, instRepo.CreateConnection(context.Background(), &models.InstanceConnection{
		InstanceID:        "restore-instance",
		Host:              "127.0.0.1",
		Port:              3306,
		Username:          "root",
		PasswordEncrypted: password,
	}))
	require.NoError(t, backupRepo.CreateRecord(context.Background(), &models.BackupRecord{
		InstanceID:  "restore-instance",
		BackupType:  "full",
		TaskID:      "backup-completed",
		StartedAt:   time.Now().Add(-time.Hour),
		CompletedAt: time.Now().Add(-30 * time.Minute),
		Status:      "completed",
		FilePath:    "/backup/mysql/full-001",
		CreatedAt:   time.Now(),
	}))
	records, err := backupRepo.ListRecords(context.Background(), "restore-instance", 10, 0)
	require.NoError(t, err)
	require.Len(t, records, 1)

	service := NewBackupService(hostRepo, instRepo, backupRepo, NewAgentClient(""), "test-encryption-key")
	result, err := service.RestoreBackup(context.Background(), RestoreBackupRequest{
		BackupID:         records[0].ID,
		TargetInstanceID: "restore-instance",
		TargetType:       "in-place",
		ConfirmOverwrite: true,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, records[0].ID, result.BackupID)
	assert.Equal(t, "restore-instance", result.TargetInstanceID)
	assert.NotZero(t, result.CompletedAt)
}

func TestDeleteBackupRecordRemovesRecord(t *testing.T) {
	service := newTestBackupService()
	ctx := context.Background()
	result, err := service.ExecuteBackup(ctx, ExecuteBackupRequest{InstanceID: "instance-001", BackupType: "full"})
	require.NoError(t, err)
	require.NotNil(t, result)
	records, err := service.ListBackups(ctx, "instance-001")
	require.NoError(t, err)
	require.Len(t, records, 1)

	err = service.DeleteBackupRecord(ctx, records[0].ID)

	require.NoError(t, err)
	records, err = service.ListBackups(ctx, "instance-001")
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestListBackups(t *testing.T) {
	service := newTestBackupService()

	ctx := context.Background()
	backups, err := service.ListBackups(ctx, "instance-001")

	assert.NoError(t, err)
	assert.Empty(t, backups)
}

func TestCreateBackupPolicyRequest_Fields(t *testing.T) {
	req := CreateBackupPolicyRequest{
		InstanceID:    "instance-001",
		BackupType:    "incremental",
		Schedule:      "0 3 * * *",
		RetentionDays: 14,
		StorageType:   "s3",
		StoragePath:   "s3://backup-bucket",
		Enabled:       true,
	}

	assert.Equal(t, "instance-001", req.InstanceID)
	assert.Equal(t, "incremental", req.BackupType)
	assert.Equal(t, "0 3 * * *", req.Schedule)
	assert.Equal(t, 14, req.RetentionDays)
	assert.Equal(t, "s3", req.StorageType)
	assert.True(t, req.Enabled)
}

func TestExecuteBackupRequest_Fields(t *testing.T) {
	req := ExecuteBackupRequest{
		InstanceID: "instance-001",
		BackupType: "full",
	}

	assert.Equal(t, "instance-001", req.InstanceID)
	assert.Equal(t, "full", req.BackupType)
}

func TestBackupTaskResult_Fields(t *testing.T) {
	result := BackupTaskResult{
		TaskID:      "backup-001",
		Status:      "completed",
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
		FilePath:    "/backup/mysql.tar.gz",
		FileSize:    1024000,
		Checksum:    "sha256:abc",
	}

	assert.Equal(t, "backup-001", result.TaskID)
	assert.Equal(t, "completed", result.Status)
	assert.NotEmpty(t, result.FilePath)
}

func TestCreatePolicy_DifferentTypes(t *testing.T) {
	service := newTestBackupService()
	ctx := context.Background()

	fullReq := CreateBackupPolicyRequest{InstanceID: "instance-001", BackupType: "full"}
	fullID, err := service.CreatePolicy(ctx, fullReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, fullID)

	incReq := CreateBackupPolicyRequest{InstanceID: "instance-001", BackupType: "incremental"}
	incID, err := service.CreatePolicy(ctx, incReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, incID)
}

func TestExecuteBackup_NoInstance(t *testing.T) {
	service := newTestBackupService()
	ctx := context.Background()

	result, err := service.ExecuteBackup(ctx, ExecuteBackupRequest{BackupType: "full"})
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestListBackups_MultipleInstances(t *testing.T) {
	service := newTestBackupService()
	ctx := context.Background()

	backups1, err := service.ListBackups(ctx, "instance-001")
	assert.NoError(t, err)
	assert.Empty(t, backups1)

	backups2, err := service.ListBackups(ctx, "instance-002")
	assert.NoError(t, err)
	assert.Empty(t, backups2)
}
