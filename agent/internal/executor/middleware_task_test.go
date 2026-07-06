package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseProxySQLBackends(t *testing.T) {
	backends := parseProxySQLBackends([]interface{}{
		map[string]interface{}{"host": "10.0.0.11", "port": 3306},
		map[string]interface{}{"host": "10.0.0.12", "port": float64(3307)},
		"ignored",
	})

	if len(backends) != 2 {
		t.Fatalf("expected 2 backends, got %d", len(backends))
	}
	if backends[0].Host != "10.0.0.11" || backends[0].Port != 3306 {
		t.Fatalf("unexpected first backend: %#v", backends[0])
	}
	if backends[1].Host != "10.0.0.12" || backends[1].Port != 3307 {
		t.Fatalf("unexpected second backend: %#v", backends[1])
	}
}

func TestParseProxySQLBackendsRejectsInvalidShape(t *testing.T) {
	if got := parseProxySQLBackends(map[string]interface{}{"host": "10.0.0.11"}); got != nil {
		t.Fatalf("expected nil for invalid shape, got %#v", got)
	}
}

func TestMiddlewareTaskResultShape(t *testing.T) {
	result := middlewareTaskResult("task-1", "completed", 100, "ok", map[string]any{"component": "proxysql"})

	if result.TaskID != "task-1" || result.Status != "completed" || result.Progress != 100 || result.Message != "ok" {
		t.Fatalf("unexpected task result: %#v", result)
	}
	if result.Data == nil {
		t.Fatal("expected data to be preserved")
	}
}

func TestKeepalivedGenerateConfigUsesSafeDynamicVRIDAndAuth(t *testing.T) {
	setup := NewKeepalivedSetup()

	content, err := setup.GenerateConfig(KeepalivedConfig{
		VIP:          "10.0.0.100",
		VIPInterface: "eth0",
		Priority:     120,
		Role:         "MASTER",
		MySQLPort:    3306,
		VRID:         77,
		AuthPass:     "ka77pass",
	})

	if err != nil {
		t.Fatalf("GenerateConfig returned error: %v", err)
	}
	if !strings.Contains(content, "virtual_router_id 77") {
		t.Fatalf("expected dynamic virtual_router_id, got:\n%s", content)
	}
	if !strings.Contains(content, "auth_pass ka77pass") {
		t.Fatalf("expected dynamic auth_pass, got:\n%s", content)
	}
}

func TestKeepalivedGenerateConfigRejectsUnsafeInterface(t *testing.T) {
	setup := NewKeepalivedSetup()

	_, err := setup.GenerateConfig(KeepalivedConfig{
		VIP:          "10.0.0.100",
		VIPInterface: "eth0\nscript",
		Priority:     100,
		MySQLPort:    3306,
	})

	if err == nil {
		t.Fatal("expected unsafe interface to be rejected")
	}
}

func TestKeepalivedWriteConfigUsesFileAPI(t *testing.T) {
	setup := NewKeepalivedSetup()
	originalPath := keepalivedConfigPath
	keepalivedConfigPath = filepath.Join(t.TempDir(), "keepalived.conf")
	t.Cleanup(func() { keepalivedConfigPath = originalPath })

	err := setup.WriteConfig(context.Background(), "safe config")

	if err != nil {
		t.Fatalf("WriteConfig returned error: %v", err)
	}
	content, err := os.ReadFile(keepalivedConfigPath)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if string(content) != "safe config" {
		t.Fatalf("unexpected config content: %q", string(content))
	}
}

func TestProxySQLAdminCommandUsesAdminAuthAndPort(t *testing.T) {
	cfg := ProxySQLConfig{AdminHost: "127.0.0.1", AdminPort: 6032, AdminUser: "admin", AdminPass: "secret"}

	cmd := proxySQLAdminCommand(context.Background(), cfg, "SELECT 1")

	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "-u admin") || !strings.Contains(args, "-psecret") || !strings.Contains(args, "-P 6032") {
		t.Fatalf("expected admin auth args, got %v", cmd.Args)
	}
}

func TestProxySQLConfigRejectsUnsafeBackendHost(t *testing.T) {
	cfg := ProxySQLConfig{
		ProxyPort: 6033,
		Backends: []ProxySQLBackendConfig{{Host: "10.0.0.1';DROP", Port: 3306}},
	}
	normalizeProxySQLConfig(&cfg)

	if err := validateProxySQLConfig(cfg); err == nil {
		t.Fatal("expected invalid backend host to be rejected")
	}
}
