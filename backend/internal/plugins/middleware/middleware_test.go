package middleware

import (
	"context"
	"testing"

	"github.com/jackcode/mysql-ops-platform/internal/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mwFakeCaller struct {
	calls   []string
	ports   []int
	resp    map[string]interface{}
	respErr error
}

func (f *mwFakeCaller) call(_ context.Context, _ string, port int, path string, _ map[string]interface{}) (map[string]interface{}, error) {
	f.calls = append(f.calls, path)
	f.ports = append(f.ports, port)
	return f.resp, f.respErr
}

func TestKeepalivedAddon_Success(t *testing.T) {
	fake := &mwFakeCaller{resp: map[string]interface{}{"status": "ok"}}
	p := NewKeepalivedAddonPlugin(fake.call)

	env := plugins.PluginEnv{
		ClusterID: "test-cluster",
		Nodes: []plugins.PluginNode{
			{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306},
			{Address: "10.0.0.2", AgentPort: 9090, MySQLPort: 3306},
		},
	}

	result, err := p.Execute(context.Background(), env, map[string]interface{}{
		"vip": "10.0.0.100", "vip_interface": "eth0",
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Len(t, fake.calls, 2)
	assert.Equal(t, "/agent/tasks/keepalived-setup", fake.calls[0])
}

func TestKeepalivedAddon_PrepareRequiresMin2(t *testing.T) {
	p := NewKeepalivedAddonPlugin(nil)
	env := plugins.PluginEnv{Nodes: []plugins.PluginNode{{Address: "10.0.0.1"}}}
	err := p.Prepare(context.Background(), env)
	assert.Error(t, err)
}

func TestKeepalivedAddon_NilCaller(t *testing.T) {
	p := NewKeepalivedAddonPlugin(nil)
	env := plugins.PluginEnv{Nodes: []plugins.PluginNode{{Address: "10.0.0.1"}, {Address: "10.0.0.2"}}}
	_, err := p.Execute(context.Background(), env, nil)
	assert.Error(t, err)
}

func TestKeepalivedAddon_AgentFailedStatus(t *testing.T) {
	fake := &mwFakeCaller{resp: map[string]interface{}{"status": "failed", "message": "service start failed"}}
	p := NewKeepalivedAddonPlugin(fake.call)

	env := plugins.PluginEnv{
		ClusterID: "test-cluster",
		Nodes: []plugins.PluginNode{
			{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306},
			{Address: "10.0.0.2", AgentPort: 9090, MySQLPort: 3306},
		},
	}

	_, err := p.Execute(context.Background(), env, map[string]interface{}{
		"vip": "10.0.0.100", "vip_interface": "eth0",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service start failed")
}

func TestProxySQLAddon_Success(t *testing.T) {
	fake := &mwFakeCaller{resp: map[string]interface{}{"status": "ok"}}
	p := NewProxySQLAddonPlugin(fake.call)

	env := plugins.PluginEnv{
		ClusterID: "test-cluster",
		Nodes: []plugins.PluginNode{
			{Address: "10.0.0.1", MySQLPort: 3306, Role: "primary"},
			{Address: "10.0.0.2", MySQLPort: 3306, Role: "replica"},
		},
		Credentials: plugins.CredentialSet{RootUser: "root", RootPassword: "pass"},
	}

	result, err := p.Execute(context.Background(), env, map[string]interface{}{
		"proxy_host": "10.0.0.10",
		"proxy_port": float64(6033),
		"agent_port": float64(9091),
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
	require.Equal(t, []string{"/agent/tasks/proxysql-setup"}, fake.calls)
	require.Equal(t, []int{9091}, fake.ports)
}

func TestProxySQLAddon_MissingProxyHost(t *testing.T) {
	fake := &mwFakeCaller{}
	p := NewProxySQLAddonPlugin(fake.call)

	env := plugins.PluginEnv{
		Nodes: []plugins.PluginNode{{Address: "10.0.0.1", MySQLPort: 3306}},
	}
	_, err := p.Execute(context.Background(), env, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "proxy_host")
}

func TestProxySQLAddon_AgentFailedStatus(t *testing.T) {
	fake := &mwFakeCaller{resp: map[string]interface{}{"status": "failed", "message": "backend config failed"}}
	p := NewProxySQLAddonPlugin(fake.call)

	env := plugins.PluginEnv{
		ClusterID: "test-cluster",
		Nodes: []plugins.PluginNode{
			{Address: "10.0.0.1", MySQLPort: 3306, Role: "primary"},
			{Address: "10.0.0.2", MySQLPort: 3306, Role: "replica"},
		},
		Credentials: plugins.CredentialSet{RootUser: "root", RootPassword: "pass"},
	}

	_, err := p.Execute(context.Background(), env, map[string]interface{}{
		"proxy_host": "10.0.0.10",
		"proxy_port": float64(6033),
		"agent_port": float64(9091),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "backend config failed")
}

func TestMiddlewarePluginNames(t *testing.T) {
	assert.Equal(t, "keepalived-addon", NewKeepalivedAddonPlugin(nil).Name())
	assert.Equal(t, "proxysql-addon", NewProxySQLAddonPlugin(nil).Name())
	assert.Equal(t, plugins.PluginTypeMiddleware, NewKeepalivedAddonPlugin(nil).Type())
	assert.Equal(t, plugins.PluginTypeMiddleware, NewProxySQLAddonPlugin(nil).Type())
}
