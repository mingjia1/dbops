package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGBase8aDeploysOnlyWithApprovedManifestAndCommands(t *testing.T) {
	root := t.TempDir()
	writeGBase8aBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })

	result, err := executor.Execute(context.Background(), gbase8aTestRequest(root))
	if err != nil || result.Status != "completed" ||
		!hasDamengCommand(commands, "SetSysEnv.py", "--dbaUser=gbase --installPrefix=/opt/dbops/gbase8a") ||
		!hasDamengCommand(commands, "gcinstall.py", "--license_file="+filepath.Join(root, "gbase8a", "10.1", "license.dat")+" --silent="+filepath.Join(root, "gbase8a", "10.1", "demo.options")+" --passwordInputMode") ||
		!hasDamengCommand(commands, "gcadmin", "distribution "+filepath.Join(root, "gbase8a", "10.1", "gcChangeInfo.xml")+" p 1 d 0 pattern 1") ||
		!hasDamengCommand(commands, "gccli", "-ugbase SET GLOBAL gcluster_rebalancing_concurrent_count = 4;") {
		t.Fatalf("result = %#v, commands = %#v, err = %v", result, commands, err)
	}
}

func TestGBase8aConfigureUsesStrictGlobalAllowlist(t *testing.T) {
	root := t.TempDir()
	writeGBase8aBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	req := gbase8aTestRequest(root)
	req.Operation = "configure"
	req.GBase8a.PasswordInputMode = false

	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" || len(commands) != 1 || !hasDamengCommand(commands, "gccli", "-ugbase SET GLOBAL gcluster_rebalancing_concurrent_count = 4;") {
		t.Fatalf("result = %#v, commands = %#v, err = %v", result, commands, err)
	}
}

func TestGBase8aRejectsUnsafeInputsTLSAndUnsupportedOperations(t *testing.T) {
	root := t.TempDir()
	writeGBase8aBundle(t, root, false)
	var calls int
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) {
		calls++
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	if result, err := executor.Execute(context.Background(), gbase8aTestRequest(root)); err != nil || result.Status != "failed" || !strings.Contains(result.Message, "gcChangeInfo.xml") {
		t.Fatalf("incomplete bundle result = %#v, err = %v", result, err)
	}
	writeGBase8aBundle(t, root, true)
	for _, mutate := range []func(*FlavorTaskRequest){
		func(req *FlavorTaskRequest) { req.GBase8a.InstallPrefix = "/opt/dbops/other" },
		func(req *FlavorTaskRequest) { req.GBase8a.Parameters = map[string]string{"gcadmin setparameter": "4"} },
		func(req *FlavorTaskRequest) {
			req.GBase8a.Parameters = map[string]string{"gcluster_rebalancing_concurrent_count": "65"}
		},
		func(req *FlavorTaskRequest) {
			req.TLS = &FlavorTLSConfig{Enabled: true, CAFile: "/tmp/ca.pem", CertFile: "/tmp/cert.pem", KeyFile: "/tmp/key.pem"}
		},
		func(req *FlavorTaskRequest) { req.Operation = "backup" },
	} {
		req := gbase8aTestRequest(root)
		mutate(&req)
		if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" {
			t.Fatalf("unsafe request result = %#v, err = %v", result, err)
		}
	}
	if calls != 0 {
		t.Fatalf("unsafe GBase 8a requests issued %d commands", calls)
	}
}

func TestGBase8aTLSConfigContentTargetsGBasedSection(t *testing.T) {
	tls := &FlavorTLSConfig{CAFile: "/opt/dbops/gbase8a/certs/ca.pem", CertFile: "/opt/dbops/gbase8a/certs/cert.pem", KeyFile: "/opt/dbops/gbase8a/certs/key.pem"}
	content := gbase8aTLSConfigContent("[gcluster]\nname=demo\n[gbased]\nssl-ca=old\nother=value\n[other]\nkey=value\n", tls)
	if !strings.Contains(content, "[gbased]\nother=value\nssl-ca=/opt/dbops/gbase8a/certs/ca.pem\nssl-cert=/opt/dbops/gbase8a/certs/cert.pem\nssl-key=/opt/dbops/gbase8a/certs/key.pem") || strings.Contains(content, "ssl-ca=old") {
		t.Fatalf("TLS configuration content = %q", content)
	}
}

func TestGBase8aBackupRestoreAndMigrationUseControlledCommands(t *testing.T) {
	root := t.TempDir()
	writeGBase8aBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })

	backup := gbase8aTestRequest(root)
	backup.Operation = "backup"
	backup.GBase8a.Backup = &GBase8aBackupConfig{Destination: "/opt/dbops/backups/gbase8a/full-001"}
	if result, err := executor.Execute(context.Background(), backup); err != nil || result.Status != "completed" ||
		!hasDamengCommand(commands, "gcadmin", "switchmode readonly") ||
		!hasDamengCommand(commands, "python", "/opt/dbops/gbase8a/server/bin/gcrcman.py -d /opt/dbops/backups/gbase8a/full-001 -P gbasedba -e backup level 0") ||
		!hasDamengCommand(commands, "gcadmin", "switchmode normal") {
		t.Fatalf("backup result = %#v, commands = %#v, err = %v", result, commands, err)
	}

	commands = nil
	restore := gbase8aTestRequest(root)
	restore.Operation = "restore"
	restore.GBase8a.Restore = &GBase8aRestoreConfig{BackupSource: "/opt/dbops/backups/gbase8a/full-001", Objects: []GBase8aRestoreObject{{Database: "sales", Table: "orders"}}}
	if result, err := executor.Execute(context.Background(), restore); err != nil || result.Status != "completed" ||
		!hasDamengCommand(commands, "gcadmin", "switchmode recovery") ||
		!hasDamengCommand(commands, "python", "/opt/dbops/gbase8a/server/bin/gcrcman.py -d /opt/dbops/backups/gbase8a/full-001 -P gbasedba -e recover table `sales`.`orders`") ||
		!hasDamengCommand(commands, "gcadmin", "switchmode normal") {
		t.Fatalf("restore result = %#v, commands = %#v, err = %v", result, commands, err)
	}

	commands = nil
	migration := gbase8aTestRequest(root)
	migration.Operation = "migrate"
	migration.GBase8a.Migration = &GBase8aMigrationConfig{SourceURI: "file:///opt/dbops/backups/gbase8a/import/orders.csv", Database: "sales", Table: "orders"}
	if result, err := executor.Execute(context.Background(), migration); err != nil || result.Status != "completed" ||
		!hasDamengCommand(commands, "gccli", "-ugbase LOAD DATA INFILE '/opt/dbops/backups/gbase8a/import/orders.csv' INTO TABLE `sales`.`orders`") {
		t.Fatalf("migration result = %#v, commands = %#v, err = %v", result, commands, err)
	}
}

func TestGBase8aLifecycleFailureReturnsNormalModeAndRejectsUnsafeInputs(t *testing.T) {
	root := t.TempDir()
	writeGBase8aBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		if name == "python" && len(args) > 0 && strings.HasSuffix(args[0], "gcrcman.py") {
			return "", fmt.Errorf("backup failed")
		}
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	backup := gbase8aTestRequest(root)
	backup.Operation = "backup"
	backup.GBase8a.Backup = &GBase8aBackupConfig{Destination: "/opt/dbops/backups/gbase8a/full-001"}
	if result, err := executor.Execute(context.Background(), backup); err != nil || result.Status != "failed" || !hasDamengCommand(commands, "gcadmin", "switchmode normal") {
		t.Fatalf("failure result = %#v, commands = %#v, err = %v", result, commands, err)
	}

	for _, mutate := range []func(*FlavorTaskRequest){
		func(req *FlavorTaskRequest) {
			req.Operation = "backup"
			req.GBase8a.Backup = &GBase8aBackupConfig{Destination: "/opt/dbops/backups/gbase8a/../escape"}
		},
		func(req *FlavorTaskRequest) {
			req.Operation = "restore"
			req.GBase8a.Restore = &GBase8aRestoreConfig{BackupSource: "/opt/dbops/backups/gbase8a/full-001"}
		},
		func(req *FlavorTaskRequest) {
			req.Operation = "migrate"
			req.GBase8a.Migration = &GBase8aMigrationConfig{SourceURI: "https://example.test/orders.csv", Database: "sales", Table: "orders"}
		},
		func(req *FlavorTaskRequest) {
			req.Operation = "migrate"
			req.GBase8a.Migration = &GBase8aMigrationConfig{SourceURI: "file:///opt/dbops/backups/gbase8a/orders.csv", Database: "sales;drop", Table: "orders"}
		},
		func(req *FlavorTaskRequest) { req.Operation = "upgrade" },
		func(req *FlavorTaskRequest) { req.Operation = "rollback" },
	} {
		commands = nil
		req := gbase8aTestRequest(root)
		mutate(&req)
		result, err := executor.Execute(context.Background(), req)
		if err != nil || result.Status != "failed" || len(commands) != 0 {
			t.Fatalf("unsafe result = %#v, commands = %#v, err = %v", result, commands, err)
		}
		if (req.Operation == "upgrade" || req.Operation == "rollback") && !strings.Contains(result.Message, "complex cluster topology and version prerequisites") {
			t.Fatalf("missing upgrade boundary in %q", result.Message)
		}
	}
}

func TestGBase8aMonitorsWithFixedStatusProcessAndCapacityCommands(t *testing.T) {
	root := t.TempDir()
	writeGBase8aBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	req := gbase8aTestRequest(root)
	req.Operation = "monitor"

	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" ||
		!hasDamengCommand(commands, "gcadmin", "status") ||
		!hasDamengCommand(commands, "ps", "-C gnode -C gcluster -C gcware -o pid=,stat=,args=") ||
		!hasDamengCommand(commands, "df", "-P /opt/dbops/gbase8a") {
		t.Fatalf("result = %#v, commands = %#v, err = %v", result, commands, err)
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("monitor data = %#v", result.Data)
	}
	for _, key := range []string{"gcadmin_status", "core_processes", "installation_capacity"} {
		if data[key] != "ok" {
			t.Fatalf("monitor data[%q] = %#v", key, data[key])
		}
	}
}

func TestGBase8aTeardownRequiresConfirmationFinalBackupAndVerifiedUninstall(t *testing.T) {
	root := t.TempDir()
	writeGBase8aBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	req := gbase8aTestRequest(root)
	req.Operation = "teardown"
	if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" || !strings.Contains(result.Message, "confirm_uninstall") {
		t.Fatalf("unconfirmed teardown result = %#v, err = %v", result, err)
	}

	req.GBase8a.ConfirmUninstall = true
	req.GBase8a.Backup = &GBase8aBackupConfig{Destination: "/opt/dbops/backups/gbase8a/final"}
	if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" || !strings.Contains(result.Message, "official GBase 8a V9 uninstall procedure") {
		t.Fatalf("unverified teardown result = %#v, err = %v", result, err)
	}
	if len(commands) != 0 {
		t.Fatalf("teardown ran commands without a verified uninstall procedure: %#v", commands)
	}
}

func TestGBase8aRejectsDistributedLifecycleOperations(t *testing.T) {
	root := t.TempDir()
	writeGBase8aBundle(t, root, true)
	var calls int
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) {
		calls++
		return "ok", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	for _, operation := range []string{"ha", "replication", "failover", "scale", "rebuild"} {
		req := gbase8aTestRequest(root)
		req.Operation = operation
		result, err := executor.Execute(context.Background(), req)
		if err != nil || result.Status != "failed" || !strings.Contains(result.Message, "single-node executor does not support") {
			t.Fatalf("%s result = %#v, err = %v", operation, result, err)
		}
	}
	if calls != 0 {
		t.Fatalf("distributed operations issued %d commands", calls)
	}
}

func gbase8aTestRequest(root string) FlavorTaskRequest {
	return FlavorTaskRequest{TaskID: "gbase8a-deploy", InstanceID: "gbase8a-1", Flavor: "gbase8a", Version: "10.1", Operation: "deploy", PackagePath: filepath.Join(root, "gbase8a", "10.1"), TLS: &FlavorTLSConfig{}, GBase8a: &GBase8aConfig{DBAUser: "gbase", InstallPrefix: "/opt/dbops/gbase8a", PasswordInputMode: true, Parameters: map[string]string{"gcluster_rebalancing_concurrent_count": "4"}}}
}

func writeGBase8aBundle(t *testing.T, root string, complete bool) {
	t.Helper()
	dir := filepath.Join(root, "gbase8a", "10.1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	artifacts := []string{"SetSysEnv.py", "gcinstall.py", "demo.options", "gcChangeInfo.xml"}
	if !complete {
		artifacts = artifacts[:3]
	}
	entries := make([]string, 0, len(artifacts))
	for _, name := range artifacts {
		contents := []byte(name)
		if err := os.WriteFile(filepath.Join(dir, name), contents, 0o644); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(contents)
		entries = append(entries, `{"file":"`+name+`","sha256":"`+hex.EncodeToString(hash[:])+`"}`)
	}
	if err := os.WriteFile(filepath.Join(dir, "license.dat"), []byte("licensed"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := `{"flavor":"gbase8a","version":"10.1","requires_license":true,"license_file":"license.dat","packages":[` + strings.Join(entries, ",") + `]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}
