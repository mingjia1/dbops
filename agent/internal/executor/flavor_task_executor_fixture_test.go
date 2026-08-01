package executor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlavorTaskExecutorFixtureScenarios(t *testing.T) {
	tests := []struct {
		name       string
		operation  string
		runner     flavorCommandRunner
		corrupt    bool
		wantStatus string
		wantError  string
	}{
		{
			name:      "successful version detection",
			operation: "version-detect",
			runner: func(_ context.Context, _ string, _ ...string) (string, error) {
				return "TiDB Server v8.5.7", nil
			},
			wantStatus: "completed",
		},
		{
			name:      "missing tool",
			operation: "version-detect",
			runner: func(_ context.Context, _ string, _ ...string) (string, error) {
				return "", exec.ErrNotFound
			},
			wantStatus: "failed",
			wantError:  "executable file not found",
		},
		{
			name:      "permission denied",
			operation: "version-detect",
			runner: func(_ context.Context, _ string, _ ...string) (string, error) {
				return "", os.ErrPermission
			},
			wantStatus: "failed",
			wantError:  "permission denied",
		},
		{
			name:       "package checksum validation failure",
			operation:  "validate-package",
			corrupt:    true,
			wantStatus: "failed",
			wantError:  "sha256 does not match",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeFlavorBundle(t, root, "tidb", "v8.5.7")
			if test.corrupt {
				if err := os.WriteFile(filepath.Join(root, "tidb", "v8.5.7", "package.tar"), []byte("corrupted package"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			executor := NewFlavorTaskExecutorWithPackageRoot(root, test.runner)
			req := FlavorTaskRequest{
				TaskID:      "fixture-task",
				InstanceID:  "fixture-instance",
				Flavor:      "tidb",
				Version:     "v8.5.7",
				Operation:   test.operation,
				PackagePath: filepath.Join(root, "tidb", "v8.5.7"),
				TLS:         &FlavorTLSConfig{Enabled: true},
			}

			result, err := executor.Execute(context.Background(), req)
			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			if result.Status != test.wantStatus {
				t.Fatalf("status = %q, want %q; message = %q", result.Status, test.wantStatus, result.Message)
			}
			if test.wantError != "" && !strings.Contains(result.Message, test.wantError) {
				t.Fatalf("message = %q, want substring %q", result.Message, test.wantError)
			}
		})
	}
}

func TestFlavorTaskExecutorFixtureCoversEveryRegisteredFlavor(t *testing.T) {
	for _, flavor := range []string{
		"oceanbase", "gaussdb-mysql", "polardb-mysql", "tdsql-mysql", "tidb", "kingbase",
		"opengauss", "highgo", "gbase8a", "shentong", "dm", "gbase8s",
	} {
		t.Run(flavor, func(t *testing.T) {
			root := t.TempDir()
			version := "fixture-1.0"
			writeFlavorBundle(t, root, flavor, version)
			executor := NewFlavorTaskExecutorWithPackageRoot(root, func(_ context.Context, _ string, _ ...string) (string, error) {
				return flavor + " " + version, nil
			})

			result, err := executor.Execute(context.Background(), FlavorTaskRequest{
				TaskID: "fixture-" + flavor, InstanceID: "fixture-instance", Flavor: flavor, Version: version,
				Operation: "version-detect", PackagePath: filepath.Join(root, flavor, version), TLS: &FlavorTLSConfig{},
			})
			if err != nil || result.Status != "completed" {
				t.Fatalf("result = %#v, err = %v", result, err)
			}
		})
	}
}

func TestFlavorTaskExecutorFixtureReturnsRunnerErrorsUnchanged(t *testing.T) {
	root := t.TempDir()
	writeFlavorBundle(t, root, "dm", "DM9")
	runnerErr := errors.New("fixture runner failure")
	executor := NewFlavorTaskExecutorWithPackageRoot(root, func(_ context.Context, _ string, _ ...string) (string, error) {
		return "", runnerErr
	})

	result, err := executor.Execute(context.Background(), FlavorTaskRequest{
		TaskID: "fixture-task", InstanceID: "fixture-instance", Flavor: "dm", Version: "DM9",
		Operation: "version-detect", PackagePath: filepath.Join(root, "dm", "DM9"), TLS: &FlavorTLSConfig{},
	})
	if err != nil || result.Status != "failed" || !strings.Contains(result.Message, runnerErr.Error()) {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
}
