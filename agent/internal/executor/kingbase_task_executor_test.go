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

func TestKingbaseDeploysWithSilentSetupAndInputOnlyCredentials(t *testing.T) {
	root := t.TempDir()
	writeKingbaseBundle(t, root, true)
	var commands []recordedOceanBaseCommand
	var inputs []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, name string, args ...string) (string, error) {
		commands = append(commands, recordedOceanBaseCommand{name, args})
		return "KingbaseES KES V9R1C10", nil
	}, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.inputRunner = func(_ context.Context, input, name string, args ...string) (string, error) {
		inputs = append(inputs, recordedOceanBaseCommand{name, append([]string{input}, args...)})
		return "ok", nil
	}

	result, err := executor.Execute(context.Background(), kingbaseTestRequest(root))
	joined := allKingbaseCommandText(commands)
	if err != nil || result.Status != "completed" || !hasDamengCommand(commands, "setup.sh", "-i silent -f") || !hasDamengCommand(commands, "initdb", "-D /opt/dbops/kingbase/data -U superuser -A scram-sha-256 --pwfile") || !hasDamengCommand(commands, "sys_ctl", "start -D /opt/dbops/kingbase/data") || strings.Contains(joined, "SuperPass1234") || strings.Contains(joined, "AppPass123456") || !hasKingbaseInput(inputs, "CREATE USER app_user WITH PASSWORD 'AppPass123456'") || !hasKingbaseInput(inputs, "SELECT version();") || !hasKingbaseInput(inputs, "sys_reload_conf") {
		t.Fatalf("result = %#v, commands = %#v, inputs = %#v, err = %v", result, commands, inputs, err)
	}
}

func TestKingbaseConfigureAppliesTLSAndExactCIDR(t *testing.T) {
	root := t.TempDir()
	writeKingbaseBundle(t, root, true)
	certDir := t.TempDir()
	files := map[string]string{}
	for _, name := range []string{"ca.pem", "cert.pem", "key.pem"} {
		path := filepath.Join(certDir, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		files[name] = path
	}
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
	req := kingbaseTestRequest(root)
	req.Operation = "configure"
	req.Kingbase.TLSCIDR = "10.24.0.0/24"
	req.TLS = &FlavorTLSConfig{Enabled: true, CAFile: files["ca.pem"], CertFile: files["cert.pem"], KeyFile: files["key.pem"]}

	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" || !hasKingbaseInput(inputs, "ALTER SYSTEM SET ssl = 'on'") || !hasKingbaseInput(inputs, "hostssl all all 10.24.0.0/24 cert") || !hasDamengCommand(commands, "sys_ctl", "reload -D /opt/dbops/kingbase/data") {
		t.Fatalf("result = %#v, commands = %#v, inputs = %#v, err = %v", result, commands, inputs, err)
	}
}

func TestKingbaseRejectsUnsafeBundlesTLSCIDRsAndOperations(t *testing.T) {
	root := t.TempDir()
	writeKingbaseBundle(t, root, false)
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "ok", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.inputRunner = func(_ context.Context, _ string, _ string, _ ...string) (string, error) { return "ok", nil }
	if result, err := executor.Execute(context.Background(), kingbaseTestRequest(root)); err != nil || result.Status != "failed" || !strings.Contains(result.Message, "setup.sh") {
		t.Fatalf("bundle result = %#v, err = %v", result, err)
	}
	writeKingbaseBundle(t, root, true)
	for _, mutate := range []func(*FlavorTaskRequest){
		func(req *FlavorTaskRequest) { req.Kingbase.DataDir = "/tmp/kingbase" },
		func(req *FlavorTaskRequest) { req.Kingbase.TLSCIDR = "0.0.0.0/0"; req.TLS.Enabled = true },
		func(req *FlavorTaskRequest) { req.Kingbase.TLSCIDR = "10.24.0.1/24"; req.TLS.Enabled = true },
		func(req *FlavorTaskRequest) { req.Operation = "ha" },
	} {
		req := kingbaseTestRequest(root)
		mutate(&req)
		if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" {
			t.Fatalf("unsafe request result = %#v, err = %v", result, err)
		}
	}
}

func TestKingbaseBackupRestoreAndMigrationUseControlledArguments(t *testing.T) {
	root := t.TempDir()
	writeKingbaseBundle(t, root, true)
	var inputs []recordedOceanBaseCommand
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { return "ok", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.inputRunner = func(_ context.Context, input, name string, args ...string) (string, error) {
		inputs = append(inputs, recordedOceanBaseCommand{name, append([]string{input}, args...)})
		return "ok", nil
	}

	backup := kingbaseTestRequest(root)
	backup.Operation = "backup"
	backup.Kingbase.Backup = &KingbaseBackupConfig{Destination: "/opt/dbops/backups/kingbase/physical"}
	if result, err := executor.Execute(context.Background(), backup); err != nil || result.Status != "completed" || !hasKingbaseInputCommand(inputs, "sys_basebackup", "-D /opt/dbops/backups/kingbase/physical -Fp -Pv -Xf") {
		t.Fatalf("backup result = %#v, inputs = %#v, err = %v", result, inputs, err)
	}

	restore := kingbaseTestRequest(root)
	restore.Operation = "restore"
	restore.Kingbase.Restore = &KingbaseRestoreConfig{BackupSource: "/opt/dbops/backups/kingbase/source.dump", TargetDatabase: "newdb"}
	if result, err := executor.Execute(context.Background(), restore); err != nil || result.Status != "completed" || !hasKingbaseInputCommand(inputs, "sys_restore", "-d newdb /opt/dbops/backups/kingbase/source.dump") {
		t.Fatalf("restore result = %#v, inputs = %#v, err = %v", result, inputs, err)
	}

	migration := kingbaseTestRequest(root)
	migration.Operation = "migrate"
	migration.Kingbase.Migration = &KingbaseMigrationConfig{SourceDatabase: "sourcedb", TargetDatabase: "targetdb", DumpFile: "/opt/dbops/backups/kingbase/migrate.dump"}
	if result, err := executor.Execute(context.Background(), migration); err != nil || result.Status != "completed" || !hasKingbaseInputCommand(inputs, "sys_dump", "-Fc -f /opt/dbops/backups/kingbase/migrate.dump sourcedb") || !hasKingbaseInputCommand(inputs, "sys_restore", "-d targetdb /opt/dbops/backups/kingbase/migrate.dump") {
		t.Fatalf("migration result = %#v, inputs = %#v, err = %v", result, inputs, err)
	}
	for _, input := range inputs {
		if strings.Contains(strings.Join(input.args[1:], " "), "SuperPass1234") {
			t.Fatalf("password leaked in command arguments: %#v", input)
		}
	}
}

func TestKingbaseRefusesUnsafeLifecyclePITRAndUpgrade(t *testing.T) {
	root := t.TempDir()
	writeKingbaseBundle(t, root, true)
	var calls int
	executor := NewFlavorTaskExecutorWithPackageRootAndStarter(root, func(_ context.Context, _ string, _ ...string) (string, error) { calls++; return "ok", nil }, func(_ context.Context, _ string, _ ...string) error { return nil })
	executor.inputRunner = func(_ context.Context, _ string, _ string, _ ...string) (string, error) { calls++; return "ok", nil }
	for _, configure := range []func(*FlavorTaskRequest){
		func(req *FlavorTaskRequest) {
			req.Operation = "backup"
			req.Kingbase.Backup = &KingbaseBackupConfig{Destination: "/opt/dbops/backups/kingbase/../outside"}
		},
		func(req *FlavorTaskRequest) {
			req.Operation = "restore"
			req.Kingbase.Restore = &KingbaseRestoreConfig{BackupSource: "/opt/dbops/backups/kingbase/source.dump", TargetDatabase: "newdb", RecoveryTarget: "2026-08-04 00:00:00"}
		},
		func(req *FlavorTaskRequest) {
			req.Operation = "restore"
			req.Kingbase.Restore = &KingbaseRestoreConfig{BackupSource: "/tmp/source.dump", TargetDatabase: "newdb"}
		},
		func(req *FlavorTaskRequest) {
			req.Operation = "upgrade"
		},
	} {
		req := kingbaseTestRequest(root)
		configure(&req)
		if result, err := executor.Execute(context.Background(), req); err != nil || result.Status != "failed" {
			t.Fatalf("unsafe lifecycle result = %#v, err = %v", result, err)
		}
	}
	if calls != 0 {
		t.Fatalf("unsafe lifecycle issued %d commands", calls)
	}
}

func kingbaseTestRequest(root string) FlavorTaskRequest {
	return FlavorTaskRequest{TaskID: "kingbase-deploy", InstanceID: "kingbase-1", Flavor: "kingbase", Version: "KES V9R1C10", Operation: "deploy", PackagePath: filepath.Join(root, "kingbase", "KES V9R1C10"), TLS: &FlavorTLSConfig{}, Kingbase: &KingbaseConfig{Address: "10.0.0.10", Port: 54321, InstallDir: "/opt/dbops/kingbase/install", DataDir: "/opt/dbops/kingbase/data", SuperuserPassword: "SuperPass1234", ApplicationUser: "app_user", ApplicationPassword: "AppPass123456", Parameters: map[string]string{"max_connections": "200"}}}
}

func writeKingbaseBundle(t *testing.T, root string, complete bool) {
	t.Helper()
	dir := filepath.Join(root, "kingbase", "KES V9R1C10")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	artifacts := []string{"setup.sh", "silent.conf"}
	if !complete {
		artifacts = []string{"setup.sh"}
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
	manifest := `{"flavor":"kingbase","version":"KES V9R1C10","requires_license":true,"license_file":"license.dat","packages":[` + strings.Join(entries, ",") + `]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasKingbaseInput(inputs []recordedOceanBaseCommand, want string) bool {
	for _, input := range inputs {
		if len(input.args) > 0 && strings.Contains(input.args[0], want) {
			return true
		}
	}
	return false
}

func hasKingbaseInputCommand(inputs []recordedOceanBaseCommand, binary, wantArgs string) bool {
	for _, input := range inputs {
		if strings.Contains(input.name, binary) && strings.Join(input.args[1:], " ") == wantArgs {
			return true
		}
	}
	return false
}

func allKingbaseCommandText(commands []recordedOceanBaseCommand) string {
	values := make([]string, 0, len(commands))
	for _, command := range commands {
		values = append(values, command.name+" "+strings.Join(command.args, " "))
	}
	return strings.Join(values, "\n")
}
