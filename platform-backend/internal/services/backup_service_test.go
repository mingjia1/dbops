package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBackupService(t *testing.T) {
	tctx := context.Background()
	service := NewBackupService(newTestHostRepo(tctx), newTestInstanceRepo(tctx), newTestAgentClient())
	assert.NotNil(t, service)
}

func TestCreatePolicy(t *testing.T) {
	tctx := context.Background()
	service := NewBackupService(newTestHostRepo(tctx), newTestInstanceRepo(tctx), newTestAgentClient())

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
	assert.Contains(t, policyID, "policy-")
}

func TestExecuteBackup(t *testing.T) {
	tctx := context.Background()
	service := NewBackupService(newTestHostRepo(tctx), newTestInstanceRepo(tctx), newTestAgentClient())

	req := ExecuteBackupRequest{
		InstanceID: "instance-001",
		BackupType: "full",
	}

	ctx := context.Background()
	result, err := service.ExecuteBackup(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.TaskID, "backup-")
	assert.NotZero(t, result.StartedAt)
}

func TestListBackups(t *testing.T) {
	tctx := context.Background()
	service := NewBackupService(newTestHostRepo(tctx), newTestInstanceRepo(tctx), newTestAgentClient())

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
	tctx := context.Background()
	service := NewBackupService(newTestHostRepo(tctx), newTestInstanceRepo(tctx), newTestAgentClient())
	ctx := context.Background()

	fullReq := CreateBackupPolicyRequest{BackupType: "full"}
	fullID, err := service.CreatePolicy(ctx, fullReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, fullID)

	incReq := CreateBackupPolicyRequest{BackupType: "incremental"}
	incID, err := service.CreatePolicy(ctx, incReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, incID)
}

func TestExecuteBackup_NoInstance(t *testing.T) {
	tctx := context.Background()
	service := NewBackupService(newTestHostRepo(tctx), newTestInstanceRepo(tctx), newTestAgentClient())
	ctx := context.Background()

	result, err := service.ExecuteBackup(ctx, ExecuteBackupRequest{BackupType: "full"})
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestListBackups_MultipleInstances(t *testing.T) {
	tctx := context.Background()
	service := NewBackupService(newTestHostRepo(tctx), newTestInstanceRepo(tctx), newTestAgentClient())
	ctx := context.Background()

	backups1, err := service.ListBackups(ctx, "instance-001")
	assert.NoError(t, err)
	assert.Empty(t, backups1)

	backups2, err := service.ListBackups(ctx, "instance-002")
	assert.NoError(t, err)
	assert.Empty(t, backups2)
}
