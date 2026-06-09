package services

import (
	"context"
	"testing"
	"time"

	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/repositories"
	"github.com/monkeycode/mysql-ops-platform/pkg/utils"
	"github.com/stretchr/testify/assert"
)

// newTestBackupService 创建一个共享 db 的 BackupService — hostRepo / instRepo /
// backupRepo 都连同一 Database, 这样 backup_policies 外键能正确指向 instances 行.
func newTestBackupService() *BackupService {
	db := newTestDB()
	hostRepo := repositories.NewHostRepository(db)
	instRepo := repositories.NewInstanceRepository(db)
	backupRepo := repositories.NewBackupRepository(db)
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
	return NewBackupService(hostRepo, instRepo, backupRepo, newTestAgentClient(), "test-encryption-key")
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
