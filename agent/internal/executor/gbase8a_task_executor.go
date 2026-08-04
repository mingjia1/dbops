package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const gbase8aRoot = "/opt/dbops/gbase8a"
const gbase8aCertificateRoot = "/opt/dbops/gbase8a/certs"

var gbase8aParameters = map[string]*regexp.Regexp{
	"gcluster_rebalancing_concurrent_count": regexp.MustCompile(`^(?:[1-9]|[1-5][0-9]|6[0-4])$`),
}

func (e *FlavorTaskExecutor) executeGBase8a(ctx context.Context, req FlavorTaskRequest, manifest LocalPackageManifest) (*TaskResult, error) {
	if err := validateGBase8aConfig(req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.TLS.Enabled {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8a TLS is unavailable until an official restart command is verified")), nil
	}
	switch req.Operation {
	case "deploy":
		if !gbase8aBundleAvailable(manifest) {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8a bundle requires manifest-verified SetSysEnv.py, gcinstall.py, demo.options, gcChangeInfo.xml, and required license")), nil
		}
		if err := e.deployGBase8a(ctx, req, manifest); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	case "configure":
		if err := e.configureGBase8a(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	default:
		return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8a deploy and configure are available only through the dedicated flavor Agent")), nil
	}
	return flavorTaskCompleted(req.TaskID, "gbase8a 10.1 single-node "+req.Operation+" completed", map[string]interface{}{"flavor": "gbase8a", "version": req.Version}), nil
}

func validateGBase8aConfig(req FlavorTaskRequest) error {
	c := req.GBase8a
	if c == nil || !tidbIdentifier.MatchString(c.DBAUser) || !gbase8aPath(c.InstallPrefix) {
		return fmt.Errorf("gbase8a deployment inputs are invalid")
	}
	for key, value := range c.Parameters {
		validator, allowed := gbase8aParameters[key]
		if !allowed || !validator.MatchString(value) {
			return fmt.Errorf("gbase8a parameter %q is not allowed", key)
		}
	}
	if req.TLS.Enabled {
		for _, path := range []string{req.TLS.CAFile, req.TLS.CertFile, req.TLS.KeyFile} {
			if !gbase8aCertificateFile(path) {
				return fmt.Errorf("gbase8a TLS file %q must be a regular absolute file under %s", path, gbase8aCertificateRoot)
			}
		}
	}
	return nil
}

func gbase8aPath(path string) bool {
	clean := filepath.Clean(path)
	return filepath.IsAbs(path) && (clean == gbase8aRoot || strings.HasPrefix(clean, gbase8aRoot+string(filepath.Separator))) && !strings.Contains(path, "..")
}

func gbase8aCertificateFile(path string) bool {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(path) || !strings.HasPrefix(clean, gbase8aCertificateRoot+string(filepath.Separator)) || strings.Contains(path, "..") {
		return false
	}
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

func gbase8aBundleAvailable(manifest LocalPackageManifest) bool {
	if !manifest.RequiresLicense || manifest.LicenseFile == "" {
		return false
	}
	required := map[string]bool{"SetSysEnv.py": false, "gcinstall.py": false, "demo.options": false, "gcChangeInfo.xml": false}
	for _, artifact := range manifest.Packages {
		if _, ok := required[artifact.File]; ok {
			required[artifact.File] = true
		}
	}
	for _, found := range required {
		if !found {
			return false
		}
	}
	return true
}

func (e *FlavorTaskExecutor) deployGBase8a(ctx context.Context, req FlavorTaskRequest, manifest LocalPackageManifest) error {
	c := req.GBase8a
	if _, err := e.handlers["gbase8a"].runner(ctx, filepath.Join(req.PackagePath, "SetSysEnv.py"), "--dbaUser="+c.DBAUser, "--installPrefix="+c.InstallPrefix); err != nil {
		return fmt.Errorf("set gbase8a environment: %w", err)
	}
	installerArgs := []string{"--license_file=" + filepath.Join(req.PackagePath, manifest.LicenseFile), "--silent=" + filepath.Join(req.PackagePath, "demo.options")}
	if c.PasswordInputMode {
		installerArgs = append(installerArgs, "--passwordInputMode")
	}
	if _, err := e.handlers["gbase8a"].runner(ctx, filepath.Join(req.PackagePath, "gcinstall.py"), installerArgs...); err != nil {
		return fmt.Errorf("install gbase8a: %w", err)
	}
	if _, err := e.handlers["gbase8a"].runner(ctx, gbase8aBinary(req, "gcadmin"), "distribution", filepath.Join(req.PackagePath, "gcChangeInfo.xml"), "p", "1", "d", "0", "pattern", "1"); err != nil {
		return fmt.Errorf("distribute manifest-verified gbase8a topology: %w", err)
	}
	return e.configureGBase8a(ctx, req)
}

func (e *FlavorTaskExecutor) configureGBase8a(ctx context.Context, req FlavorTaskRequest) error {
	keys := make([]string, 0, len(req.GBase8a.Parameters))
	for key := range req.GBase8a.Parameters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := e.gbase8aSQL(ctx, req, "SET GLOBAL "+key+" = "+req.GBase8a.Parameters[key]+";"); err != nil {
			return fmt.Errorf("set gbase8a parameter %s: %w", key, err)
		}
	}
	return nil
}

// writeGBase8aTLSConfig prepares the fixed gbased TLS settings for use after
// a vendor-verified restart procedure becomes available.
func writeGBase8aTLSConfig(req FlavorTaskRequest, current []byte, tls *FlavorTLSConfig) error {
	if tls == nil {
		return fmt.Errorf("gbase8a TLS configuration is required")
	}
	configPath := filepath.Join(req.GBase8a.InstallPrefix, "gbase_8a_gcluster.cnf")
	info, err := os.Lstat(configPath)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("gbase8a TLS configuration must be a regular file")
	}
	if err := os.WriteFile(configPath, []byte(gbase8aTLSConfigContent(string(current), tls)), 0o600); err != nil {
		return fmt.Errorf("write gbase8a TLS configuration: %w", err)
	}
	return os.Chmod(configPath, 0o600)
}

func gbase8aTLSConfigContent(current string, tls *FlavorTLSConfig) string {
	lines := strings.Split(current, "\n")
	result := make([]string, 0, len(lines)+3)
	inGBased := false
	inserted := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if inGBased && !inserted {
				result = append(result, "ssl-ca="+tls.CAFile, "ssl-cert="+tls.CertFile, "ssl-key="+tls.KeyFile)
				inserted = true
			}
			inGBased = trimmed == "[gbased]"
		}
		if inGBased && (strings.HasPrefix(trimmed, "ssl-ca=") || strings.HasPrefix(trimmed, "ssl-cert=") || strings.HasPrefix(trimmed, "ssl-key=")) {
			continue
		}
		result = append(result, line)
	}
	if inGBased && !inserted {
		result = append(result, "ssl-ca="+tls.CAFile, "ssl-cert="+tls.CertFile, "ssl-key="+tls.KeyFile)
		inserted = true
	}
	if !inserted {
		result = append(result, "[gbased]", "ssl-ca="+tls.CAFile, "ssl-cert="+tls.CertFile, "ssl-key="+tls.KeyFile)
	}
	return strings.Join(result, "\n")
}

func (e *FlavorTaskExecutor) gbase8aSQL(ctx context.Context, req FlavorTaskRequest, statement string) error {
	_, err := e.handlers["gbase8a"].runner(ctx, gbase8aBinary(req, "gccli"), "-u"+req.GBase8a.DBAUser, statement)
	return err
}

func gbase8aBinary(req FlavorTaskRequest, binary string) string {
	return filepath.Join(req.GBase8a.InstallPrefix, "bin", binary)
}
