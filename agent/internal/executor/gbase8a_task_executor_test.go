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
