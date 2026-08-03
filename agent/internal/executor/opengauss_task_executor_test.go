package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenGaussDeploysSingleNodeAndCreatesApplicationUser(t *testing.T) {
	root := t.TempDir()
	writeOpenGaussBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	var input recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "openGauss 8.1", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.inputRunner = func(_ context.Context, value, name string, args ...string) (string, error) {
		input = recordedOceanBaseCommand{name, append([]string{value}, args...)}
		return "installed", nil
	}
	result, err := executor.Execute(context.Background(), openGaussTestRequest(root))
	if err != nil || result.Status != "completed" || input.name != "sh" || !strings.Contains(strings.Join(input.args, " "), "install.sh --mode single -D /opt/dbops/opengauss/data -R /opt/dbops/opengauss/install --start") || !hasDamengCommand(commands, "gsql", "CREATE USER app_user") || !hasDamengCommand(commands, "gsql", "GRANT CONNECT ON DATABASE postgres TO app_user") || !hasDamengCommand(commands, "gsql", "SELECT version()") || !hasDamengCommand(commands, "gs_ctl", "restart -D") {
		t.Fatalf("result = %#v, input = %#v, commands = %#v, err = %v", result, input, commands, err)
	}
}

func TestOpenGaussRejectsMismatchedVersion(t *testing.T) {
	root := t.TempDir()
	writeOpenGaussBundle(t, root, true)
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "openGauss 7.0", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.inputRunner = func(_ context.Context, _ string, _ string, _ ...string) (string, error) { return "installed", nil }
	result, err := executor.Execute(context.Background(), openGaussTestRequest(root))
	if err != nil || result.Status != "failed" || !strings.Contains(result.Message, "does not match") {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

func TestOpenGaussTLSUsesOfficialGUCSettings(t *testing.T) {
	root := t.TempDir()
	writeOpenGaussBundle(t, root, true)
	certDir := t.TempDir()
	for _, name := range []string{"ca.pem", "cert.pem", "key.pem"} {
		if err := os.WriteFile(filepath.Join(certDir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.inputRunner = func(_ context.Context, _ string, _ string, _ ...string) (string, error) { return "ok", nil }
	req := openGaussTestRequest(root)
	req.Operation = "configure"
	req.TLS = &FlavorTLSConfig{Enabled: true, CAFile: filepath.Join(certDir, "ca.pem"), CertFile: filepath.Join(certDir, "cert.pem"), KeyFile: filepath.Join(certDir, "key.pem")}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" || !hasDamengCommand(commands, "gs_guc", "ssl=on") || !hasDamengCommand(commands, "gs_guc", "ssl_cert_file=") || !hasDamengCommand(commands, "gs_guc", "ssl_key_file=") || !hasDamengCommand(commands, "gs_guc", "ssl_ca_file=") || !hasDamengCommand(commands, "gs_ctl", "restart -D") {
		t.Fatalf("result = %#v, commands = %#v, err = %v", result, commands, err)
	}
}

func TestOpenGaussRejectsBundleAndUnsafeConfiguration(t *testing.T) {
	root := t.TempDir()
	writeOpenGaussBundle(t, root, false)
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.inputRunner = func(_ context.Context, _ string, _ string, _ ...string) (string, error) { return "", nil }
	if result, err := executor.Execute(context.Background(), openGaussTestRequest(root)); err != nil || result.Status != "failed" || !strings.Contains(result.Message, "install.sh") {
		t.Fatalf("missing installer result = %#v, err = %v", result, err)
	}
	writeOpenGaussBundle(t, root, true)
	for _, configure := range []func(*FlavorTaskRequest){
		func(req *FlavorTaskRequest) { req.OpenGauss.DataDir = "/tmp/opengauss" },
		func(req *FlavorTaskRequest) {
			req.OpenGauss.Parameters = map[string]string{"shared_buffers": "1GB; SELECT 1"}
		},
		func(req *FlavorTaskRequest) {
			req.TLS = &FlavorTLSConfig{Enabled: true, CAFile: "/missing/ca", CertFile: "/missing/cert", KeyFile: "/missing/key"}
		},
	} {
		req := openGaussTestRequest(root)
		configure(&req)
		if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" {
			t.Fatalf("unsafe request result = %#v, err = %v", result, err)
		}
	}
	certDir := t.TempDir()
	certFile := filepath.Join(certDir, "cert.pem")
	if err := os.WriteFile(certFile, []byte("certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(certDir, "cert-link.pem")
	if err := os.Symlink(certFile, link); err != nil {
		t.Fatal(err)
	}
	req := openGaussTestRequest(root)
	req.TLS = &FlavorTLSConfig{Enabled: true, CAFile: certFile, CertFile: link, KeyFile: certFile}
	if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" {
		t.Fatalf("symbolic-link TLS result = %#v, err = %v", result, err)
	}
}

func TestOpenGaussRejectsDeferredLifecycleAndHAOperations(t *testing.T) {
	root := t.TempDir()
	writeOpenGaussBundle(t, root, true)
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	for _, operation := range []string{"backup", "restore", "migrate", "upgrade", "monitor", "teardown", "ha", "replication", "failover"} {
		req := openGaussTestRequest(root)
		req.Operation = operation
		if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" || !strings.Contains(result.Message, "not executable") {
			t.Fatalf("%s result = %#v, err = %v", operation, result, err)
		}
	}
}

func openGaussTestRequest(root string) FlavorTaskRequest {
	return FlavorTaskRequest{TaskID: "opengauss-deploy", InstanceID: "opengauss-1", Flavor: "opengauss", Version: "8.1", Operation: "deploy", PackagePath: filepath.Join(root, "opengauss", "8.1"), TLS: &FlavorTLSConfig{}, OpenGauss: &OpenGaussConfig{Address: "10.0.0.10", Port: 5432, InstallDir: "/opt/dbops/opengauss/install", DataDir: "/opt/dbops/opengauss/data", AdminPassword: "AdminPass1234", ApplicationUser: "app_user", ApplicationPassword: "AppPass123456", Parameters: map[string]string{"max_connections": "200"}}}
}

func writeOpenGaussBundle(t *testing.T, root string, installer bool) {
	t.Helper()
	dir := filepath.Join(root, "opengauss", "8.1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := "bundle.tar.gz"
	if installer {
		file = "install.sh"
	}
	contents := []byte(file)
	if err := os.WriteFile(filepath.Join(dir, file), contents, 0o755); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(contents)
	manifest := `{"flavor":"opengauss","version":"8.1","packages":[{"file":"` + file + `","sha256":"` + hex.EncodeToString(hash[:]) + `"}]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}
