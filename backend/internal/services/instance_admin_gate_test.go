package services

import (
	"context"
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
	// MySQL instances keep the backup-then-decommission guarantee.
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-delete-mysql", "", "mysql")

	service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
	err := service.Delete(ctx, "inst-delete-mysql")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "backup")

	surviving, getErr := instanceRepo.GetByID(ctx, "inst-delete-mysql")
	require.NoError(t, getErr)
	assert.Equal(t, "inst-delete-mysql", surviving.ID)
}

var _ = models.Instance{}
