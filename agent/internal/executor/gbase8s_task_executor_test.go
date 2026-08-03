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

func TestGBase8sDeploysAndVerifiesDocumentedSingleNodeWorkflow(t *testing.T) {
	root := t.TempDir()
	writeGBase8sBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	var inputs []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.inputRunner = func(_ context.Context, input, name string, args ...string) (string, error) {
		inputs = append(inputs, recordedOceanBaseCommand{name, append([]string{input}, args...)})
		return "ok", nil
	}

	result, err := executor.Execute(context.Background(), gbase8sTestRequest(root))
	if err != nil || result.Status != "completed" || !hasDamengCommand(commands, "sh", "cd \"$1/PluginPak\" && sh ./install_init.sh") || !hasDamengCommand(commands, "oninit", "-vy") || !hasDamengCommand(commands, "onmode", "-wf BUFFERS=2048") || !hasDamengCommand(commands, "onmode", "-wm DYNAMIC_LOGS=1") || !hasDamengCommand(commands, "onstat", "-c") || !hasDamengCommand(commands, "onstat", "-g cfg") || !hasDamengCommand(inputs, "dbaccess", "CREATE USER app_user") || !hasDamengCommand(inputs, "dbaccess", "DBINFO('version', 'full')") {
		t.Fatalf("result = %#v, commands = %#v, inputs = %#v, err = %v", result, commands, inputs, err)
	}
}

func TestGBase8sConfigureUsesPersistentAndMemoryModes(t *testing.T) {
	root := t.TempDir()
	writeGBase8sBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	var inputs []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.inputRunner = func(_ context.Context, input, name string, args ...string) (string, error) {
		inputs = append(inputs, recordedOceanBaseCommand{name, append([]string{input}, args...)})
		return "ok", nil
	}
	req := gbase8sTestRequest(root)
	req.Operation = "configure"

	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" || !hasDamengCommand(commands, "onmode", "-wf BUFFERS=2048") || !hasDamengCommand(commands, "onmode", "-wm DYNAMIC_LOGS=1") || !hasDamengCommand(commands, "onstat", "-c") || !hasDamengCommand(commands, "onstat", "-g cfg") || !hasDamengCommand(inputs, "dbaccess", "DBINFO('version', 'full')") {
		t.Fatalf("result = %#v, commands = %#v, inputs = %#v, err = %v", result, commands, inputs, err)
	}
}

func TestGBase8sRejectsTLSUnsafeInputAndUnverifiedInstaller(t *testing.T) {
	root := t.TempDir()
	writeGBase8sBundle(t, root, false)
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.inputRunner = func(_ context.Context, _ string, _ string, _ ...string) (string, error) { return "", nil }
	if result, err := executor.Execute(context.Background(), gbase8sTestRequest(root)); err != nil || result.Status != "failed" || !strings.Contains(result.Message, "PluginPak/install_init.sh") {
		t.Fatalf("missing installer result = %#v, err = %v", result, err)
	}

	writeGBase8sBundle(t, root, true)
	for _, mutate := range []func(*FlavorTaskRequest){
		func(req *FlavorTaskRequest) { req.TLS.Enabled = true },
		func(req *FlavorTaskRequest) { req.GBase8s.InstallDir = "/opt/GBase" },
		func(req *FlavorTaskRequest) {
			req.GBase8s.PersistentParameters = map[string]string{"BUFFERS": "1; onmode -ky"}
		},
		func(req *FlavorTaskRequest) { req.GBase8s.MemoryParameters = map[string]string{"UNSAFE": "1"} },
	} {
		req := gbase8sTestRequest(root)
		mutate(&req)
		if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" {
			t.Fatalf("unsafe request result = %#v, err = %v", result, err)
		}
	}
}

func TestGBase8sRejectsOtherLifecycleOperations(t *testing.T) {
	root := t.TempDir()
	writeGBase8sBundle(t, root, true)
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	for _, operation := range []string{"backup", "restore", "migrate", "upgrade", "monitor", "teardown", "ha", "replication", "failover", "scale"} {
		req := gbase8sTestRequest(root)
		req.Operation = operation
		if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" || !strings.Contains(result.Message, "only supports deploy and configure") {
			t.Fatalf("%s result = %#v, err = %v", operation, result, err)
		}
	}
}

func gbase8sTestRequest(root string) FlavorTaskRequest {
	return FlavorTaskRequest{
		TaskID: "gbase8s-deploy", InstanceID: "gbase8s-1", Flavor: "gbase8s", Version: "8.8", Operation: "deploy", PackagePath: filepath.Join(root, "gbase8s", "8.8"), TLS: &FlavorTLSConfig{},
		GBase8s: &GBase8sConfig{InstallDir: "/opt/dbops/gbase8s/install", ApplicationUser: "app_user", ApplicationPassword: "GBasePass1234", PersistentParameters: map[string]string{"BUFFERS": "2048"}, MemoryParameters: map[string]string{"DYNAMIC_LOGS": "1"}},
	}
}

func writeGBase8sBundle(t *testing.T, root string, installer bool) {
	t.Helper()
	dir := filepath.Join(root, "gbase8s", "8.8")
	if err := os.MkdirAll(filepath.Join(dir, "PluginPak"), 0o755); err != nil {
		t.Fatal(err)
	}
	name := "other.bin"
	if installer {
		name = "PluginPak/install_init.sh"
	}
	contents := []byte("gbase8s installer")
	if err := os.WriteFile(filepath.Join(dir, name), contents, 0o755); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(contents)
	manifest := `{"flavor":"gbase8s","version":"8.8","packages":[{"file":"` + name + `","sha256":"` + hex.EncodeToString(hash[:]) + `"}]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}
