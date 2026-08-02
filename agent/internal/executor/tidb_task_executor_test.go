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

func TestTiDBDeploysOfflineHybridTopology(t *testing.T) {
	root := t.TempDir()
	writeTiDBBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		if strings.Contains(strings.Join(args, " "), "cluster start") {
			return "The new password is: 'generated-password'", nil
		}
		if strings.Contains(strings.Join(args, " "), "cluster display") {
			return "tidb-server Up\ntikv Up\npd Up", nil
		}
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.tidbControlRoot = t.TempDir()
	req := tidbTestRequest(root)
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	if !hasOceanBaseCommand(commands, "tar", "tidb-community-server-v8.5.7-linux-amd64.tar.gz") || !hasOceanBaseCommand(commands, "sh", "local_install.sh") || !hasTiDBCommand(commands, "cluster deploy") || !hasTiDBCommand(commands, "cluster start") || !hasOceanBaseCommand(commands, "mysql", "SELECT @@version") {
		t.Fatalf("deployment commands missing: %#v", commands)
	}
	topology, err := os.ReadFile(filepath.Join(executor.tidbControlRoot, "tidb_single", "v8.5.7", "topology.yaml"))
	if err != nil || !strings.Contains(string(topology), "pd_servers:") || !strings.Contains(string(topology), "log.level: \"info\"") {
		t.Fatalf("topology = %q, err = %v", topology, err)
	}
}

func TestTiDBDeployIncludesTLSConfiguration(t *testing.T) {
	root := t.TempDir()
	writeTiDBBundle(t, root, true)
	certDir := t.TempDir()
	for _, name := range []string{"ca.pem", "cert.pem", "key.pem"} {
		if err := os.WriteFile(filepath.Join(certDir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "cluster start") {
			return "The new password is: 'generated-password'", nil
		}
		if strings.Contains(strings.Join(args, " "), "cluster display") {
			return "Up", nil
		}
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.tidbControlRoot = t.TempDir()
	req := tidbTestRequest(root)
	req.TLS = &FlavorTLSConfig{Enabled: true, CAFile: filepath.Join(certDir, "ca.pem"), CertFile: filepath.Join(certDir, "cert.pem"), KeyFile: filepath.Join(certDir, "key.pem")}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	topology, err := os.ReadFile(filepath.Join(executor.tidbControlRoot, "tidb_single", "v8.5.7", "topology.yaml"))
	if err != nil || !strings.Contains(string(topology), "security.cluster-ssl-ca") || !strings.Contains(string(topology), "cacert-path") || !strings.Contains(string(topology), "ca-path") {
		t.Fatalf("topology = %q, err = %v", topology, err)
	}
}

func TestTiDBConfigureReloadsComponentConfiguration(t *testing.T) {
	root := t.TempDir()
	writeTiDBBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.tidbControlRoot = t.TempDir()
	req := tidbTestRequest(root)
	req.Operation = "configure"
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" || !hasTiDBCommand(commands, "cluster reload") {
		t.Fatalf("result = %#v, commands = %#v, err = %v", result, commands, err)
	}
}

func TestTiDBRejectsBundleWithoutToolkitArchive(t *testing.T) {
	root := t.TempDir()
	writeTiDBBundle(t, root, false)
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.tidbControlRoot = t.TempDir()
	result, err := executor.Execute(context.Background(), tidbTestRequest(root))
	if err != nil || result.Status != "failed" || !strings.Contains(result.Message, "server and toolkit") {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

func TestTiDBBackupStartsBRAndLogBackup(t *testing.T) {
	root := t.TempDir()
	writeTiDBBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	req := tidbTestRequest(root)
	req.Operation = "backup"
	req.TiDB.Backup = &TiDBBackupConfig{Destination: "local:///opt/dbops/backups/tidb/full", LogDestination: "local:///opt/dbops/backups/tidb/log"}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" || !hasTiDBCommand(commands, "br backup full") || !hasTiDBCommand(commands, "br log start") || !hasTiDBCommand(commands, "br log status") {
		t.Fatalf("result = %#v, commands = %#v, err = %v", result, commands, err)
	}
}

func TestTiDBRestoreUsesPITRAndValidatesData(t *testing.T) {
	root := t.TempDir()
	writeTiDBBundle(t, root, true)
	checksum := strings.Repeat("a", 64)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		if strings.Contains(strings.Join(args, " "), "COUNT(*)") {
			return "3", nil
		}
		if strings.Contains(strings.Join(args, " "), "CHECKSUM TABLE") {
			return checksum, nil
		}
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	req := tidbTestRequest(root)
	req.Operation = "restore"
	req.TiDB.Restore = &TiDBRestoreConfig{BackupSource: "local:///opt/dbops/backups/tidb/full", LogBackupSource: "local:///opt/dbops/backups/tidb/log", RestoredTS: "2026-08-02 10:00:00+0800", ValidationTables: []TiDBTableCheck{{Database: "app", Table: "orders", ExpectedRowCount: 3, ExpectedChecksum: checksum}}}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" || !hasTiDBCommand(commands, "br restore point") || !hasOceanBaseCommand(commands, "mysql", "CHECKSUM TABLE") {
		t.Fatalf("result = %#v, commands = %#v, err = %v", result, commands, err)
	}
}

func TestTiDBMigrationAndUpgradeUseApprovedPaths(t *testing.T) {
	root := t.TempDir()
	writeTiDBBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		if strings.Contains(strings.Join(args, " "), "cluster display") {
			return "Up", nil
		}
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.tidbControlRoot = t.TempDir()
	migration := tidbTestRequest(root)
	migration.Operation = "migrate"
	migration.TiDB.Migration = &TiDBMigrationConfig{DataSourceDir: "/opt/dbops/backups/tidb/export", SortedKVDir: "/opt/dbops/backups/tidb/sorted"}
	if result, err := executor.Execute(context.Background(), migration); err != nil || result.Status != "completed" || !hasTiDBCommand(commands, "dumpling") || !hasTiDBCommand(commands, "tidb-lightning") {
		t.Fatalf("migration result = %#v, commands = %#v, err = %v", result, commands, err)
	} else if data, ok := result.Data.(map[string]interface{}); !ok || data["dumpling_output"] != "ok" || data["lightning_output"] != "ok" {
		t.Fatalf("migration data = %#v", result.Data)
	}
	upgrade := tidbTestRequest(root)
	upgrade.Operation = "upgrade"
	upgrade.TiDB.UpgradeVersion = "v8.5.8"
	upgrade.TiDB.Backup = &TiDBBackupConfig{Destination: "local:///opt/dbops/backups/tidb/pre-upgrade"}
	if result, err := executor.Execute(context.Background(), upgrade); err != nil || result.Status != "completed" || !hasTiDBCommand(commands, "cluster upgrade") {
		t.Fatalf("upgrade result = %#v, commands = %#v, err = %v", result, commands, err)
	}
}

func TestTiDBMonitorTeardownAndSingleHostBoundaries(t *testing.T) {
	root := t.TempDir()
	writeTiDBBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		if strings.Contains(strings.Join(args, " "), "cluster display") {
			return "pd Up\ntikv Up\ntidb Up", nil
		}
		if strings.Contains(strings.Join(args, " "), "SELECT @@version") {
			return "8.5.7", nil
		}
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	monitor := tidbTestRequest(root)
	monitor.Operation = "monitor"
	if result, err := executor.Execute(context.Background(), monitor); err != nil || result.Status != "completed" || !hasTiDBCommand(commands, "cluster display") {
		t.Fatalf("monitor result = %#v, commands = %#v, err = %v", result, commands, err)
	}
	for _, operation := range []string{"ha", "replication", "failover", "scale"} {
		req := tidbTestRequest(root)
		req.Operation = operation
		if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" || !strings.Contains(result.Message, "single-host") {
			t.Fatalf("%s result = %#v, err = %v", operation, result, err)
		}
	}
	teardown := tidbTestRequest(root)
	teardown.Operation = "teardown"
	teardown.TiDB.Backup = &TiDBBackupConfig{Destination: "local:///opt/dbops/backups/tidb/final"}
	if result, err := executor.Execute(context.Background(), teardown); err != nil || result.Status != "failed" || !strings.Contains(result.Message, "confirm_uninstall") {
		t.Fatalf("unconfirmed teardown result = %#v, err = %v", result, err)
	}
	teardown.TiDB.ConfirmUninstall = true
	if result, err := executor.Execute(context.Background(), teardown); err != nil || result.Status != "completed" || !hasTiDBCommand(commands, "cluster destroy") {
		t.Fatalf("teardown result = %#v, commands = %#v, err = %v", result, commands, err)
	}
}

func tidbTestRequest(root string) FlavorTaskRequest {
	return FlavorTaskRequest{
		TaskID: "tidb-deploy", InstanceID: "tidb-1", Flavor: "tidb", Version: "v8.5.7", Operation: "deploy",
		PackagePath: filepath.Join(root, "tidb", "v8.5.7"), TLS: &FlavorTLSConfig{},
		TiDB: &TiDBConfig{ClusterName: "tidb_single", Address: "10.0.0.9", Architecture: "amd64", DeployUser: "tidb", RootPassword: "tidb-root-pass", Parameters: map[string]string{"tidb.log.level": "info"}},
	}
}

func writeTiDBBundle(t *testing.T, root string, toolkit bool) {
	t.Helper()
	dir := filepath.Join(root, "tidb", "v8.5.7")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := []string{"tidb-community-server-v8.5.7-linux-amd64.tar.gz"}
	if toolkit {
		files = append(files, "tidb-community-toolkit-v8.5.7-linux-amd64.tar.gz")
	}
	artifacts := make([]string, 0, len(files))
	for _, file := range files {
		contents := []byte(file)
		hash := sha256.Sum256(contents)
		if err := os.WriteFile(filepath.Join(dir, file), contents, 0o644); err != nil {
			t.Fatal(err)
		}
		artifacts = append(artifacts, `{"file":"`+file+`","sha256":"`+hex.EncodeToString(hash[:])+`"}`)
	}
	manifest := `{"flavor":"tidb","version":"v8.5.7","packages":[` + strings.Join(artifacts, ",") + `]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasTiDBCommand(commands []recordedOceanBaseCommand, argument string) bool {
	for _, command := range commands {
		if strings.Contains(command.name, ".tiup/bin/tiup") && strings.Contains(strings.Join(command.args, " "), argument) {
			return true
		}
	}
	return false
}
