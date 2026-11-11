package plugins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_UnregisterProtectsBuiltin(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.Register(&stubPlugin{name: "replica-addon", pluginType: PluginTypeArch, version: "1.0"}))

	err := r.Unregister("replica-addon")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "builtin")

	_, err = r.Get("replica-addon")
	assert.NoError(t, err, "builtin plugin must remain registered")
}

func TestRegistry_UnregisterDynamic(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.RegisterDynamic(NewHTTPPlugin(HTTPPluginConfig{
		Name: "demo-addon", Endpoint: "http://127.0.0.1:9100",
	})))

	info, ok := r.GetInfo("demo-addon")
	require.True(t, ok)
	assert.Equal(t, "dynamic", info.Source)

	require.NoError(t, r.Unregister("demo-addon"))
	_, ok = r.GetInfo("demo-addon")
	assert.False(t, ok)
}

func TestRegistry_DynamicCannotShadowBuiltin(t *testing.T) {
	r := NewRegistry()
	require.NoError(t, r.RegisterMetadata(PluginInfo{Name: "mysql-core", Type: "kernel", Version: "1.0.0"}))
	err := r.RegisterDynamic(NewHTTPPlugin(HTTPPluginConfig{
		Name: "mysql-core", Endpoint: "http://127.0.0.1:9100",
	}))
	require.Error(t, err)
}

func TestHTTPPluginConfig_Validate(t *testing.T) {
	valid := HTTPPluginConfig{Name: "demo-addon", Endpoint: "http://127.0.0.1:9100/hook"}
	require.NoError(t, valid.Validate())

	bad := []HTTPPluginConfig{
		{Name: "../escape", Endpoint: valid.Endpoint},
		{Name: "demo", Endpoint: "file:///etc/passwd"},
		{Name: "demo", Endpoint: "http://127.0.0.1:9100/hook?token=x"},
		{Name: "demo", Endpoint: ""},
	}
	for _, cfg := range bad {
		assert.Error(t, cfg.Validate(), "cfg=%+v", cfg)
	}
}

func TestHTTPPlugin_ExecuteAndPing(t *testing.T) {
	var gotAction string
	var gotParams map[string]interface{}
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAction = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		var req pluginHTTPRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotParams = req.Params
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/execute" {
			_, _ = w.Write([]byte(`{"success":true,"message":"deployed","data":{"nodes":3}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	cfg := HTTPPluginConfig{Name: "demo-addon", Type: "arch", Endpoint: srv.URL, Token: "secret"}
	require.NoError(t, cfg.Validate())
	p := NewHTTPPlugin(cfg)

	require.NoError(t, p.Ping(context.Background()))
	assert.Equal(t, "/ping", gotAction)
	assert.Equal(t, "Bearer secret", gotAuth)

	result, err := p.Execute(context.Background(), PluginEnv{ClusterID: "c-1"}, map[string]interface{}{"flavor": "mysql"})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "deployed", result.Message)
	assert.Equal(t, "/execute", gotAction)
	assert.Equal(t, "mysql", gotParams["flavor"])
}

func TestHTTPPlugin_ExecuteFailureTriggersRollback(t *testing.T) {
	var actions []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actions = append(actions, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/execute" {
			_, _ = w.Write([]byte(`{"success":false,"message":"disk full"}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	r := NewRegistry()
	require.NoError(t, r.RegisterDynamic(NewHTTPPlugin(HTTPPluginConfig{
		Name: "demo-addon", Endpoint: srv.URL,
	})))

	_, err := NewExecutor(r).RunExecute(context.Background(), "demo-addon", PluginEnv{}, nil)
	require.Error(t, err)
	assert.Contains(t, actions, "/prepare")
	assert.Contains(t, actions, "/execute")
	assert.Contains(t, actions, "/rollback")
}
