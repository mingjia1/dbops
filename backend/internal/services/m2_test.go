package services

import (
	"context"
	"strings"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
	"github.com/jackcode/mysql-ops-platform/internal/plugins/arch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubKernelPlugin struct {
	name string
	plugins.DefaultPluginMethods
}

func (s *stubKernelPlugin) Name() string              { return s.name }
func (s *stubKernelPlugin) Type() plugins.PluginType   { return plugins.PluginTypeKernel }
func (s *stubKernelPlugin) Version() string            { return "1.0.0" }
func (s *stubKernelPlugin) Prepare(_ context.Context, _ plugins.PluginEnv) error { return nil }
func (s *stubKernelPlugin) Execute(_ context.Context, _ plugins.PluginEnv, _ map[string]interface{}) (*plugins.PluginResult, error) {
	return &plugins.PluginResult{Success: true, Message: "deployed"}, nil
}
func (s *stubKernelPlugin) Rollback(_ context.Context, _ plugins.PluginEnv) error { return nil }
func (s *stubKernelPlugin) Teardown(_ context.Context, _ plugins.PluginEnv) error { return nil }

func newTestOrchestratorRegistry() *plugins.Registry {
	r := plugins.NewRegistry()
	fakeAgent := func(_ context.Context, _ string, _ int, _ string, _ map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"status": "ok"}, nil
	}
	_ = r.Register(&stubKernelPlugin{name: "mysql-core"})
	_ = r.Register(&stubKernelPlugin{name: "mariadb-core"})
	_ = r.Register(&stubKernelPlugin{name: "percona-core"})
	_ = r.Register(arch.NewReplicaAddonPlugin(fakeAgent))
	_ = r.Register(arch.NewMHAAddonPlugin(fakeAgent))
	_ = r.Register(arch.NewMGRAddonPlugin(fakeAgent))
	return r
}

func TestDeployOrchestrator_SingleNode(t *testing.T) {
	registry := newTestOrchestratorRegistry()
	repo := newTestInstanceRepo(context.Background())
	vault := NewCredentialVault(newTestCredentialRepo(), "test-key")

	orch := NewDeployOrchestrator(registry, vault, repo)

	req := DeployOrchestratorRequest{
		ClusterID:   "cluster-single",
		ClusterName: "test-single",
		Flavor:      "mysql",
		ArchType:    "single",
		EncKey:      "test-key",
		Nodes: []OrchestratorNode{
			{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, Role: "primary", DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
		},
	}

	result, err := orch.Run(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Len(t, result.InstanceIDs, 1)
	assert.Len(t, result.Phases, 3)
	assert.Equal(t, "skipped", result.Phases[1].Status)
	assert.Equal(t, "skipped", result.Phases[2].Status)
}

func TestDeployOrchestrator_ThreeNodeHA(t *testing.T) {
	registry := newTestOrchestratorRegistry()
	repo := newTestInstanceRepo(context.Background())
	vault := NewCredentialVault(newTestCredentialRepo(), "test-key")

	orch := NewDeployOrchestrator(registry, vault, repo)

	req := DeployOrchestratorRequest{
		ClusterID:   "cluster-ha",
		ClusterName: "test-ha",
		Flavor:      "mysql",
		ArchType:    "replica",
		EncKey:      "test-key",
		Nodes: []OrchestratorNode{
			{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, Role: "primary", ServerID: 1, DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
			{Address: "10.0.0.2", AgentPort: 9090, MySQLPort: 3306, Role: "replica", ServerID: 2, DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
			{Address: "10.0.0.3", AgentPort: 9090, MySQLPort: 3306, Role: "replica", ServerID: 3, DataDir: "/data/mysql_3306", Basedir: "/opt/mysql"},
		},
	}

	result, err := orch.Run(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Len(t, result.InstanceIDs, 3)
}

func TestDeployOrchestrator_NoNodes(t *testing.T) {
	registry := newTestOrchestratorRegistry()
	vault := NewCredentialVault(newTestCredentialRepo(), "test-key")
	orch := NewDeployOrchestrator(registry, vault, nil)

	req := DeployOrchestratorRequest{
		Flavor:   "mysql",
		ArchType: "single",
		EncKey:   "test-key",
	}

	_, err := orch.Run(context.Background(), req)
	assert.Error(t, err)
}

func TestConfigRenderer_MySQL57(t *testing.T) {
	r := NewConfigRenderer()
	cfg := NewMySQLConfig("mysql", "5.7.44", 3306, 1)
	cfg.Datadir = "/data/mysql_3306"
	cfg.Basedir = "/opt/mysql"
	cfg.ReportHost = "10.0.0.1"

	output, err := r.Render(cfg)
	require.NoError(t, err)
	assert.Contains(t, output, "gtid_mode=ON")
	assert.Contains(t, output, "server-id=1")
	assert.NotContains(t, output, "default_authentication_plugin")
}

func TestConfigRenderer_MySQL80(t *testing.T) {
	r := NewConfigRenderer()
	cfg := NewMySQLConfig("mysql", "8.0.36", 3307, 2)
	cfg.Datadir = "/data/mysql_3307"
	cfg.Basedir = "/opt/mysql"
	cfg.ReportHost = "10.0.0.2"

	output, err := r.Render(cfg)
	require.NoError(t, err)
	assert.Contains(t, output, "gtid_mode=ON")
	assert.Contains(t, output, "default_authentication_plugin=mysql_native_password")
}

func TestConfigRenderer_MariaDB(t *testing.T) {
	r := NewConfigRenderer()
	cfg := NewMySQLConfig("mariadb", "10.11.4", 3306, 5)
	cfg.Datadir = "/data/mysql_3306"
	cfg.Basedir = "/opt/mariadb"
	cfg.ReportHost = "10.0.0.1"

	output, err := r.Render(cfg)
	require.NoError(t, err)
	assert.Contains(t, output, "gtid_domain_id=5")
	assert.NotContains(t, output, "gtid_mode=ON")
	assert.NotContains(t, output, "enforce_gtid_consistency")
}

func TestConfigRenderer_Percona(t *testing.T) {
	r := NewConfigRenderer()
	cfg := NewMySQLConfig("percona", "8.0.36-28", 3306, 1)
	cfg.Datadir = "/data/mysql_3306"
	cfg.Basedir = "/opt/percona"
	cfg.ReportHost = "10.0.0.1"

	output, err := r.Render(cfg)
	require.NoError(t, err)
	assert.Contains(t, output, "gtid_mode=ON")
	assert.Contains(t, output, "default_authentication_plugin=mysql_native_password")
}

func TestNewMySQLConfig_Defaults(t *testing.T) {
	cfg := NewMySQLConfig("mysql", "8.0.36", 3306, 1)
	assert.Equal(t, "mysql", cfg.Flavor)
	assert.Equal(t, "8.0", cfg.MajorMinor)
	assert.Equal(t, 3306, cfg.Port)
	assert.Equal(t, 1, cfg.ServerID)
	assert.Equal(t, "utf8mb4", cfg.CharacterSet)
	assert.Equal(t, 500, cfg.MaxConnections)
	assert.True(t, cfg.GTIDMode)
	assert.True(t, cfg.EnforceGTID)
}

func TestNewMySQLConfig_MariaDB(t *testing.T) {
	cfg := NewMySQLConfig("mariadb", "10.11.4", 3306, 7)
	assert.False(t, cfg.GTIDMode)
	assert.False(t, cfg.EnforceGTID)
	assert.Equal(t, 7, cfg.GTIDDomainID)
	assert.Equal(t, "mariadbd", cfg.BinaryName)
}

func TestMajorMinor(t *testing.T) {
	assert.Equal(t, "8.0", majorMinor("8.0.36"))
	assert.Equal(t, "5.7", majorMinor("5.7.44"))
	assert.Equal(t, "10.11", majorMinor("10.11.4"))
	assert.Equal(t, "8", majorMinor("8"))
}

func TestConfigRenderer_MGRConfig(t *testing.T) {
	r := NewConfigRenderer()
	cfg := NewMySQLConfig("mysql", "8.0.36", 3306, 1)
	cfg.Datadir = "/data/mysql_3306"
	cfg.Basedir = "/opt/mysql"
	cfg.ReportHost = "10.0.0.1"
	cfg.GroupReplication = true
	cfg.GroupLocalAddr = "10.0.0.1:33061"
	cfg.GroupSeeds = "10.0.0.1:33061,10.0.0.2:33061,10.0.0.3:33061"

	output, err := r.Render(cfg)
	require.NoError(t, err)
	assert.Contains(t, output, "group_replication_local_address=10.0.0.1:33061")
	assert.Contains(t, output, "group_replication_start_on_boot=OFF")
}

func TestConfigRenderer_NoBinlog(t *testing.T) {
	r := NewConfigRenderer()
	cfg := NewMySQLConfig("mysql", "8.0.36", 3306, 1)
	cfg.Datadir = "/data/mysql_3306"
	cfg.Basedir = "/opt/mysql"
	cfg.LogBin = false
	cfg.ReportHost = "10.0.0.1"

	output, err := r.Render(cfg)
	require.NoError(t, err)
	assert.NotContains(t, output, "log_bin=")
}

func TestConfigRenderer_DifferentPorts(t *testing.T) {
	r := NewConfigRenderer()
	cfg := NewMySQLConfig("mysql", "8.0.36", 3307, 2)
	cfg.Datadir = "/data/mysql_3307"
	cfg.Basedir = "/opt/mysql"
	cfg.ReportHost = "10.0.0.2"

	output, err := r.Render(cfg)
	require.NoError(t, err)
	assert.True(t, strings.Contains(output, "port=3307"))
	assert.True(t, strings.Contains(output, "server-id=2"))
}
