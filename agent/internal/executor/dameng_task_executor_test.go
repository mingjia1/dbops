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

func TestDamengDeploysOfflineInstanceAndVerifiesSQL(t *testing.T) {
	root := t.TempDir()
	writeDamengBundle(t, root)
	var commands []recordedOceanBaseCommand
	var starts []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "DM Database Server 9", nil
	}, func(_ context.Context, name string, args ...string) error {
		starts = append(starts, recordedOceanBaseCommand{name, args})
		return nil
	})
	req := damengTestRequest(root)
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" || !hasDamengCommand(commands, "DMInstall.bin", "-q") || !hasDamengCommand(commands, "dminit", "SYSDBA_PWD") || !hasDamengCommand(commands, "disql", "CREATE USER") || !hasDamengCommand(commands, "disql", "SELECT VERSION()") || len(starts) != 1 || !strings.Contains(starts[0].name, "dmserver") {
		t.Fatalf("result = %#v, commands = %#v, starts = %#v, err = %v", result, commands, starts, err)
	}
}

func TestDamengTLSUsesServerEncryptionSetting(t *testing.T) {
	root := t.TempDir()
	writeDamengBundle(t, root)
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
	req := damengTestRequest(root)
	req.TLS = &FlavorTLSConfig{Enabled: true, CAFile: filepath.Join(certDir, "ca.pem"), CertFile: filepath.Join(certDir, "cert.pem"), KeyFile: filepath.Join(certDir, "key.pem")}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" || !hasDamengCommand(commands, "disql", "ENABLE_ENCRYPT") {
		t.Fatalf("result = %#v, commands = %#v, err = %v", result, commands, err)
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok || data["restart_required"] != true {
		t.Fatalf("result data = %#v", result.Data)
	}
}

func TestDamengRejectsMissingInstaller(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "dm", "DM9")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := []byte("other package")
	if err := os.WriteFile(filepath.Join(dir, "other.bin"), contents, 0o644); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(contents)
	manifest := `{"flavor":"dm","version":"DM9","packages":[{"file":"other.bin","sha256":"` + hex.EncodeToString(hash[:]) + `"}]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	result, err := executor.Execute(context.Background(), damengTestRequest(root))
	if err != nil || result.Status != "failed" || !strings.Contains(result.Message, "DMInstall.bin") {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

func hasDamengCommand(commands []recordedOceanBaseCommand, name, argument string) bool {
	for _, command := range commands {
		if strings.Contains(command.name, name) && strings.Contains(strings.Join(command.args, " "), argument) {
			return true
		}
	}
	return false
}

func damengTestRequest(root string) FlavorTaskRequest {
	return FlavorTaskRequest{TaskID: "dm-deploy", InstanceID: "dm-1", Flavor: "dm", Version: "DM9", Operation: "deploy", PackagePath: filepath.Join(root, "dm", "DM9"), TLS: &FlavorTLSConfig{}, Dameng: &DamengConfig{Address: "10.0.0.8", Port: 5236, InstallDir: "/opt/dbops/dm/install", DataDir: "/opt/dbops/dm/data", SysdbaPassword: "DamengPass123", ApplicationUser: "app_user", ApplicationPassword: "AppPass123456", Parameters: map[string]string{"SVR_LOG": "1"}}}
}

func writeDamengBundle(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "dm", "DM9")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := []byte("dameng installer")
	if err := os.WriteFile(filepath.Join(dir, "DMInstall.bin"), contents, 0o755); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(contents)
	manifest := `{"flavor":"dm","version":"DM9","packages":[{"file":"DMInstall.bin","sha256":"` + hex.EncodeToString(hash[:]) + `"}]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}
