package services

import (
	"context"
	"net"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminActionRefusedForNonMySQLFlavor(t *testing.T) {
	for _, flavor := range tieredFlavors {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			hostRepo := repositories.NewHostRepository(db)
			instanceRepo := repositories.NewInstanceRepository(db)
			taskRepo := repositories.NewTaskRepository(db)
			seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-admin-"+flavor, "", flavor)

			// nil agent client: a leaked dispatch would report a different error.
			service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
			result, err := service.AdminAction(ctx, "inst-admin-"+flavor, InstanceAdminRequest{
				Action:   "create_user",
				Username: "probe",
				Password: "Probe#12345",
			})

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), flavor)
			assert.Contains(t, err.Error(), string(CapInstanceAdmin))
		})
	}
}

func TestAdminActionAllowsMySQLFlavorPastCapabilityGate(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-admin-mysql", "", "mysql")

	service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
	result, err := service.AdminAction(ctx, "inst-admin-mysql", InstanceAdminRequest{
		Action:   "create_user",
		Username: "probe",
		Password: "Probe#12345",
	})

	// The gate must not be what stops a MySQL instance: it fails later on the
	// missing agent client, which proves the gate let it through.
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotContains(t, result.Message, string(CapInstanceAdmin))
}

func TestBatchUpdatePasswordSkipsNonMySQLInstances(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)

	seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-batch-kingbase", "", "kingbase")

	service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
	result, err := service.BatchUpdatePassword(ctx, BatchPasswordRequest{
		Host:        "10.0.0.90",
		Ports:       []int{3306},
		Username:    "root",
		NewPassword: "Probe#12345",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Message, "skipped")
}

func TestDeleteDeregistersTieredOnboardingInstanceWithoutBackup(t *testing.T) {
	// Tiered-onboarding engines were never provisioned by the platform, so there
	// is no xtrabackup image to take. Delete must de-register them rather than
	// being permanently blocked by the physical-backup gate.
	for _, flavor := range tieredFlavors {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			hostRepo := repositories.NewHostRepository(db)
			instanceRepo := repositories.NewInstanceRepository(db)
			taskRepo := repositories.NewTaskRepository(db)
			instanceID := "inst-delete-" + flavor
			seedInstanceWithFlavor(t, ctx, instanceRepo, instanceID, "", flavor)

			// nil backup service: reaching the backup path would fail outright.
			service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
			require.NoError(t, service.Delete(ctx, instanceID))

			_, err := instanceRepo.GetByID(ctx, instanceID)
			assert.Error(t, err, "instance record should be gone after de-registration")
		})
	}
}

func TestDeleteStillRequiresBackupForMySQLFlavor(t *testing.T) {
	// 可达的 MySQL 实例保留 backup-then-decommission 保证: 备份服务缺失时
	// 删除必须失败并保留实例。
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)

	// 本地监听模拟端口可达的 MySQL 实例
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	instanceID := "inst-delete-mysql"
	require.NoError(t, instanceRepo.Create(ctx, &models.Instance{ID: instanceID, Name: instanceID}))
	require.NoError(t, instanceRepo.CreateConnection(ctx, &models.InstanceConnection{
		InstanceID: instanceID,
		Host:       "127.0.0.1",
		Port:       port,
		Username:   "root",
	}))
	require.NoError(t, instanceRepo.CreateVersion(ctx, &models.InstanceVersion{
		InstanceID: instanceID,
		Flavor:     "mysql",
		Version:    "1.0.0",
	}))

	// nil backup service: reaching the backup path would fail outright.
	service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
	err = service.Delete(ctx, instanceID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup")

	surviving, getErr := instanceRepo.GetByID(ctx, instanceID)
	require.NoError(t, getErr)
	assert.Equal(t, instanceID, surviving.ID)
}

func TestDeleteDeregistersUnreachableMySQLInstance(t *testing.T) {
	// 端口拒绝连接的 unhealthy 实例无法备份也无法退役, 强制先备份会把实例
	// 永久锁死在清单里; 删除降级为仅注销 (deregister-only)。
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	instanceID := "inst-delete-mysql-unreachable"
	seedInstanceWithFlavor(t, ctx, instanceRepo, instanceID, "", "mysql")

	service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
	require.NoError(t, service.Delete(ctx, instanceID))

	_, err := instanceRepo.GetByID(ctx, instanceID)
	assert.Error(t, err, "unreachable instance should be deregistered")
}

var _ = models.Instance{}
