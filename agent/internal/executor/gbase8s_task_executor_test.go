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
	for _, operation := range []string{"upgrade", "monitor", "teardown", "ha", "replication", "failover", "scale"} {
		req := gbase8sTestRequest(root)
		req.Operation = operation
		if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" {
			t.Fatalf("%s result = %#v, err = %v", operation, result, err)
		}
	}
}

func TestGBase8sBackupRestoreAndMigrationUseControlledDirectories(t *testing.T) {
	root := t.TempDir()
	writeGBase8sBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })

	backup := gbase8sTestRequest(root)
	backup.Operation = "backup"
	backup.GBase8s.Backup = &GBase8sBackupConfig{TapeDirectory: "/opt/dbops/backups/gbase8s/archive", LogicalLogDirectory: "/opt/dbops/backups/gbase8s/logs"}
	if result, err := executor.Execute(context.Background(), backup); err != nil || result.Status != "completed" || !hasDamengCommand(commands, "env", "TAPEDEV=/opt/dbops/backups/gbase8s/archive LTAPEDEV=/opt/dbops/backups/gbase8s/logs /opt/dbops/gbase8s/install/bin/ontape -s -L 0 -d") || !hasDamengCommand(commands, "env", "TAPEDEV=/opt/dbops/backups/gbase8s/archive LTAPEDEV=/opt/dbops/backups/gbase8s/logs /opt/dbops/gbase8s/install/bin/ontape -a") {
		t.Fatalf("backup result = %#v, commands = %#v, err = %v", result, commands, err)
	}

	restore := gbase8sTestRequest(root)
	restore.Operation = "restore"
	restore.GBase8s.Restore = &GBase8sRestoreConfig{TapeDirectory: "/opt/dbops/backups/gbase8s/archive", LogicalLogDirectory: "/opt/dbops/backups/gbase8s/logs", ReplayLogicalLogs: true}
	if result, err := executor.Execute(context.Background(), restore); err != nil || result.Status != "completed" || !hasDamengCommand(commands, "env", "ontape -r") || !hasDamengCommand(commands, "env", "ontape -l") {
		t.Fatalf("restore result = %#v, commands = %#v, err = %v", result, commands, err)
	}

	migration := gbase8sTestRequest(root)
	migration.Operation = "migrate"
	migration.GBase8s.Migration = &GBase8sMigrationConfig{SourceDatabase: "source_db", TargetDatabase: "target_db", ExportDirectory: "/opt/dbops/backups/gbase8s/export", ImportDirectory: "/opt/dbops/backups/gbase8s/import"}
	if result, err := executor.Execute(context.Background(), migration); err != nil || result.Status != "completed" || !hasDamengCommand(commands, "dbexport", "-o /opt/dbops/backups/gbase8s/export source_db") || !hasDamengCommand(commands, "dbimport", "-c -i /opt/dbops/backups/gbase8s/import target_db") {
		t.Fatalf("migration result = %#v, commands = %#v, err = %v", result, commands, err)
	}
}

func TestGBase8sRejectsBackupMigrationBoundaryAndExternalUpgrade(t *testing.T) {
	root := t.TempDir()
	writeGBase8sBundle(t, root, true)
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	for _, mutate := range []func(*FlavorTaskRequest){
		func(req *FlavorTaskRequest) {
			req.Operation = "backup"
			req.GBase8s.Backup = &GBase8sBackupConfig{TapeDirectory: "/opt/dbops/backups/gbase8s/archive", LogicalLogDirectory: "/tmp/logs"}
		},
		func(req *FlavorTaskRequest) {
			req.Operation = "restore"
			req.GBase8s.Restore = &GBase8sRestoreConfig{TapeDirectory: "/opt/dbops/backups/gbase8s/archive", LogicalLogDirectory: "/opt/dbops/backups/gbase8s/archive"}
		},
		func(req *FlavorTaskRequest) {
			req.Operation = "migrate"
			req.GBase8s.Migration = &GBase8sMigrationConfig{SourceDatabase: "source_db", TargetDatabase: "target_db", ExportDirectory: "/opt/dbops/backups/gbase8s/../outside", ImportDirectory: "/opt/dbops/backups/gbase8s/import"}
		},
	} {
		req := gbase8sTestRequest(root)
		mutate(&req)
		if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" {
			t.Fatalf("unsafe request result = %#v, err = %v", result, err)
		}
	}

	req := gbase8sTestRequest(root)
	req.Operation = "upgrade"
	if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" || !strings.Contains(result.Message, "ON-Bar") {
		t.Fatalf("upgrade result = %#v, err = %v", result, err)
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
