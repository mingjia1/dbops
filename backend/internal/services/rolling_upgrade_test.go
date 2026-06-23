package services

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
	"github.com/jackcode/mysql-ops-platform/internal/plugins/arch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRollingUpgradeService_RequiresMin2Nodes(t *testing.T) {
	registry := plugins.NewRegistry()
	_ = registry.Register(&stubKernelPlugin{name: "mysql-core"})
	vault := NewCredentialVault(newTestCredentialRepo(), "ru-key-1")
	orch := NewDeployOrchestrator(registry, vault, nil)

	svc := NewRollingUpgradeService(orch, nil)
	_, err := svc.ExecuteRollingUpgrade(context.Background(), RollingUpgradeRequest{
		ClusterID: "cluster-001",
		Flavor:    "mysql",
		EncKey:    "test-key",
		Nodes:     []OrchestratorNode{{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, Role: "primary"}},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2")
}

func TestRollingUpgradeService_RequiresPrimary(t *testing.T) {
	registry := plugins.NewRegistry()
	_ = registry.Register(&stubKernelPlugin{name: "mysql-core"})
	vault := NewCredentialVault(newTestCredentialRepo(), "ru-key-2")
	orch := NewDeployOrchestrator(registry, vault, nil)

	svc := NewRollingUpgradeService(orch, nil)
	_, err := svc.ExecuteRollingUpgrade(context.Background(), RollingUpgradeRequest{
		ClusterID: "cluster-001",
		Flavor:    "mysql",
		EncKey:    "test-key",
		Nodes: []OrchestratorNode{
			{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, Role: "replica"},
			{Address: "10.0.0.2", AgentPort: 9090, MySQLPort: 3306, Role: "replica"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "primary")
}

func TestRollingUpgradeService_Success(t *testing.T) {
	registry := plugins.NewRegistry()
	_ = registry.Register(&stubKernelPlugin{name: "mysql-core"})
	fakeAgent := func(_ context.Context, _ string, _ int, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{}, nil
	}
	_ = registry.Register(arch.NewReplicaAddonPlugin(fakeAgent))
	repo := newTestInstanceRepo(context.Background())
	vault := NewCredentialVault(newTestCredentialRepo(), "ru-key-3")
	orch := NewDeployOrchestrator(registry, vault, repo)

	svc := NewRollingUpgradeService(orch, repo)
	result, err := svc.ExecuteRollingUpgrade(context.Background(), RollingUpgradeRequest{
		ClusterID:   "rolling-001",
		Flavor:      "mysql",
		ArchType:    "single",
		EncKey:      "test-key",
		SourceVersion: "5.7.44",
		TargetVersion: "8.0.36",
		Nodes: []OrchestratorNode{
			{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, Role: "primary", DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
			{Address: "10.0.0.2", AgentPort: 9090, MySQLPort: 3306, Role: "replica", DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, 2, result.UpgradedNodes)
}
