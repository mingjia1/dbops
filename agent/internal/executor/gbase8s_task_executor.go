package executor

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const gbase8sRoot = "/opt/dbops/gbase8s"
const gbase8sBackupRoot = "/opt/dbops/backups/gbase8s"

var gbase8sPassword = regexp.MustCompile(`^[A-Za-z0-9!@#%+=._-]{12,128}$`)
var gbase8sParameters = map[string]*regexp.Regexp{
	"BUFFERS":      regexp.MustCompile(`^[1-9][0-9]{0,6}$`),
	"DYNAMIC_LOGS": regexp.MustCompile(`^[01]$`),
	"LOCKS":        regexp.MustCompile(`^[1-9][0-9]{0,6}$`),
	"LOGFILES":     regexp.MustCompile(`^[1-9][0-9]{0,3}$`),
}

func (e *FlavorTaskExecutor) executeGBase8s(ctx context.Context, req FlavorTaskRequest, manifest LocalPackageManifest) (*TaskResult, error) {
	if err := validateGBase8sConfig(req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	switch req.Operation {
	case "monitor":
		data, err := e.monitorGBase8s(ctx, req)
		if err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "gbase8s monitoring snapshot collected", data), nil
	case "teardown":
		if err := e.teardownGBase8s(req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	case "backup":
		if err := e.backupGBase8s(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "gbase8s backup completed", map[string]interface{}{"flavor": "gbase8s"}), nil
	case "restore":
		if err := e.restoreGBase8s(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "gbase8s restore completed", map[string]interface{}{"flavor": "gbase8s"}), nil
	case "migrate":
		if err := e.migrateGBase8s(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "gbase8s migration completed", map[string]interface{}{"flavor": "gbase8s"}), nil
	case "ha", "replication", "failover", "scale":
		return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8s single-node executor does not support %s", req.Operation)), nil
	case "upgrade":
		return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8s upgrade is unavailable because ON-Bar and cross-version upgrade require external configuration")), nil
	case "deploy", "configure":
	default:
		return flavorTaskFailure(req.TaskID, fmt.Errorf("operation %q is not executable for flavor %q", req.Operation, "gbase8s")), nil
	}
	if req.Operation == "deploy" {
		if !gbase8sInstallerAvailable(manifest) {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("gbase8s bundle requires manifest-verified PluginPak/install_init.sh")), nil
		}
		if _, err := e.handlers["gbase8s"].runner(ctx, "sh", "-c", "cd \"$1/PluginPak\" && sh ./install_init.sh", "sh", req.PackagePath); err != nil {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("install gbase8s with PluginPak/install_init.sh: %w", err)), nil
		}
		if _, err := e.handlers["gbase8s"].runner(ctx, gbase8sBinary(req, "oninit"), "-vy"); err != nil {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("initialize gbase8s instance: %w", err)), nil
		}
	}
	if err := e.configureGBase8s(ctx, req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.Operation == "deploy" {
		if err := e.createGBase8sApplicationUser(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	}
	if err := e.verifyGBase8s(ctx, req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	return flavorTaskCompleted(req.TaskID, "gbase8s single-node "+req.Operation+" completed", map[string]interface{}{"flavor": "gbase8s", "version": req.Version}), nil
}

func (e *FlavorTaskExecutor) monitorGBase8s(ctx context.Context, req FlavorTaskRequest) (map[string]interface{}, error) {
	bin := gbase8sBinary(req, "onstat")
	configuration, err := e.handlers["gbase8s"].runner(ctx, bin, "-c")
	if err != nil {
		return nil, fmt.Errorf("collect gbase8s onstat -c: %w", err)
	}
	configurationGroups, err := e.handlers["gbase8s"].runner(ctx, bin, "-g", "cfg")
	if err != nil {
		return nil, fmt.Errorf("collect gbase8s onstat -g cfg: %w", err)
	}
	version, err := e.handlers["gbase8s"].runner(ctx, bin, "-V")
	if err != nil {
		return nil, fmt.Errorf("collect gbase8s onstat -V: %w", err)
	}
	processes, err := e.handlers["gbase8s"].runner(ctx, "ps", "-C", "oninit", "-o", "pid=,stat=,args=")
	if err != nil {
		return nil, fmt.Errorf("collect gbase8s oninit process status: %w", err)
	}
	disk, err := e.handlers["gbase8s"].runner(ctx, "df", "-P", req.GBase8s.InstallDir)
	if err != nil {
		return nil, fmt.Errorf("collect gbase8s disk status: %w", err)
	}
	return map[string]interface{}{
		"flavor":               "gbase8s",
		"onstat_config":        configuration,
		"onstat_config_groups": configurationGroups,
		"onstat_version":       version,
		"oninit_processes":     processes,
		"disk_status":          disk,
	}, nil
}

func (e *FlavorTaskExecutor) teardownGBase8s(req FlavorTaskRequest) error {
	if !req.GBase8s.ConfirmUninstall || req.GBase8s.Backup == nil || !gbase8sIndependentBackupDirectories(req.GBase8s.Backup.TapeDirectory, req.GBase8s.Backup.LogicalLogDirectory) {
		return fmt.Errorf("gbase8s teardown requires confirm_uninstall and approved final ontape backup directories")
	}
	return fmt.Errorf("gbase8s teardown is unavailable because no official, verified uninstall command is configured")
}

func validateGBase8sConfig(req FlavorTaskRequest) error {
	c := req.GBase8s
	if c == nil || !gbase8sPath(c.InstallDir) {
		return fmt.Errorf("gbase8s install directory is invalid")
	}
	if req.TLS.Enabled {
		return fmt.Errorf("gbase8s TLS is unavailable because no public, verifiable TLS command is configured")
	}
	if req.Operation == "deploy" && (!tidbIdentifier.MatchString(c.ApplicationUser) || !gbase8sPassword.MatchString(c.ApplicationPassword)) {
		return fmt.Errorf("gbase8s deployment inputs are invalid")
	}
	if req.Operation == "deploy" || req.Operation == "configure" {
		for _, parameters := range []map[string]string{c.PersistentParameters, c.MemoryParameters} {
			for key, value := range parameters {
				validator, allowed := gbase8sParameters[key]
				if !allowed || !validator.MatchString(value) {
					return fmt.Errorf("gbase8s parameter %q is not allowed", key)
				}
			}
		}
	}
	if c.Backup != nil && !gbase8sIndependentBackupDirectories(c.Backup.TapeDirectory, c.Backup.LogicalLogDirectory) {
		return fmt.Errorf("gbase8s backup directories must be distinct paths under %s", gbase8sBackupRoot)
	}
	if c.Restore != nil && !gbase8sIndependentBackupDirectories(c.Restore.TapeDirectory, c.Restore.LogicalLogDirectory) {
		return fmt.Errorf("gbase8s restore directories must be distinct paths under %s", gbase8sBackupRoot)
	}
	if c.Migration != nil && (!tidbIdentifier.MatchString(c.Migration.SourceDatabase) || !tidbIdentifier.MatchString(c.Migration.TargetDatabase) || !gbase8sBackupPath(c.Migration.ExportDirectory) || !gbase8sBackupPath(c.Migration.ImportDirectory)) {
		return fmt.Errorf("gbase8s migration inputs are invalid")
	}
	return nil
}

func gbase8sBackupPath(path string) bool {
	clean := filepath.Clean(path)
	return filepath.IsAbs(path) && strings.HasPrefix(clean, gbase8sBackupRoot+string(filepath.Separator)) && !strings.Contains(path, "..")
}

func gbase8sIndependentBackupDirectories(tapeDirectory, logicalLogDirectory string) bool {
	return gbase8sBackupPath(tapeDirectory) && gbase8sBackupPath(logicalLogDirectory) && filepath.Clean(tapeDirectory) != filepath.Clean(logicalLogDirectory)
}

func gbase8sPath(path string) bool {
	clean := filepath.Clean(path)
	return filepath.IsAbs(path) && strings.HasPrefix(clean, gbase8sRoot+string(filepath.Separator)) && !strings.Contains(path, "..")
}

func gbase8sInstallerAvailable(manifest LocalPackageManifest) bool {
	for _, artifact := range manifest.Packages {
		if artifact.File == "PluginPak/install_init.sh" {
			return true
		}
	}
	return false
}

func gbase8sBinary(req FlavorTaskRequest, binary string) string {
	return filepath.Join(req.GBase8s.InstallDir, "bin", binary)
}

func (e *FlavorTaskExecutor) configureGBase8s(ctx context.Context, req FlavorTaskRequest) error {
	c := req.GBase8s
	for _, update := range []struct {
		parameters map[string]string
		mode       string
	}{
		{c.PersistentParameters, "-wf"},
		{c.MemoryParameters, "-wm"},
	} {
		keys := make([]string, 0, len(update.parameters))
		for key := range update.parameters {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if _, err := e.handlers["gbase8s"].runner(ctx, gbase8sBinary(req, "onmode"), update.mode, key+"="+update.parameters[key]); err != nil {
				return fmt.Errorf("set gbase8s parameter %s: %w", key, err)
			}
		}
	}
	return nil
}

func (e *FlavorTaskExecutor) createGBase8sApplicationUser(ctx context.Context, req FlavorTaskRequest) error {
	c := req.GBase8s
	statement := "CREATE USER " + c.ApplicationUser + " WITH PASSWORD '" + c.ApplicationPassword + "';"
	if _, err := e.inputRunner(ctx, statement+"\n", gbase8sBinary(req, "dbaccess"), "-", "-"); err != nil {
		return fmt.Errorf("create gbase8s application user: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) verifyGBase8s(ctx context.Context, req FlavorTaskRequest) error {
	for _, args := range [][]string{{"-c"}, {"-g", "cfg"}} {
		if _, err := e.handlers["gbase8s"].runner(ctx, gbase8sBinary(req, "onstat"), args...); err != nil {
			return fmt.Errorf("verify gbase8s onstat %s: %w", strings.Join(args, " "), err)
		}
	}
	if _, err := e.inputRunner(ctx, "SELECT DBINFO('version', 'full') FROM systables WHERE tabid = 1;\n", gbase8sBinary(req, "dbaccess"), "-", "-"); err != nil {
		return fmt.Errorf("verify gbase8s DB-Access query: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) backupGBase8s(ctx context.Context, req FlavorTaskRequest) error {
	c := req.GBase8s.Backup
	if c == nil {
		return fmt.Errorf("gbase8s backup configuration is required")
	}
	if _, err := e.gbase8sOntape(ctx, req, c.TapeDirectory, c.LogicalLogDirectory, "-s", "-L", "0", "-d"); err != nil {
		return fmt.Errorf("create gbase8s level-0 archive backup: %w", err)
	}
	if _, err := e.gbase8sOntape(ctx, req, c.TapeDirectory, c.LogicalLogDirectory, "-a"); err != nil {
		return fmt.Errorf("archive gbase8s logical logs: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) restoreGBase8s(ctx context.Context, req FlavorTaskRequest) error {
	c := req.GBase8s.Restore
	if c == nil {
		return fmt.Errorf("gbase8s restore configuration is required")
	}
	if _, err := e.gbase8sOntape(ctx, req, c.TapeDirectory, c.LogicalLogDirectory, "-r"); err != nil {
		return fmt.Errorf("restore gbase8s archive backup: %w", err)
	}
	if c.ReplayLogicalLogs {
		if _, err := e.gbase8sOntape(ctx, req, c.TapeDirectory, c.LogicalLogDirectory, "-l"); err != nil {
			return fmt.Errorf("restore gbase8s logical logs: %w", err)
		}
	}
	return nil
}

func (e *FlavorTaskExecutor) gbase8sOntape(ctx context.Context, req FlavorTaskRequest, tapeDirectory, logicalLogDirectory string, args ...string) (string, error) {
	command := append([]string{"TAPEDEV=" + tapeDirectory, "LTAPEDEV=" + logicalLogDirectory, gbase8sBinary(req, "ontape")}, args...)
	return e.handlers["gbase8s"].runner(ctx, "env", command...)
}

func (e *FlavorTaskExecutor) migrateGBase8s(ctx context.Context, req FlavorTaskRequest) error {
	c := req.GBase8s.Migration
	if c == nil {
		return fmt.Errorf("gbase8s migration configuration is required")
	}
	if _, err := e.handlers["gbase8s"].runner(ctx, gbase8sBinary(req, "dbexport"), "-o", c.ExportDirectory, c.SourceDatabase); err != nil {
		return fmt.Errorf("export gbase8s database: %w", err)
	}
	if _, err := e.handlers["gbase8s"].runner(ctx, gbase8sBinary(req, "dbimport"), "-c", "-i", c.ImportDirectory, c.TargetDatabase); err != nil {
		return fmt.Errorf("import gbase8s database: %w", err)
	}
	return nil
}
