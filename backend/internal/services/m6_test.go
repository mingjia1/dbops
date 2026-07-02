package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
	"github.com/jackcode/mysql-ops-platform/internal/plugins/arch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLifecycleRegistry() *plugins.Registry {
	r := plugins.NewRegistry()
	fakeAgent := func(_ context.Context, _ string, _ int, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"status": "ok"}, nil
	}
	_ = r.Register(&stubKernelPlugin{name: "mysql-core"})
	_ = r.Register(&stubKernelPlugin{name: "mariadb-core"})
	_ = r.Register(arch.NewReplicaAddonPlugin(fakeAgent))
	_ = r.Register(arch.NewMHAAddonPlugin(fakeAgent))
	_ = r.Register(arch.NewMGRAddonPlugin(fakeAgent))
	return r
}

func TestScaleService_ScaleOut(t *testing.T) {
	registry := newTestLifecycleRegistry()
	repo := newTestInstanceRepo(t, context.Background())
	vault := NewCredentialVault(newTestCredentialRepo(t), "test-key")
	orch := NewDeployOrchestrator(registry, vault, repo)
	pluginExec := plugins.NewExecutor(registry)

	svc := NewScaleService(orch, repo, pluginExec)

	req := ScaleOutRequest{
		ClusterID: "scale-out-cluster",
		Flavor:    "mysql",
		ArchType:  "replica",
		EncKey:    "test-key",
		NewNodes: []OrchestratorNode{
			{Address: "10.0.0.5", AgentPort: 9090, MySQLPort: 3306, Role: "replica", DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
		},
	}

	result, err := svc.ScaleOut(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Len(t, result.NewInstIDs, 1)
}

func TestScaleService_ScaleOut_NoNodes(t *testing.T) {
	registry := newTestLifecycleRegistry()
	vault := NewCredentialVault(newTestCredentialRepo(t), "test-key")
	orch := NewDeployOrchestrator(registry, vault, nil)
	pluginExec := plugins.NewExecutor(registry)

	svc := NewScaleService(orch, nil, pluginExec)

	_, err := svc.ScaleOut(context.Background(), ScaleOutRequest{Flavor: "mysql", EncKey: "test-key"})
	assert.Error(t, err)
}

func TestScaleService_ScaleIn(t *testing.T) {
	registry := newTestLifecycleRegistry()
	repo := newTestInstanceRepo(t, context.Background())
	vault := NewCredentialVault(newTestCredentialRepo(t), "test-key")
	orch := NewDeployOrchestrator(registry, vault, repo)
	pluginExec := plugins.NewExecutor(registry)

	svc := NewScaleService(orch, repo, pluginExec)

	result, err := svc.ScaleIn(context.Background(), ScaleInRequest{
		ClusterID:    "scale-in-cluster",
		RemoveNodeID: "instance-001",
		Flavor:       "mysql",
		ArchType:     "replica",
		IsPrimary:    false,
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
}

func TestScaleService_ScaleIn_PrimaryRejected(t *testing.T) {
	registry := newTestLifecycleRegistry()
	repo := newTestInstanceRepo(t, context.Background())
	vault := NewCredentialVault(newTestCredentialRepo(t), "test-key")
	orch := NewDeployOrchestrator(registry, vault, repo)
	pluginExec := plugins.NewExecutor(registry)

	svc := NewScaleService(orch, repo, pluginExec)

	_, err := svc.ScaleIn(context.Background(), ScaleInRequest{
		ClusterID:    "scale-in-primary",
		RemoveNodeID: "instance-001",
		IsPrimary:    true,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "primary")
}

func TestScaleService_ScaleIn_EmptyID(t *testing.T) {
	svc := NewScaleService(nil, nil, nil)
	_, err := svc.ScaleIn(context.Background(), ScaleInRequest{})
	assert.Error(t, err)
}

func TestScaleService_RebuildNode(t *testing.T) {
	registry := newTestLifecycleRegistry()
	repo := newTestInstanceRepo(t, context.Background())
	vault := NewCredentialVault(newTestCredentialRepo(t), "test-key")
	orch := NewDeployOrchestrator(registry, vault, repo)
	pluginExec := plugins.NewExecutor(registry)

	svc := NewScaleService(orch, repo, pluginExec)

	result, err := svc.RebuildNode(context.Background(), RebuildRequest{
		ClusterID:  "rebuild-cluster",
		InstanceID: "inst-001",
		Flavor:     "mysql",
		EncKey:     "test-key",
		Node:       OrchestratorNode{Address: "10.0.0.2", AgentPort: 9090, MySQLPort: 3306, DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.GreaterOrEqual(t, result.DurationMs, int64(0))
}

func TestClusterLifecycleService_DestroyCluster(t *testing.T) {
	registry := newTestLifecycleRegistry()
	repo := newTestInstanceRepo(t, context.Background())
	vault := NewCredentialVault(newTestCredentialRepo(t), "test-key")
	orch := NewDeployOrchestrator(registry, vault, repo)
	pluginExec := plugins.NewExecutor(registry)

	svc := NewClusterLifecycleService(orch, pluginExec, vault, repo)

	err := vault.SetCredential(context.Background(), "destroy-cluster", "root", "root", "pass123")
	require.NoError(t, err)

	result, err := svc.DestroyCluster(context.Background(), DestroyRequest{
		ClusterID: "destroy-cluster",
		Flavor:    "mysql",
		ArchType:  "replica",
		EncKey:    "test-key",
		Nodes:     []OrchestratorNode{{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306}},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)

	creds, _ := vault.ListClusterCredentials(context.Background(), "destroy-cluster")
	assert.Len(t, creds, 0)
}

func TestClusterLifecycleService_RebuildCluster(t *testing.T) {
	registry := newTestLifecycleRegistry()
	repo := newTestInstanceRepo(t, context.Background())
	vault := NewCredentialVault(newTestCredentialRepo(t), "test-key")
	orch := NewDeployOrchestrator(registry, vault, repo)
	pluginExec := plugins.NewExecutor(registry)

	svc := NewClusterLifecycleService(orch, pluginExec, vault, repo)

	result, err := svc.RebuildCluster(context.Background(), RebuildClusterRequest{
		OriginalReq: DeployOrchestratorRequest{
			ClusterID:   "cluster-rebuild",
			ClusterName: "test-rebuild",
			Flavor:      "mysql",
			ArchType:    "single",
			EncKey:      "test-key",
			Nodes: []OrchestratorNode{
				{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.NotNil(t, result.Deploy)
}
