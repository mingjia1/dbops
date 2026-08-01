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

type recordedOceanBaseCommand struct {
	name string
	args []string
}

func TestOceanBaseDeployInstallsStartsAndCreatesTenant(t *testing.T) {
	root := t.TempDir()
	writeOceanBaseBundle(t, root)
	var commands, starts []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root,
		func(_ context.Context, name string, args ...string) (string, error) {
			commands = append(commands, recordedOceanBaseCommand{name, args})
			if name == oceanBaseOBClient && len(args) > 0 && strings.Contains(args[len(args)-1], "SELECT") {
				return "verified", nil
			}
			return "ok", nil
		},
		func(_ context.Context, name string, args ...string) error {
			starts = append(starts, recordedOceanBaseCommand{name, args})
			return nil
		},
	)
	req := oceanBaseTestRequest(root)
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	if len(starts) != 2 || starts[0].name != oceanBaseObserver || starts[1].name != oceanBaseOBProxy {
		t.Fatalf("starts = %#v", starts)
	}
	if !hasOceanBaseCommand(commands, "rpm", "--replacepkgs") {
		t.Fatalf("RPM installation command missing: %#v", commands)
	}
	if !hasOceanBaseSQL(commands, "CREATE RESOURCE UNIT tenant_a_unit") || !hasOceanBaseSQL(commands, "CREATE TENANT tenant_a") {
		t.Fatalf("tenant setup commands missing: %#v", commands)
	}
	if !hasOceanBaseSQL(commands, "ALTER SYSTEM SET enable_sql_audit") {
		t.Fatalf("parameter command missing: %#v", commands)
	}
	if !hasOceanBaseSQL(commands, "SELECT VERSION()") || !hasOceanBaseSQL(commands, "SHOW OB SERVERS") || !hasOceanBaseSQL(commands, "SHOW PARAMETERS LIKE 'enable_sql_audit'") {
		t.Fatalf("deployment verification commands missing: %#v", commands)
	}
}

func TestOceanBaseDeployConfiguresTLS(t *testing.T) {
	root := t.TempDir()
	writeOceanBaseBundle(t, root)
	certDir := t.TempDir()
	for _, name := range []string{"ca.pem", "cert.pem", "key.pem"} {
		if err := os.WriteFile(filepath.Join(certDir, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root,
		func(_ context.Context, name string, args ...string) (string, error) {
			commands = append(commands, recordedOceanBaseCommand{name, args})
			if name == oceanBaseOBClient && len(args) > 0 && strings.Contains(args[len(args)-1], "SELECT") {
				return "verified", nil
			}
			return "ok", nil
		},
		func(_ context.Context, _ string, _ ...string) error { return nil },
	)
	req := oceanBaseTestRequest(root)
	req.TLS = &FlavorTLSConfig{Enabled: true, CAFile: filepath.Join(certDir, "ca.pem"), CertFile: filepath.Join(certDir, "cert.pem"), KeyFile: filepath.Join(certDir, "key.pem")}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	if !hasOceanBaseSQL(commands, "ssl_external_kms_info") || !hasOceanBaseSQL(commands, "ssl_client_authentication=True") || !hasOceanBaseSQL(commands, "UPDATE proxyconfig.security_config") {
		t.Fatalf("TLS commands missing: %#v", commands)
	}
}

func TestOceanBaseRejectsUnsafeParameterAndNonRPMPackage(t *testing.T) {
	root := t.TempDir()
	writeOceanBaseBundle(t, root)
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	req := oceanBaseTestRequest(root)
	req.OceanBase.Parameters = map[string]string{"bad;command": "value"}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "failed" || !strings.Contains(result.Message, "not allowed") {
		t.Fatalf("result = %#v, err = %v", result, err)
	}

	writeFlavorBundle(t, root, "oceanbase", "invalid-rpm")
	req = oceanBaseTestRequest(root)
	req.Version = "invalid-rpm"
	req.PackagePath = filepath.Join(root, "oceanbase", "invalid-rpm")
	result, err = executor.Execute(context.Background(), req)
	if err != nil || result.Status != "failed" || !strings.Contains(result.Message, "must be an RPM") {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

func TestOceanBaseBackupConfiguresArchiveAndVerifiesJob(t *testing.T) {
	root := t.TempDir()
	writeOceanBaseBundle(t, root)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "COMPLETED", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	req := oceanBaseTestRequest(root)
	req.Operation = "backup"
	req.OceanBase.Backup = &OceanBaseBackupConfig{Destination: "file:///opt/dbops/backups/oceanbase/tenant_a"}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	if !hasOceanBaseSQL(commands, "DATA_BACKUP_DEST") || !hasOceanBaseSQL(commands, "BACKUP DATABASE PLUS ARCHIVELOG") || !hasOceanBaseSQL(commands, "CDB_OB_BACKUP_JOBS") {
		t.Fatalf("backup commands missing: %#v", commands)
	}
}

func TestOceanBaseRestoreWaitsAndValidatesTenantData(t *testing.T) {
	root := t.TempDir()
	writeOceanBaseBundle(t, root)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		statement := args[len(args)-1]
		switch {
		case strings.Contains(statement, "CDB_OB_RESTORE_PROGRESS"):
			return "RESTORE_SUCCESS", nil
		case strings.Contains(statement, "information_schema.tables"):
			return "1", nil
		case strings.Contains(statement, "SELECT COUNT(*) FROM app.orders"):
			return "7", nil
		case strings.Contains(statement, "CHECKSUM TABLE app.orders"):
			return "app.orders abc123", nil
		default:
			return "verified", nil
		}
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	req := oceanBaseTestRequest(root)
	req.Operation = "restore"
	req.OceanBase.Restore = &OceanBaseRestoreConfig{
		Tenant: "tenant_restore", BackupSource: "file:///opt/dbops/backups/oceanbase/tenant_a", ResourcePool: "tenant_restore_pool",
		ValidationTables: []OceanBaseTableCheck{{Database: "app", Table: "orders", ExpectedRowCount: 7, ExpectedChecksum: "abc123"}},
	}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	if !hasOceanBaseSQL(commands, "ALTER SYSTEM RESTORE tenant_restore") || !hasOceanBaseSQL(commands, "CDB_OB_RESTORE_PROGRESS") || !hasOceanBaseSQL(commands, "CHECKSUM TABLE app.orders") {
		t.Fatalf("restore commands missing: %#v", commands)
	}
}

func TestOceanBaseUpgradeCreatesBackupBeforeObshellUpgrade(t *testing.T) {
	root := t.TempDir()
	writeOceanBaseBundle(t, root)
	var commands []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		if name == oceanBaseOBClient && strings.Contains(args[len(args)-1], "CDB_OB_BACKUP_JOBS") {
			return "COMPLETED", nil
		}
		if name == oceanBaseOBClient && strings.Contains(args[len(args)-1], "DBA_OB_SERVERS") {
			return "ACTIVE", nil
		}
		return "verified", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	req := oceanBaseTestRequest(root)
	req.Operation = "upgrade"
	req.OceanBase.Backup = &OceanBaseBackupConfig{Destination: "file:///opt/dbops/backups/oceanbase/pre-upgrade"}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	if !hasOceanBaseSQL(commands, "BACKUP DATABASE PLUS ARCHIVELOG") || !hasOceanBaseSQL(commands, "DBA_OB_SERVERS") || !hasOceanBaseCommand(commands, oceanBaseOBShell, "cluster upgrade") {
		t.Fatalf("upgrade commands missing: %#v", commands)
	}
}

func TestOceanBaseRejectsUnapprovedBackupDestination(t *testing.T) {
	root := t.TempDir()
	writeOceanBaseBundle(t, root)
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	req := oceanBaseTestRequest(root)
	req.Operation = "backup"
	req.OceanBase.Backup = &OceanBaseBackupConfig{Destination: "file:///tmp/unapproved"}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "failed" || !strings.Contains(result.Message, "backup destination") {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

func TestOceanBaseRestoreReportsTerminalFailure(t *testing.T) {
	root := t.TempDir()
	writeOceanBaseBundle(t, root)
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, args ...string) (string, error) {
		if strings.Contains(args[len(args)-1], "CDB_OB_RESTORE_PROGRESS") {
			return "RESTORE_FAIL", nil
		}
		return "started", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	req := oceanBaseTestRequest(root)
	req.Operation = "restore"
	req.OceanBase.Restore = &OceanBaseRestoreConfig{
		Tenant: "tenant_restore", BackupSource: "file:///opt/dbops/backups/oceanbase/tenant_a", ResourcePool: "tenant_restore_pool",
		ValidationTables: []OceanBaseTableCheck{{Database: "app", Table: "orders", ExpectedRowCount: 7, ExpectedChecksum: "abc123"}},
	}
	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "failed" || !strings.Contains(result.Message, "RESTORE_FAIL") {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}

func oceanBaseTestRequest(root string) FlavorTaskRequest {
	return FlavorTaskRequest{
		TaskID: "oceanbase-deploy", InstanceID: "oceanbase-1", Flavor: "oceanbase", Version: "v4.4.2_CE_BP2", Operation: "deploy",
		PackagePath: filepath.Join(root, "oceanbase", "v4.4.2_CE_BP2"), TLS: &FlavorTLSConfig{},
		OceanBase: &OceanBaseConfig{
			ClusterName: "obdemo", Address: "10.0.0.8", Zone: "zone1", ClusterID: 10001, DataDir: "/data/oceanbase",
			SystemMemory: "8G", DatafileSize: "20G", RootPassword: "secret", Tenant: "tenant_a",
			TenantCPU: 2, TenantMemory: "2G", TenantLogDiskSize: "10G", Parameters: map[string]string{"enable_sql_audit": "false"},
			EnableOBProxy: true, OBProxySysPasswordSHA: strings.Repeat("a", 40),
		},
	}
}

func writeOceanBaseBundle(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "oceanbase", "v4.4.2_CE_BP2")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := []byte("oceanbase rpm")
	hash := sha256.Sum256(contents)
	if err := os.WriteFile(filepath.Join(dir, "oceanbase.rpm"), contents, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `{"flavor":"oceanbase","version":"v4.4.2_CE_BP2","packages":[{"file":"oceanbase.rpm","sha256":"` + hex.EncodeToString(hash[:]) + `"}]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasOceanBaseCommand(commands []recordedOceanBaseCommand, name, argument string) bool {
	for _, command := range commands {
		if command.name == name && strings.Contains(strings.Join(command.args, " "), argument) {
			return true
		}
	}
	return false
}

func hasOceanBaseSQL(commands []recordedOceanBaseCommand, statement string) bool {
	return hasOceanBaseCommand(commands, oceanBaseOBClient, statement)
}
