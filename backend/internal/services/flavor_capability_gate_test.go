package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedInstanceWithFlavor creates an instance with a connection and a persisted
// engine flavor so that capability gates have something to read.
func seedInstanceWithFlavor(t *testing.T, ctx context.Context, instanceRepo *repositories.InstanceRepository, instanceID, clusterID, flavor string) {
	t.Helper()
	inst := &models.Instance{
		ID:        instanceID,
		Name:      instanceID,
		ClusterID: clusterID,
	}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, instanceRepo.CreateConnection(ctx, &models.InstanceConnection{
		InstanceID: instanceID,
		Host:       "10.0.0.90",
		Port:       3306,
		Username:   "root",
	}))
	require.NoError(t, instanceRepo.CreateVersion(ctx, &models.InstanceVersion{
		InstanceID: instanceID,
		Flavor:     flavor,
		Version:    "1.0.0",
	}))
}

func TestExecuteAutoFailoverRefusedForNonMySQLFlavor(t *testing.T) {
	for _, flavor := range []string{"gbase8s", "gbase8a", "dm", "shentong", "kingbase"} {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			instanceRepo := repositories.NewInstanceRepository(db)
			seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-failover-"+flavor, "cluster-failover-"+flavor, flavor)

			service := NewFailoverService(db, "test-encryption-key")
			result, err := service.ExecuteAutoFailover(ctx, FailoverRequest{
				ClusterID: "cluster-failover-" + flavor,
			})

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), flavor)
			assert.Contains(t, err.Error(), string(CapFailover))
		})
	}
}

func TestPreflightFailoverRefusedForNonMySQLFlavor(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	instanceRepo := repositories.NewInstanceRepository(db)
	seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-preflight-dm", "cluster-preflight-dm", "dm")

	service := NewFailoverService(db, "test-encryption-key")
	result, err := service.PreflightFailover(ctx, FailoverPreflightRequest{
		ClusterID: "cluster-preflight-dm",
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "dm")
	assert.Contains(t, err.Error(), string(CapFailover))
}

func TestExecuteBackupRefusesPhysicalBackupForNonMySQLFlavor(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	backupRepo := repositories.NewBackupRepository(db)

	seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-backup-gbase8a", "cluster-backup", "gbase8a")

	// Deliberately pass a nil agent client: if the gate works, no agent call is
	// ever attempted, so a nil client cannot cause a panic.
	service := NewBackupService(hostRepo, instanceRepo, backupRepo, nil, "test-encryption-key")
	result, err := service.ExecuteBackup(ctx, ExecuteBackupRequest{
		InstanceID: "inst-backup-gbase8a",
		BackupType: "full",
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "gbase8a")
	assert.Contains(t, err.Error(), string(CapPhysicalBackup))
}

func TestExecuteBackupAllowsMySQLFlavorPastCapabilityGate(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	backupRepo := repositories.NewBackupRepository(db)

	seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-backup-mysql", "cluster-backup-mysql", "mysql")

	service := NewBackupService(hostRepo, instanceRepo, backupRepo, nil, "test-encryption-key")
	_, err := service.ExecuteBackup(ctx, ExecuteBackupRequest{
		InstanceID: "inst-backup-mysql",
		BackupType: "full",
	})

	// The capability gate must not be what stops a MySQL instance. It fails later
	// for a missing agent host, which proves the gate let it through.
	require.Error(t, err)
	assert.NotContains(t, err.Error(), string(CapPhysicalBackup))
	assert.Contains(t, err.Error(), "agent host")
}

func TestSwitchRoleWithinClusterRefusedForNonMySQLFlavor(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	deployRepo := repositories.NewClusterDeployRepository(db)
	historyRepo := repositories.NewRoleSwitchHistoryRepository(db)

	seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-roleswitch-dm", "cluster-roleswitch-dm", "dm")
	require.NoError(t, deployRepo.Create(ctx, &models.ClusterDeployment{
		ID:          "cluster-roleswitch-dm",
		ClusterID:   "cluster-roleswitch-dm",
		Name:        "cluster-roleswitch-dm",
		ClusterType: ClusterTypeHA,
		Status:      "success",
	}))

	service := NewSwitchService(hostRepo, instanceRepo, deployRepo, nil, historyRepo, nil)
	service.SetEncryptionKey("test-encryption-key")

	result, err := service.SwitchRoleWithinCluster(ctx, RoleSwitchRequest{
		ClusterID:  "cluster-roleswitch-dm",
		InstanceID: "inst-roleswitch-dm",
		TargetRole: "master",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "dm")
	assert.Contains(t, result.Message, string(CapReplication))
}
