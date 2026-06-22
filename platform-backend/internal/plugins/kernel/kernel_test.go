package kernel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAgentCaller struct {
	calls   []agentCall
	resp    map[string]interface{}
	respErr error
}

type agentCall struct {
	Host      string
	AgentPort int
	Path      string
	Payload   map[string]interface{}
}

func (f *fakeAgentCaller) call(_ context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error) {
	f.calls = append(f.calls, agentCall{Host: host, AgentPort: agentPort, Path: path, Payload: payload})
	return f.resp, f.respErr
}

func testEnv() KernelEnv {
	return KernelEnv{
		ClusterID: "cluster-001",
		Node: KernelNode{
			HostID:    "host-001",
			Address:   "192.168.1.10",
			AgentPort: 9090,
			MySQLPort: 3306,
			Role:      "primary",
		},
		Creds: KernelCredentials{
			RootPassword:    "rootpass",
			ReplUser:        "repl",
			ReplPassword:    "replpass",
			MonitorUser:     "monitor",
			MonitorPassword: "monpass",
		},
	}
}

func TestMySQLCorePlugin_FlavorAndName(t *testing.T) {
	p := NewMySQLCorePlugin(nil)
	assert.Equal(t, "mysql-core", p.Name())
	assert.Equal(t, "kernel", p.Type())
	assert.Equal(t, "mysql", p.Flavor())
}

func TestMySQLCorePlugin_Prepare_Defaults(t *testing.T) {
	p := NewMySQLCorePlugin(nil)
	env := testEnv()
	env.Node.DataDir = ""
	env.Node.Basedir = ""
	env.Node.MySQLPort = 3307

	err := p.Prepare(context.Background(), env)
	require.NoError(t, err)
	assert.Equal(t, 3307, env.Node.MySQLPort, "Prepare should not panic on valid env with defaults empty")
}

func TestMySQLCorePlugin_Prepare_SetsDefaultsInternally(t *testing.T) {
	p := NewMySQLCorePlugin(nil)
	env := testEnv()
	env.Node.DataDir = "/custom/data"
	env.Node.Basedir = "/custom/basedir"
	env.Node.MySQLPort = 3307

	err := p.Prepare(context.Background(), env)
	require.NoError(t, err)
	assert.Equal(t, "/custom/data", env.Node.DataDir)
	assert.Equal(t, "/custom/basedir", env.Node.Basedir)
}

func TestMySQLCorePlugin_Prepare_MissingAddress(t *testing.T) {
	p := NewMySQLCorePlugin(nil)
	env := testEnv()
	env.Node.Address = ""

	err := p.Prepare(context.Background(), env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "address")
}

func TestMySQLCorePlugin_Prepare_MissingAgentPort(t *testing.T) {
	p := NewMySQLCorePlugin(nil)
	env := testEnv()
	env.Node.AgentPort = 0

	err := p.Prepare(context.Background(), env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent port")
}

func TestMySQLCorePlugin_Execute_Success(t *testing.T) {
	fake := &fakeAgentCaller{resp: map[string]interface{}{"status": "ok"}}
	p := NewMySQLCorePlugin(fake.call)
	env := testEnv()
	env.Node.DataDir = "/data/mysql_3306"
	env.Node.Basedir = "/opt/mysql-3306"

	result, err := p.Execute(context.Background(), env, nil)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Len(t, fake.calls, 1)
	assert.Equal(t, "/api/v1/deploy/init", fake.calls[0].Path)
	assert.Equal(t, "mysql", fake.calls[0].Payload["flavor"])
}

func TestMySQLCorePlugin_Execute_NilCaller(t *testing.T) {
	p := NewMySQLCorePlugin(nil)
	env := testEnv()
	_, err := p.Execute(context.Background(), env, nil)
	assert.Error(t, err)
}

func TestMySQLCorePlugin_Execute_AgentError(t *testing.T) {
	fake := &fakeAgentCaller{respErr: assert.AnError}
	p := NewMySQLCorePlugin(fake.call)
	env := testEnv()
	env.Node.DataDir = "/data/mysql_3306"
	env.Node.Basedir = "/opt/mysql-3306"

	_, err := p.Execute(context.Background(), env, nil)
	assert.Error(t, err)
}

func TestPerconaCorePlugin_Flavor(t *testing.T) {
	fake := &fakeAgentCaller{resp: map[string]interface{}{}}
	p := NewPerconaCorePlugin(fake.call)
	assert.Equal(t, "percona-core", p.Name())
	assert.Equal(t, "percona", p.Flavor())

	env := testEnv()
	env.Node.DataDir = "/data/mysql_3306"
	env.Node.Basedir = "/opt/percona-3306"
	result, err := p.Execute(context.Background(), env, nil)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "percona", fake.calls[0].Payload["flavor"])
}

func TestMariaDBCorePlugin_Flavor(t *testing.T) {
	fake := &fakeAgentCaller{resp: map[string]interface{}{}}
	p := NewMariaDBCorePlugin(fake.call)
	assert.Equal(t, "mariadb-core", p.Name())
	assert.Equal(t, "mariadb", p.Flavor())

	env := testEnv()
	env.Node.DataDir = "/data/mysql_3306"
	env.Node.Basedir = "/opt/mariadb-3306"
	env.Node.ServerID = 1
	result, err := p.Execute(context.Background(), env, nil)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "mariadb", fake.calls[0].Payload["flavor"])
	assert.Equal(t, 1, fake.calls[0].Payload["gtid_domain_id"])
}

func TestKernelPlugins_Rollback(t *testing.T) {
	fake := &fakeAgentCaller{resp: map[string]interface{}{}}
	p := NewMySQLCorePlugin(fake.call)
	env := testEnv()
	env.Node.DataDir = "/data/mysql_3306"
	env.Node.Basedir = "/opt/mysql-3306"

	err := p.Rollback(context.Background(), env)
	assert.NoError(t, err)
	assert.Equal(t, "/api/v1/deploy/rollback", fake.calls[0].Path)
}

func TestKernelPlugins_Teardown(t *testing.T) {
	fake := &fakeAgentCaller{resp: map[string]interface{}{}}
	p := NewMySQLCorePlugin(fake.call)
	env := testEnv()
	env.Node.DataDir = "/data/mysql_3306"
	env.Node.Basedir = "/opt/mysql-3306"

	err := p.Teardown(context.Background(), env)
	assert.NoError(t, err)
	assert.Equal(t, "/api/v1/deploy/teardown", fake.calls[0].Path)
}
