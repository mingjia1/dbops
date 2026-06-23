package arch

import (
	"context"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type archFakeCaller struct {
	calls   []string
	resp    map[string]interface{}
	respErr error
}

func (f *archFakeCaller) call(_ context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error) {
	f.calls = append(f.calls, path)
	return f.resp, f.respErr
}

func testPluginEnv(n int) plugins.PluginEnv {
	nodes := make([]plugins.PluginNode, n)
	nodes[0] = plugins.PluginNode{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, Role: "primary"}
	for i := 1; i < n; i++ {
		nodes[i] = plugins.PluginNode{Address: "10.0.0." + string(rune('1'+i)), AgentPort: 9090, MySQLPort: 3306, Role: "replica"}
	}
	return plugins.PluginEnv{
		ClusterID: "test-cluster",
		Nodes:     nodes,
		Credentials: plugins.CredentialSet{
			ReplUser: "repl", ReplPassword: "replpass",
		},
	}
}

func TestReplicaAddon_Success(t *testing.T) {
	fake := &archFakeCaller{resp: map[string]interface{}{"status": "ok"}}
	p := NewReplicaAddonPlugin(fake.call)

	env := testPluginEnv(3)
	result, err := p.Execute(context.Background(), env, nil)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Len(t, fake.calls, 2)
}

func TestReplicaAddon_NoReplica(t *testing.T) {
	fake := &archFakeCaller{}
	p := NewReplicaAddonPlugin(fake.call)

	env := testPluginEnv(1)
	_, err := p.Execute(context.Background(), env, nil)
	assert.Error(t, err)
}

func TestReplicaAddon_PrepareRequiresMin2(t *testing.T) {
	p := NewReplicaAddonPlugin(nil)
	env := testPluginEnv(1)
	err := p.Prepare(context.Background(), env)
	assert.Error(t, err)
}

func TestReplicaAddon_PrepareRequiresReplCreds(t *testing.T) {
	p := NewReplicaAddonPlugin(nil)
	env := testPluginEnv(3)
	env.Credentials.ReplUser = ""
	err := p.Prepare(context.Background(), env)
	assert.Error(t, err)
}

func TestMHAAddon_Success(t *testing.T) {
	fake := &archFakeCaller{resp: map[string]interface{}{"status": "ok"}}
	p := NewMHAAddonPlugin(fake.call)

	env := testPluginEnv(3)
	env.Nodes[0].Role = "master"
	env.Nodes = append(env.Nodes, plugins.PluginNode{Address: "10.0.0.100", AgentPort: 9090, Role: "manager"})
	result, err := p.Execute(context.Background(), env, nil)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, fake.calls, "/api/v1/mha/setup")
}

func TestMHAAddon_PrepareRequiresMin2(t *testing.T) {
	p := NewMHAAddonPlugin(nil)
	env := testPluginEnv(1)
	err := p.Prepare(context.Background(), env)
	assert.Error(t, err)
}

func TestMGRAddon_Success(t *testing.T) {
	fake := &archFakeCaller{resp: map[string]interface{}{"status": "ok"}}
	p := NewMGRAddonPlugin(fake.call)

	env := testPluginEnv(3)
	env.Nodes[0].Role = "primary"
	result, err := p.Execute(context.Background(), env, nil)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Len(t, fake.calls, 3)
	assert.Contains(t, fake.calls, "/api/v1/mgr/setup")
}

func TestMGRAddon_NilCaller(t *testing.T) {
	p := NewMGRAddonPlugin(nil)
	env := testPluginEnv(3)
	_, err := p.Execute(context.Background(), env, nil)
	assert.Error(t, err)
}

func TestArchPluginNames(t *testing.T) {
	assert.Equal(t, "replica-addon", NewReplicaAddonPlugin(nil).Name())
	assert.Equal(t, "mha-addon", NewMHAAddonPlugin(nil).Name())
	assert.Equal(t, "mgr-addon", NewMGRAddonPlugin(nil).Name())
}

func TestArchPluginTypes(t *testing.T) {
	assert.Equal(t, plugins.PluginTypeArch, NewReplicaAddonPlugin(nil).Type())
	assert.Equal(t, plugins.PluginTypeArch, NewMHAAddonPlugin(nil).Type())
	assert.Equal(t, plugins.PluginTypeArch, NewMGRAddonPlugin(nil).Type())
}

func TestReplicaAddon_Join(t *testing.T) {
	fake := &archFakeCaller{resp: map[string]interface{}{"status": "ok"}}
	p := NewReplicaAddonPlugin(fake.call)

	env := plugins.PluginEnv{
		ClusterID: "test-cluster",
		Nodes: []plugins.PluginNode{
			{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, Role: "primary"},
		},
		Credentials: plugins.CredentialSet{ReplUser: "repl", ReplPassword: "replpass"},
	}

	err := p.Join(context.Background(), env, plugins.PluginNode{
		Address: "10.0.0.5", AgentPort: 9090, MySQLPort: 3306,
	})
	require.NoError(t, err)
	assert.Len(t, fake.calls, 1)
	assert.Equal(t, "/api/v1/replication/setup", fake.calls[0])
}

func TestMGRAddon_Join(t *testing.T) {
	fake := &archFakeCaller{resp: map[string]interface{}{"status": "ok"}}
	p := NewMGRAddonPlugin(fake.call)

	env := plugins.PluginEnv{
		ClusterID: "test-cluster",
		Nodes: []plugins.PluginNode{
			{Address: "10.0.0.1", AgentPort: 9090},
		},
	}

	err := p.Join(context.Background(), env, plugins.PluginNode{
		Address: "10.0.0.4", AgentPort: 9090,
	})
	require.NoError(t, err)
	assert.Len(t, fake.calls, 1)
	assert.Equal(t, "/api/v1/mgr/setup", fake.calls[0])
}

func TestMGRAddon_Leave(t *testing.T) {
	fake := &archFakeCaller{resp: map[string]interface{}{"status": "ok"}}
	p := NewMGRAddonPlugin(fake.call)

	env := plugins.PluginEnv{ClusterID: "test-cluster"}
	err := p.Leave(context.Background(), env, plugins.PluginNode{Address: "10.0.0.2"})
	require.NoError(t, err)
	assert.Len(t, fake.calls, 1)
}

func TestPXCGaleraAddon_Join(t *testing.T) {
	fake := &archFakeCaller{resp: map[string]interface{}{"status": "ok"}}
	p := NewPXCGaleraAddonPlugin(fake.call)

	env := plugins.PluginEnv{
		ClusterID: "test-cluster",
		Nodes: []plugins.PluginNode{
			{Address: "10.0.0.1", AgentPort: 9090},
		},
	}

	err := p.Join(context.Background(), env, plugins.PluginNode{
		Address: "10.0.0.4", AgentPort: 9090,
	})
	require.NoError(t, err)
	assert.Len(t, fake.calls, 1)
	assert.Equal(t, "/api/v1/pxc/setup", fake.calls[0])
}

func TestReplicaAddon_Join_NilCaller(t *testing.T) {
	p := NewReplicaAddonPlugin(nil)
	err := p.Join(context.Background(), plugins.PluginEnv{}, plugins.PluginNode{})
	assert.Error(t, err)
}

func TestMGRAddon_Join_NilCaller(t *testing.T) {
	p := NewMGRAddonPlugin(nil)
	err := p.Join(context.Background(), plugins.PluginEnv{}, plugins.PluginNode{})
	assert.Error(t, err)
}
