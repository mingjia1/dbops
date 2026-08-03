package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// driverlessFlavors are the engines with no usable Go driver. They are the
// strictest case for every gate.
var driverlessFlavors = []string{"gbase8s", "dm"}

// tieredFlavors covers engines limited to inventory and health checks.
var tieredFlavors = []string{"gbase8a", "dm", "shentong", "kingbase", "highgo"}

func TestRestoreBackupRefusedForNonMySQLFlavor(t *testing.T) {
	for _, flavor := range tieredFlavors {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			hostRepo := repositories.NewHostRepository(db)
			instanceRepo := repositories.NewInstanceRepository(db)
			backupRepo := repositories.NewBackupRepository(db)

			seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-restore-"+flavor, "cluster-restore-"+flavor, flavor)
			// CreateRecord assigns its own UUID, so read the ID back afterwards.
			record := &models.BackupRecord{
				InstanceID: "inst-restore-" + flavor,
				BackupType: "full",
				Status:     "completed",
				FilePath:   "/backup/mysql/full-1.xbstream",
			}
			require.NoError(t, backupRepo.CreateRecord(ctx, record))
			require.NotEmpty(t, record.ID)

			// nil agent client: if the gate leaks, the restore dispatch panics.
			service := NewBackupService(hostRepo, instanceRepo, backupRepo, nil, "test-encryption-key")
			result, err := service.RestoreBackup(ctx, RestoreBackupRequest{
				BackupID:         record.ID,
				TargetInstanceID: "inst-restore-" + flavor,
				ConfirmOverwrite: true,
			})

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), flavor)
			assert.Contains(t, err.Error(), string(CapPhysicalBackup))
		})
	}
}

func TestScaleOutRefusedForNonMySQLFlavor(t *testing.T) {
	for _, flavor := range tieredFlavors {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			instanceRepo := repositories.NewInstanceRepository(db)
			seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-scaleout-"+flavor, "cluster-scaleout-"+flavor, flavor)

			// nil orchestrator and nil plugin executor: any leaked call panics.
			service := NewScaleService(nil, instanceRepo, nil)
			result, err := service.ScaleOut(ctx, ScaleOutRequest{
				ClusterID: "cluster-scaleout-" + flavor,
				Flavor:    flavor,
				ArchType:  "ha",
				NewNodes:  []OrchestratorNode{{HostID: "host-1", Address: "10.0.0.91", MySQLPort: 3306}},
			})

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), flavor)
			assert.Contains(t, err.Error(), string(CapScale))
		})
	}
}

func TestScaleInRefusedForNonMySQLFlavorAndDoesNotDeleteInstance(t *testing.T) {
	for _, flavor := range tieredFlavors {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			instanceRepo := repositories.NewInstanceRepository(db)
			instanceID := "inst-scalein-" + flavor
			seedInstanceWithFlavor(t, ctx, instanceRepo, instanceID, "cluster-scalein-"+flavor, flavor)

			service := NewScaleService(nil, instanceRepo, nil)
			result, err := service.ScaleIn(ctx, ScaleInRequest{
				ClusterID:    "cluster-scalein-" + flavor,
				ArchType:     "ha",
				RemoveNodeID: instanceID,
			})

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), string(CapScale))

			// The instance record must survive: scale-in deletes it on success.
			surviving, getErr := instanceRepo.GetByID(ctx, instanceID)
			require.NoError(t, getErr)
			assert.Equal(t, instanceID, surviving.ID)
		})
	}
}

func TestScaleServiceRebuildNodeRefusedForNonMySQLFlavor(t *testing.T) {
	for _, flavor := range tieredFlavors {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			instanceRepo := repositories.NewInstanceRepository(db)
			seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-rebuild-"+flavor, "cluster-rebuild-"+flavor, flavor)

			service := NewScaleService(nil, instanceRepo, nil)
			result, err := service.RebuildNode(ctx, RebuildRequest{
				ClusterID:  "cluster-rebuild-" + flavor,
				InstanceID: "inst-rebuild-" + flavor,
				Flavor:     flavor,
				ArchType:   "ha",
				Node:       OrchestratorNode{HostID: "host-1", Address: "10.0.0.92", MySQLPort: 3306},
			})

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), string(CapNodeRebuild))
		})
	}
}

func TestRebuildServiceRefusesBeforeTeardown(t *testing.T) {
	// RebuildService calls RunTeardown before RunExecute. A nil plugin executor
	// proves the gate runs first: a leak would panic instead of returning.
	for _, flavor := range driverlessFlavors {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			instanceRepo := repositories.NewInstanceRepository(db)
			seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-rs-"+flavor, "cluster-rs-"+flavor, flavor)

			service := NewRebuildService(nil, nil, instanceRepo)
			result, err := service.RebuildNode(ctx, RebuildServiceRequest{
				ClusterID:  "cluster-rs-" + flavor,
				InstanceID: "inst-rs-" + flavor,
				Flavor:     flavor,
				ArchType:   "ha",
				Node:       OrchestratorNode{HostID: "host-1", Address: "10.0.0.93", MySQLPort: 3306},
			})

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), string(CapNodeRebuild))
		})
	}
}

func TestRebuildClusterRefusedBeforeDestroy(t *testing.T) {
	// RebuildCluster destroys before redeploying. A nil orchestrator and plugin
	// executor prove nothing ran.
	ctx := context.Background()
	db := newTestDB(t)
	instanceRepo := repositories.NewInstanceRepository(db)
	seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-rebuild-cluster", "cluster-rebuild-full", "gbase8s")

	service := NewClusterLifecycleService(nil, nil, nil, instanceRepo)
	result, err := service.RebuildCluster(ctx, RebuildClusterRequest{
		OriginalReq: DeployOrchestratorRequest{
			ClusterID: "cluster-rebuild-full",
			Flavor:    "gbase8s",
			ArchType:  "ha",
			Nodes:     []OrchestratorNode{{HostID: "host-1", Address: "10.0.0.94", MySQLPort: 3306}},
		},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "gbase8s")
	assert.Contains(t, err.Error(), string(CapNodeRebuild))
}

func TestInstanceDeployRefusedForNonMySQLFlavor(t *testing.T) {
	for _, flavor := range tieredFlavors {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			hostRepo := repositories.NewHostRepository(db)
			instanceRepo := repositories.NewInstanceRepository(db)
			taskRepo := repositories.NewTaskRepository(db)
			seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-deploy-"+flavor, "", flavor)

			service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
			result, err := service.Deploy(ctx, "inst-deploy-"+flavor)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), flavor)
			assert.Contains(t, err.Error(), string(CapInstanceDeploy))
		})
	}
}

func TestGBase8sEnablesOnlyVerifiedSingleNodeDeployAndConfigurationCapabilities(t *testing.T) {
	for _, capability := range []Capability{CapInstanceDeploy, CapParameterTemplate, CapPhysicalBackup, CapLogicalUpgrade} {
		if !HasCapability("gbase8s", capability) {
			t.Fatalf("gbase8s capability %s is disabled", capability)
		}
	}
	for _, capability := range []Capability{CapInPlaceUpgrade, CapInstanceAdmin, CapReplication, CapFailover, CapScale, CapNodeRebuild, CapClusterDeploy} {
		if HasCapability("gbase8s", capability) {
			t.Fatalf("gbase8s capability %s is enabled without verified coverage", capability)
		}
	}
}

func TestReplicationStatusRefusedForNonMySQLFlavor(t *testing.T) {
	for _, flavor := range tieredFlavors {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			hostRepo := repositories.NewHostRepository(db)
			instanceRepo := repositories.NewInstanceRepository(db)
			taskRepo := repositories.NewTaskRepository(db)
			seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-repl-"+flavor, "", flavor)

			service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
			result, err := service.ReplicationStatus(ctx, "inst-repl-"+flavor)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), string(CapReplication))
		})
	}
}

func TestInstanceDeployAllowsMySQLFlavorPastCapabilityGate(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	hostRepo := repositories.NewHostRepository(db)
	instanceRepo := repositories.NewInstanceRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-deploy-mysql", "", "mysql")

	service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
	_, err := service.Deploy(ctx, "inst-deploy-mysql")

	// It must fail for a downstream reason, not because of the capability gate.
	if err != nil {
		assert.NotContains(t, err.Error(), string(CapInstanceDeploy))
	}
}

func TestMigrationExecutionRefusedForNonMySQLFlavor(t *testing.T) {
	for _, flavor := range tieredFlavors {
		t.Run(flavor, func(t *testing.T) {
			ctx := context.Background()
			db := newTestDB(t)
			instanceRepo := repositories.NewInstanceRepository(db)
			migrationRepo := repositories.NewMigrationRepository(db)
			seedInstanceWithFlavor(t, ctx, instanceRepo, "migration-source-"+flavor, "migration-cluster-"+flavor, flavor)
			targetID := "migration-target-" + flavor
			require.NoError(t, instanceRepo.Create(ctx, &models.Instance{ID: targetID, Name: targetID, ClusterID: "migration-cluster-" + flavor}))
			require.NoError(t, instanceRepo.CreateVersion(ctx, &models.InstanceVersion{InstanceID: targetID, Flavor: flavor, Version: "1.0.0"}))
			service := NewMigrationService(migrationRepo, instanceRepo, nil, nil)
			taskID, err := service.CreateTask(ctx, CreateMigrationTaskRequest{
				Name:             "blocked migration",
				SourceInstanceID: "migration-source-" + flavor,
				TargetInstanceID: "migration-target-" + flavor,
				Strategy:         models.MigrationStrategyPhysical,
			})
			require.NoError(t, err)

			result, err := service.ExecutePhysicalMigration(ctx, taskID)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), flavor)
			assert.Contains(t, err.Error(), string(CapPhysicalBackup))
		})
	}
}

func TestDeployOrchestratorRefusesNonMySQLFlavor(t *testing.T) {
	for _, flavor := range tieredFlavors {
		t.Run(flavor, func(t *testing.T) {
			orchestrator := NewDeployOrchestrator(nil, nil, nil)

			result, err := orchestrator.Run(context.Background(), DeployOrchestratorRequest{
				Flavor: flavor,
				Nodes:  []OrchestratorNode{{Address: "10.0.0.96", MySQLPort: 3306}},
			})

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), flavor)
			assert.Contains(t, err.Error(), string(CapClusterDeploy))
		})
	}
}

func TestDestroyClusterRefusesManagedNonMySQLFlavor(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	instanceRepo := repositories.NewInstanceRepository(db)
	seedInstanceWithFlavor(t, ctx, instanceRepo, "destroy-gbase8s", "destroy-gbase8s-cluster", "gbase8s")
	service := NewClusterLifecycleService(nil, nil, nil, instanceRepo)

	result, err := service.DestroyCluster(ctx, DestroyRequest{
		ClusterID: "destroy-gbase8s-cluster",
		Flavor:    "mysql",
		Nodes:     []OrchestratorNode{{Address: "10.0.0.97", MySQLPort: 3306}},
	})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "gbase8s")
	assert.Contains(t, err.Error(), string(CapClusterDeploy))
}

func TestLegacyEmptyFlavorKeepsEveryGatedOperation(t *testing.T) {
	// Instances registered before flavor persistence carry an empty flavor. No
	// gate may reject them, otherwise upgrading the platform breaks live clusters.
	ctx := context.Background()
	db := newTestDB(t)
	instanceRepo := repositories.NewInstanceRepository(db)

	inst := &models.Instance{ID: "inst-legacy", Name: "inst-legacy", ClusterID: "cluster-legacy"}
	require.NoError(t, instanceRepo.Create(ctx, inst))
	require.NoError(t, instanceRepo.CreateConnection(ctx, &models.InstanceConnection{
		InstanceID: "inst-legacy",
		Host:       "10.0.0.95",
		Port:       3306,
		Username:   "root",
	}))

	assert.Equal(t, "", resolveInstanceFlavor(ctx, instanceRepo, "inst-legacy"))
	assert.Equal(t, "", resolveClusterFlavor(ctx, instanceRepo, "cluster-legacy"))
	for _, capability := range allCapabilities() {
		require.NoError(t, RequireCapability(resolveInstanceFlavor(ctx, instanceRepo, "inst-legacy"), capability),
			"legacy instance must retain %s", capability)
	}
}

func TestResolveClusterFlavorHydratesVersionRecord(t *testing.T) {
	// ListByClusterID does not populate the version record, so the resolver has to
	// fall back to GetByID. Without that, every cluster-level gate reads "unknown".
	ctx := context.Background()
	db := newTestDB(t)
	instanceRepo := repositories.NewInstanceRepository(db)
	seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-resolve", "cluster-resolve", "gbase8a")

	assert.Equal(t, "gbase8a", resolveClusterFlavor(ctx, instanceRepo, "cluster-resolve"))
	assert.Equal(t, "gbase8a", resolveInstanceFlavor(ctx, instanceRepo, "inst-resolve"))
}

func TestResolveFlavorHandlesMissingInputs(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	instanceRepo := repositories.NewInstanceRepository(db)

	assert.Equal(t, "unknown", resolveClusterFlavor(ctx, nil, "cluster-x"))
	assert.Equal(t, "unknown", resolveInstanceFlavor(ctx, nil, "inst-x"))
	assert.Equal(t, "unknown", resolveClusterFlavor(ctx, instanceRepo, ""))
	assert.Equal(t, "unknown", resolveInstanceFlavor(ctx, instanceRepo, ""))
	assert.Equal(t, "unknown", resolveClusterFlavor(ctx, instanceRepo, "cluster-absent"))
	assert.Equal(t, "unknown", resolveInstanceFlavor(ctx, instanceRepo, "inst-absent"))
}

func TestInstanceHealthCheckRefusesNonMySQLProtocolFlavor(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	instanceRepo := repositories.NewInstanceRepository(db)
	hostRepo := repositories.NewHostRepository(db)
	taskRepo := repositories.NewTaskRepository(db)
	seedInstanceWithFlavor(t, ctx, instanceRepo, "inst-health-kingbase", "cluster-health-kingbase", "kingbase")

	service := NewInstanceService(instanceRepo, hostRepo, taskRepo, nil, nil, "test-encryption-key")
	result, err := service.HealthCheck(ctx, "inst-health-kingbase")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "统一健康检测接口")
}
