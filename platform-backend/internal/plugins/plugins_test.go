package plugins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPlugin struct {
	name       string
	pluginType PluginType
	version    string
	prepErr    error
	execResult *PluginResult
	execErr    error
	rollbacked bool
	tornDown   bool
	DefaultPluginMethods
}

func (s *stubPlugin) Name() string            { return s.name }
func (s *stubPlugin) Type() PluginType         { return s.pluginType }
func (s *stubPlugin) Version() string          { return s.version }
func (s *stubPlugin) Prepare(_ context.Context, _ PluginEnv) error {
	return s.prepErr
}
func (s *stubPlugin) Execute(_ context.Context, _ PluginEnv, _ map[string]interface{}) (*PluginResult, error) {
	return s.execResult, s.execErr
}
func (s *stubPlugin) Rollback(_ context.Context, _ PluginEnv) error {
	s.rollbacked = true
	return nil
}
func (s *stubPlugin) Teardown(_ context.Context, _ PluginEnv) error {
	s.tornDown = true
	return nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	p := &stubPlugin{name: "mysql-core", pluginType: PluginTypeKernel, version: "1.0.0"}

	require.NoError(t, r.Register(p))

	got, err := r.Get("mysql-core")
	require.NoError(t, err)
	assert.Equal(t, "mysql-core", got.Name())
	assert.Equal(t, PluginTypeKernel, got.Type())
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	p := &stubPlugin{name: "mysql-core", pluginType: PluginTypeKernel, version: "1.0.0"}
	require.NoError(t, r.Register(p))

	err := r.Register(p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_RegisterNil(t *testing.T) {
	r := NewRegistry()
	err := r.Register(nil)
	assert.Error(t, err)
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_ListByType(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(&stubPlugin{name: "mysql-core", pluginType: PluginTypeKernel, version: "1.0"})
	_ = r.Register(&stubPlugin{name: "mha-addon", pluginType: PluginTypeArch, version: "1.0"})
	_ = r.Register(&stubPlugin{name: "keepalived-addon", pluginType: PluginTypeMiddleware, version: "1.0"})

	kernels := r.ListByType(PluginTypeKernel)
	assert.Len(t, kernels, 1)
	assert.Equal(t, "mysql-core", kernels[0].Name())

	archs := r.ListByType(PluginTypeArch)
	assert.Len(t, archs, 1)

	all := r.List()
	assert.Len(t, all, 3)
}

func TestExecutor_RunExecute_Success(t *testing.T) {
	r := NewRegistry()
	p := &stubPlugin{
		name:       "mysql-core",
		pluginType: PluginTypeKernel,
		version:    "1.0",
		execResult: &PluginResult{Success: true, Message: "ok"},
	}
	_ = r.Register(p)

	exec := NewExecutor(r)
	result, err := exec.RunExecute(context.Background(), "mysql-core", PluginEnv{}, nil)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.False(t, p.rollbacked)
}

func TestExecutor_RunExecute_FailTriggersRollback(t *testing.T) {
	r := NewRegistry()
	p := &stubPlugin{
		name:       "mysql-core",
		pluginType: PluginTypeKernel,
		version:    "1.0",
		execErr:    assert.AnError,
	}
	_ = r.Register(p)

	exec := NewExecutor(r)
	_, err := exec.RunExecute(context.Background(), "mysql-core", PluginEnv{}, nil)
	assert.Error(t, err)
	assert.True(t, p.rollbacked)
}

func TestExecutor_RunTeardown(t *testing.T) {
	r := NewRegistry()
	p := &stubPlugin{name: "mysql-core", pluginType: PluginTypeKernel, version: "1.0"}
	_ = r.Register(p)

	exec := NewExecutor(r)
	err := exec.RunTeardown(context.Background(), "mysql-core", PluginEnv{})
	assert.NoError(t, err)
	assert.True(t, p.tornDown)
}

func TestExecutor_RunExecute_PrepareFails(t *testing.T) {
	r := NewRegistry()
	p := &stubPlugin{
		name:       "mysql-core",
		pluginType: PluginTypeKernel,
		version:    "1.0",
		prepErr:    assert.AnError,
	}
	_ = r.Register(p)

	exec := NewExecutor(r)
	_, err := exec.RunExecute(context.Background(), "mysql-core", PluginEnv{}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prepare")
}
