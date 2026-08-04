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
const gbase8aBackupRoot = "/opt/dbops/backups/gbase8a"

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
	case "monitor":
		data, err := e.monitorGBase8a(ctx, req)
		if err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "gbase8a monitoring snapshot collected", data), nil
	case "teardown":
		if err := teardownGBase8a(req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8a teardown is unavailable because no official GBase 8a V9 uninstall procedure and command parameters are publicly verified")), nil
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
	case "backup", "restore", "migrate":
		if err := e.executeGBase8aLifecycle(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	case "upgrade", "rollback":
		return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8a %s is unavailable: complex cluster topology and version prerequisites require a vendor-verified procedure", req.Operation)), nil
	case "ha", "replication", "failover", "scale", "rebuild":
		return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8a single-node executor does not support %s", req.Operation)), nil
	default:
		return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8a operation %q is unavailable through the dedicated flavor Agent", req.Operation)), nil
	}
	return flavorTaskCompleted(req.TaskID, "gbase8a 10.1 single-node "+req.Operation+" completed", map[string]interface{}{"flavor": "gbase8a", "version": req.Version}), nil
}

func (e *FlavorTaskExecutor) monitorGBase8a(ctx context.Context, req FlavorTaskRequest) (map[string]interface{}, error) {
	status, err := e.handlers["gbase8a"].runner(ctx, gbase8aBinary(req, "gcadmin"), "status")
	if err != nil {
		return nil, fmt.Errorf("collect gbase8a gcadmin status: %w", err)
	}
	processes, err := e.handlers["gbase8a"].runner(ctx, "ps", "-C", "gnode", "-C", "gcluster", "-C", "gcware", "-o", "pid=,stat=,args=")
	if err != nil {
		return nil, fmt.Errorf("collect gbase8a core process status: %w", err)
	}
	capacity, err := e.handlers["gbase8a"].runner(ctx, "df", "-P", req.GBase8a.InstallPrefix)
	if err != nil {
		return nil, fmt.Errorf("collect gbase8a installation capacity: %w", err)
	}
	return map[string]interface{}{
		"flavor":                "gbase8a",
		"gcadmin_status":        status,
		"core_processes":        processes,
		"installation_capacity": capacity,
	}, nil
}

func teardownGBase8a(req FlavorTaskRequest) error {
	if !req.GBase8a.ConfirmUninstall || req.GBase8a.Backup == nil || !gbase8aBackupPath(req.GBase8a.Backup.Destination) {
		return fmt.Errorf("gbase8a teardown requires confirm_uninstall and an approved final backup destination")
	}
	return nil
}

func (e *FlavorTaskExecutor) executeGBase8aLifecycle(ctx context.Context, req FlavorTaskRequest) error {
	if err := validateGBase8aLifecycle(req); err != nil {
		return err
	}
	switch req.Operation {
	case "backup":
		return e.gbase8aWithMode(ctx, req, "readonly", func() error {
			if _, err := e.handlers["gbase8a"].runner(ctx, "python", gbase8aGCRCMan(req), "-d", req.GBase8a.Backup.Destination, "-P", "gbasedba", "-e", "backup level 0"); err != nil {
				return fmt.Errorf("create gbase8a full backup: %w", err)
			}
			return nil
		})
	case "restore":
		restore := req.GBase8a.Restore
		return e.gbase8aWithMode(ctx, req, "recovery", func() error {
			if _, err := e.handlers["gbase8a"].runner(ctx, "python", gbase8aGCRCMan(req), "-d", restore.BackupSource, "-P", "gbasedba", "-e", gbase8aRecoverExpression(restore)); err != nil {
				return fmt.Errorf("recover gbase8a backup: %w", err)
			}
			return nil
		})
	case "migrate":
		migration := req.GBase8a.Migration
		path := strings.TrimPrefix(migration.SourceURI, "file://")
		if err := e.gbase8aSQL(ctx, req, "LOAD DATA INFILE '"+path+"' INTO TABLE `"+migration.Database+"`.`"+migration.Table+"`"); err != nil {
			return fmt.Errorf("load gbase8a migration data: %w", err)
		}
	}
	return nil
}

func (e *FlavorTaskExecutor) gbase8aWithMode(ctx context.Context, req FlavorTaskRequest, mode string, operation func() error) (err error) {
	if _, err := e.handlers["gbase8a"].runner(ctx, gbase8aBinary(req, "gcadmin"), "switchmode", mode); err != nil {
		return fmt.Errorf("switch gbase8a to %s: %w", mode, err)
	}
	defer func() {
		if _, normalErr := e.handlers["gbase8a"].runner(ctx, gbase8aBinary(req, "gcadmin"), "switchmode", "normal"); normalErr != nil {
			if err != nil {
				err = fmt.Errorf("%w; restore gbase8a normal mode: %v", err, normalErr)
				return
			}
			err = fmt.Errorf("restore gbase8a normal mode: %w", normalErr)
		}
	}()
	return operation()
}

func validateGBase8aLifecycle(req FlavorTaskRequest) error {
	c := req.GBase8a
	switch req.Operation {
	case "backup":
		if c.Backup == nil || !gbase8aBackupPath(c.Backup.Destination) {
			return fmt.Errorf("gbase8a backup destination must be under %s", gbase8aBackupRoot)
		}
	case "restore":
		if c.Restore == nil || !gbase8aBackupPath(c.Restore.BackupSource) || !gbase8aRestoreScopeValid(c.Restore) {
			return fmt.Errorf("gbase8a restore requires a controlled backup and either full=true or validated restore objects")
		}
	case "migrate":
		if c.Migration == nil || !gbase8aMigrationFile(c.Migration.SourceURI) || !tidbIdentifier.MatchString(c.Migration.Database) || !tidbIdentifier.MatchString(c.Migration.Table) {
			return fmt.Errorf("gbase8a migration requires a controlled file URI and validated database and table names")
		}
	}
	return nil
}

func gbase8aBackupPath(path string) bool {
	clean := filepath.Clean(path)
	return filepath.IsAbs(path) && strings.HasPrefix(clean, gbase8aBackupRoot+string(filepath.Separator)) && !strings.Contains(path, "..") && !strings.ContainsAny(path, "\r\n")
}

func gbase8aMigrationFile(uri string) bool {
	if !strings.HasPrefix(uri, "file://") || strings.ContainsAny(uri, "'\r\n") {
		return false
	}
	return gbase8aBackupPath(strings.TrimPrefix(uri, "file://"))
}

func gbase8aRestoreScopeValid(restore *GBase8aRestoreConfig) bool {
	if restore.Full {
		return len(restore.Objects) == 0
	}
	if len(restore.Objects) == 0 {
		return false
	}
	for _, object := range restore.Objects {
		if !tidbIdentifier.MatchString(object.Database) || (object.Table != "" && !tidbIdentifier.MatchString(object.Table)) {
			return false
		}
	}
	return true
}

func gbase8aRecoverExpression(restore *GBase8aRestoreConfig) string {
	if restore.Full {
		return "recover"
	}
	object := restore.Objects[0]
	if object.Table == "" {
		return "recover database `" + object.Database + "`"
	}
	return "recover table `" + object.Database + "`.`" + object.Table + "`"
}

func gbase8aGCRCMan(req FlavorTaskRequest) string {
	return filepath.Join(req.GBase8a.InstallPrefix, "server", "bin", "gcrcman.py")
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
