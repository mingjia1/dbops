package arch

import (
	"context"
	"testing"

	"github.com/monkeycode/mysql-ops-platform/internal/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPXCGaleraAddon_Success(t *testing.T) {
	fake := &archFakeCaller{resp: map[string]interface{}{"status": "ok"}}
	p := NewPXCGaleraAddonPlugin(fake.call)

	env := plugins.PluginEnv{
		ClusterID: "test-cluster",
		Nodes: []plugins.PluginNode{
			{Address: "10.0.0.1", AgentPort: 9090, MySQLPort: 3306, Role: "bootstrap"},
			{Address: "10.0.0.2", AgentPort: 9090, MySQLPort: 3306, Role: "member"},
			{Address: "10.0.0.3", AgentPort: 9090, MySQLPort: 3306, Role: "member"},
		},
	}

	result, err := p.Execute(context.Background(), env, nil)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Len(t, fake.calls, 3)
	assert.Equal(t, "/api/v1/pxc/setup", fake.calls[0])
}

func TestPXCGaleraAddon_PrepareRequires1Node(t *testing.T) {
	p := NewPXCGaleraAddonPlugin(nil)
	env := plugins.PluginEnv{Nodes: []plugins.PluginNode{}}
	err := p.Prepare(context.Background(), env)
	assert.Error(t, err)
}

func TestPXCGaleraAddon_NilCaller(t *testing.T) {
	p := NewPXCGaleraAddonPlugin(nil)
	env := plugins.PluginEnv{Nodes: []plugins.PluginNode{{Address: "10.0.0.1"}}}
	_, err := p.Execute(context.Background(), env, nil)
	assert.Error(t, err)
}

func TestPXCGaleraAddon_Name(t *testing.T) {
	p := NewPXCGaleraAddonPlugin(nil)
	assert.Equal(t, "pxc-galera-addon", p.Name())
	assert.Equal(t, plugins.PluginTypeArch, p.Type())
}
