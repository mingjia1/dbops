package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestFlavorTaskExecutorValidatesBundleAndDetectsVersion(t *testing.T) {
	root := t.TempDir()
	writeFlavorBundle(t, root, "tidb", "v8.5.7")
	executor := NewFlavorTaskExecutorWithPackageRoot(root, func(_ context.Context, name string, args ...string) (string, error) {
		if name != filepath.Join(root, "tidb", "v8.5.7", "bin", "tidb-server") || len(args) != 1 || args[0] != "--version" {
			t.Fatalf("unexpected command: %s %v", name, args)
		}
		return "TiDB Server v8.5.7", nil
	})
	req := FlavorTaskRequest{TaskID: "task-1", InstanceID: "instance-1", Flavor: "TiDB", Version: "v8.5.7", Operation: "version-detect", PackagePath: filepath.Join(root, "tidb", "v8.5.7"), TLS: &FlavorTLSConfig{Enabled: true}}

	result, err := executor.Execute(context.Background(), req)
	if err != nil || result.Status != "completed" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	data, ok := result.Data.(map[string]interface{})
	if !ok || data["flavor"] != "tidb" {
		t.Fatalf("data = %#v", result.Data)
	}
}

func TestFlavorTaskExecutorRejectsIncompleteOrMismatchedRequests(t *testing.T) {
	root := t.TempDir()
	writeFlavorBundle(t, root, "dm", "DM9")
	executor := NewFlavorTaskExecutorWithPackageRoot(root, nil)
	base := FlavorTaskRequest{TaskID: "task-1", InstanceID: "instance-1", Flavor: "dm", Version: "DM9", Operation: "validate-package", PackagePath: filepath.Join(root, "dm", "DM9"), TLS: &FlavorTLSConfig{}}

	for _, test := range []struct {
		name   string
		mutate func(*FlavorTaskRequest)
	}{
		{name: "missing tls", mutate: func(req *FlavorTaskRequest) { req.TLS = nil }},
		{name: "unknown flavor", mutate: func(req *FlavorTaskRequest) { req.Flavor = "unknown" }},
		{name: "mismatched path", mutate: func(req *FlavorTaskRequest) { req.PackagePath = filepath.Join(root, "tidb", "DM9") }},
		{name: "unsupported operation", mutate: func(req *FlavorTaskRequest) { req.Operation = "deploy" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := base
			test.mutate(&req)
			result, err := executor.Execute(context.Background(), req)
			if err != nil || result.Status != "failed" {
				t.Fatalf("result = %#v, err = %v", result, err)
			}
		})
	}
}

func writeFlavorBundle(t *testing.T, root, flavor, version string) {
	t.Helper()
	dir := filepath.Join(root, flavor, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := []byte("approved package")
	hash := sha256.Sum256(contents)
	if err := os.WriteFile(filepath.Join(dir, "package.tar"), contents, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `{"flavor":"` + flavor + `","version":"` + version + `","packages":[{"file":"package.tar","sha256":"` + hex.EncodeToString(hash[:]) + `"}]}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}
