package executor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const defaultLocalPackageRoot = "/opt/dbops/packages"

// LocalPackageManifest is stored as manifest.json beside an approved local
// package bundle. Every package is checked before a flavor executor runs an
// installation command.
type LocalPackageManifest struct {
	Flavor          string                 `json:"flavor"`
	Version         string                 `json:"version"`
	RequiresLicense bool                   `json:"requires_license"`
	LicenseFile     string                 `json:"license_file,omitempty"`
	Packages        []LocalPackageArtifact `json:"packages"`
}

type LocalPackageArtifact struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

// ValidateLocalPackageBundle validates only files already placed on the target
// host. It performs no download, extraction, or installation work.
func ValidateLocalPackageBundle(root, flavor, version string) (LocalPackageManifest, error) {
	if root == "" {
		root = defaultLocalPackageRoot
	}
	flavor = strings.ToLower(strings.TrimSpace(flavor))
	version = strings.TrimSpace(version)
	if flavor == "" || version == "" {
		return LocalPackageManifest{}, fmt.Errorf("flavor and version are required")
	}

	bundleDir := filepath.Join(root, flavor, version)
	manifestPath := filepath.Join(bundleDir, "manifest.json")
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		return LocalPackageManifest{}, fmt.Errorf("read local package manifest %s: %w", manifestPath, err)
	}

	var manifest LocalPackageManifest
	if err := json.Unmarshal(contents, &manifest); err != nil {
		return LocalPackageManifest{}, fmt.Errorf("parse local package manifest: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(manifest.Flavor)) != flavor {
		return LocalPackageManifest{}, fmt.Errorf("local package flavor %q does not match requested flavor %q", manifest.Flavor, flavor)
	}
	if strings.TrimSpace(manifest.Version) != version {
		return LocalPackageManifest{}, fmt.Errorf("local package version %q does not match requested version %q", manifest.Version, version)
	}
	if len(manifest.Packages) == 0 {
		return LocalPackageManifest{}, fmt.Errorf("local package manifest has no packages")
	}
	if manifest.RequiresLicense {
		if err := validateBundleFile(bundleDir, manifest.LicenseFile); err != nil {
			return LocalPackageManifest{}, fmt.Errorf("validate required license: %w", err)
		}
	}

	for _, artifact := range manifest.Packages {
		if err := validateArtifact(bundleDir, artifact); err != nil {
			return LocalPackageManifest{}, err
		}
	}
	return manifest, nil
}

func validateArtifact(bundleDir string, artifact LocalPackageArtifact) error {
	if len(artifact.SHA256) != sha256.Size*2 {
		return fmt.Errorf("package %q has an invalid sha256", artifact.File)
	}
	if _, err := hex.DecodeString(artifact.SHA256); err != nil {
		return fmt.Errorf("package %q has an invalid sha256: %w", artifact.File, err)
	}
	path, err := bundleFilePath(bundleDir, artifact.File)
	if err != nil {
		return fmt.Errorf("package %q: %w", artifact.File, err)
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open local package %q: %w", artifact.File, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash local package %q: %w", artifact.File, err)
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), artifact.SHA256) {
		return fmt.Errorf("local package %q sha256 does not match manifest", artifact.File)
	}
	return nil
}

func validateBundleFile(bundleDir, name string) error {
	if _, err := bundleFilePath(bundleDir, name); err != nil {
		return err
	}
	return nil
}

func bundleFilePath(bundleDir, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("must be a relative file name")
	}
	path := filepath.Join(bundleDir, name)
	relative, err := filepath.Rel(bundleDir, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("must remain inside the package bundle")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("must be a regular file")
	}
	return path, nil
}
