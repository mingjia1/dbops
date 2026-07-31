package executor

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateLocalPackageBundleAcceptsMatchingBundle(t *testing.T) {
	root := t.TempDir()
	packageContents := []byte("approved package")
	packageHash := sha256.Sum256(packageContents)
	bundleDir := filepath.Join(root, "tidb", "v8.5.7")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "tidb.tar.gz"), packageContents, 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `{"flavor":"tidb","version":"v8.5.7","packages":[{"file":"tidb.tar.gz","sha256":"` + hex.EncodeToString(packageHash[:]) + `"}]}`
	if err := os.WriteFile(filepath.Join(bundleDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ValidateLocalPackageBundle(root, "tidb", "v8.5.7")
	if err != nil {
		t.Fatalf("ValidateLocalPackageBundle() error = %v", err)
	}
	if got.Packages[0].File != "tidb.tar.gz" {
		t.Fatalf("package = %q", got.Packages[0].File)
	}
}

func TestValidateLocalPackageBundleRejectsInvalidBundlesBeforeInstallation(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		files    map[string]string
	}{
		{name: "missing manifest"},
		{name: "flavor mismatch", manifest: `{"flavor":"dm","version":"v8.5.7","packages":[{"file":"package.tar","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`},
		{name: "version mismatch", manifest: `{"flavor":"tidb","version":"v8.5.6","packages":[{"file":"package.tar","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`},
		{name: "missing package", manifest: `{"flavor":"tidb","version":"v8.5.7","packages":[{"file":"package.tar","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`},
		{name: "hash mismatch", manifest: `{"flavor":"tidb","version":"v8.5.7","packages":[{"file":"package.tar","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`, files: map[string]string{"package.tar": "different content"}},
		{name: "path escape", manifest: `{"flavor":"tidb","version":"v8.5.7","packages":[{"file":"../package.tar","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`},
		{name: "missing license", manifest: `{"flavor":"tidb","version":"v8.5.7","requires_license":true,"license_file":"license.key","packages":[{"file":"package.tar","sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}`, files: map[string]string{"package.tar": "content"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			bundleDir := filepath.Join(root, "tidb", "v8.5.7")
			if err := os.MkdirAll(bundleDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for name, contents := range tt.files {
				if err := os.WriteFile(filepath.Join(bundleDir, name), []byte(contents), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tt.manifest != "" {
				if err := os.WriteFile(filepath.Join(bundleDir, "manifest.json"), []byte(tt.manifest), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			if _, err := ValidateLocalPackageBundle(root, "tidb", "v8.5.7"); err == nil {
				t.Fatal("ValidateLocalPackageBundle() error = nil")
			}
		})
	}
}
