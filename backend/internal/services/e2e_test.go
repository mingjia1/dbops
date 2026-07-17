package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
	"github.com/jackcode/mysql-ops-platform/internal/plugins/arch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndToEnd_MHA_Deploy_ScaleOut_Destroy(t *testing.T) {
	registry := plugins.NewRegistry()
	fakeAgent := func(_ context.Context, _ string, _ int, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"status": "ok"}, nil
	}
	_ = registry.Register(&stubKernelPlugin{name: "mysql-core"})
	_ = registry.Register(arch.NewReplicaAddonPlugin(fakeAgent))
	_ = registry.Register(arch.NewMHAAddonPlugin(fakeAgent))

	repo := newTestInstanceRepo(t, context.Background())
	vault := NewCredentialVault(newTestCredentialRepo(t), "e2e-key")
	orch := NewDeployOrchestrator(registry, vault, repo)
	pluginExec := plugins.NewExecutor(registry)

	ctx := context.Background()

	// Phase 1: Deploy MHA cluster
	deployResult, err := orch.Run(ctx, DeployOrchestratorRequest{
		ClusterID:   "e2e-mha-cluster",
		ClusterName: "e2e-mha",
		Flavor:      "mysql",
		ArchType:    "replica",
		EncKey:      "e2e-key",
		Nodes: []OrchestratorNode{
			{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, Role: "primary", ServerID: 1, DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
			{Address: "10.0.0.2", AgentPort: 9090, MySQLPort: 3306, Role: "replica", ServerID: 2, DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
			{Address: "10.0.0.3", AgentPort: 9090, MySQLPort: 3306, Role: "replica", ServerID: 3, DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", deployResult.Status)
	assert.Len(t, deployResult.InstanceIDs, 3)

	// Phase 2: Scale out (add 1 replica)
	scaleSvc := NewScaleService(orch, repo, pluginExec)
	scaleResult, err := scaleSvc.ScaleOut(ctx, ScaleOutRequest{
		ClusterID: "e2e-mha-cluster",
		Flavor:    "mysql",
		ArchType:  "replica",
		EncKey:    "e2e-key",
		NewNodes: []OrchestratorNode{
			{Address: "10.0.0.4", AgentPort: 9090, MySQLPort: 3306, Role: "replica", ServerID: 4, DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", scaleResult.Status)
	assert.Len(t, scaleResult.NewInstIDs, 1)

	// Phase 3: Destroy cluster
	lifecycleSvc := NewClusterLifecycleService(orch, pluginExec, vault, repo)
	destroyResult, err := lifecycleSvc.DestroyCluster(ctx, DestroyRequest{
		ClusterID: "e2e-mha-cluster",
		Flavor:    "mysql",
		ArchType:  "replica",
		EncKey:    "e2e-key",
		Nodes: []OrchestratorNode{
			{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306},
			{Address: "10.0.0.2", AgentPort: 9090, MySQLPort: 3306},
			{Address: "10.0.0.3", AgentPort: 9090, MySQLPort: 3306},
			{Address: "10.0.0.4", AgentPort: 9090, MySQLPort: 3306},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", destroyResult.Status)

	// Verify credentials cleaned up
	creds, _ := vault.ListClusterCredentials(ctx, "e2e-mha-cluster")
	assert.Len(t, creds, 0)
}

func TestEndToEnd_SingleNode_Deploy_Rebuild(t *testing.T) {
	registry := plugins.NewRegistry()
	_ = registry.Register(&stubKernelPlugin{name: "mysql-core"})

	repo := newTestInstanceRepo(t, context.Background())
	vault := NewCredentialVault(newTestCredentialRepo(t), "e2e-single-key")
	orch := NewDeployOrchestrator(registry, vault, repo)
	pluginExec := plugins.NewExecutor(registry)

	ctx := context.Background()

	// Deploy single node
	deployResult, err := orch.Run(ctx, DeployOrchestratorRequest{
		ClusterID:   "e2e-single-cluster",
		ClusterName: "e2e-single",
		Flavor:      "mysql",
		ArchType:    "single",
		EncKey:      "e2e-single-key",
		Nodes: []OrchestratorNode{
			{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, Role: "primary", DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", deployResult.Status)
	assert.Len(t, deployResult.InstanceIDs, 1)

	// Rebuild cluster
	lifecycleSvc := NewClusterLifecycleService(orch, pluginExec, vault, repo)
	rebuildResult, err := lifecycleSvc.RebuildCluster(ctx, RebuildClusterRequest{
		OriginalReq: DeployOrchestratorRequest{
			ClusterID:   "e2e-single-cluster",
			ClusterName: "e2e-single-rebuilt",
			Flavor:      "mysql",
			ArchType:    "single",
			EncKey:      "e2e-single-key",
			Nodes: []OrchestratorNode{
				{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, Role: "primary", DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", rebuildResult.Status)
	assert.NotNil(t, rebuildResult.Deploy)
}

func TestEndToEnd_MessageBus_Integration(t *testing.T) {
	bus := NewMessageBus()

	ch := bus.Subscribe("e2e-task-001")
	defer bus.Unsubscribe("e2e-task-001", ch)

	bus.PublishProgress("e2e-task-001", 0, "init", "running")
	bus.PublishProgress("e2e-task-001", 33, "template_build", "running")
	bus.PublishLog("e2e-task-001", "Primary node deployed")
	bus.PublishProgress("e2e-task-001", 66, "replicate", "running")
	bus.PublishLog("e2e-task-001", "Replica 1 deployed")
	bus.PublishProgress("e2e-task-001", 100, "complete", "completed")

	var lastEvent TaskEvent
	for event := range ch {
		lastEvent = event
		if event.Status == "completed" {
			break
		}
	}

	assert.Equal(t, 100, lastEvent.Progress)
	assert.Equal(t, "completed", lastEvent.Status)
	assert.Equal(t, "complete", lastEvent.Stage)
}

func TestEndToEnd_ConfigRenderer_AllFlavors(t *testing.T) {
	renderer := NewConfigRenderer()

	flavors := []struct {
		flavor  string
		version string
		checks  []string
	}{
		{"mysql", "5.7.44", []string{"gtid_mode=ON", "server-id=1", "innodb_buffer_pool_size=2G", "sync_binlog=1"}},
		{"mysql", "8.0.36", []string{"gtid_mode=ON", "sync_binlog=1", "binlog_expire_logs_seconds=604800"}},
		{"mariadb", "10.11.4", []string{"gtid_domain_id=1", "innodb_buffer_pool_size=2G", "sync_binlog=1"}},
		{"percona", "8.0.36-28", []string{"gtid_mode=ON", "sync_binlog=1"}},
	}

	for _, tc := range flavors {
		t.Run(tc.flavor+"-"+tc.version, func(t *testing.T) {
			cfg := NewMySQLConfig(tc.flavor, tc.version, 3306, 1)
			cfg.Datadir = "/data/mysql_3306"
			cfg.Basedir = "/opt/" + tc.flavor
			cfg.InnodbBufferPool = "2G"
			cfg.ReportHost = "10.0.0.1"

			output, err := renderer.Render(cfg)
			require.NoError(t, err)

			for _, check := range tc.checks {
				assert.Contains(t, output, check, "flavor=%s version=%s", tc.flavor, tc.version)
			}
		})
	}
}
