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

const tidbControlRoot = "/opt/dbops/tidb"
const tidbBackupRoot = "/opt/dbops/backups/tidb"

var tidbIdentifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,62}$`)
var tidbHost = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)
var tidbPassword = regexp.MustCompile(`^[A-Za-z0-9!@#%+=._-]{12,128}$`)
var tidbInitialPassword = regexp.MustCompile(`The new password is: '([^']+)'`)

var tidbParameters = map[string]bool{
	"tidb.lease":                  true,
	"tidb.log.level":              true,
	"pd.replication.max-replicas": true,
	"tikv.log-level":              true,
}

func (e *FlavorTaskExecutor) executeTiDB(ctx context.Context, req FlavorTaskRequest, manifest LocalPackageManifest) (*TaskResult, error) {
	if err := validateTiDBConfig(req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.Operation == "deploy" {
		if err := validateTiDBArchives(manifest, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	}
	if req.Operation == "configure" {
		if err := e.configureTiDB(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "tidb configuration completed", map[string]interface{}{"flavor": "tidb", "cluster": req.TiDB.ClusterName}), nil
	}
	if req.Operation == "monitor" {
		data, err := e.monitorTiDB(ctx, req)
		if err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "tidb monitoring snapshot collected", data), nil
	}
	if req.Operation == "ha" || req.Operation == "replication" || req.Operation == "failover" || req.Operation == "scale" {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("tidb single-host executor does not support %s", req.Operation)), nil
	}
	if req.Operation == "backup" || req.Operation == "restore" || req.Operation == "migrate" || req.Operation == "upgrade" {
		data, err := e.executeTiDBLifecycle(ctx, req)
		if err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		data["flavor"] = "tidb"
		data["cluster"] = req.TiDB.ClusterName
		return flavorTaskCompleted(req.TaskID, "tidb "+req.Operation+" completed", data), nil
	}
	if req.Operation == "teardown" {
		if err := e.teardownTiDB(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "tidb cluster destroyed", map[string]interface{}{"flavor": "tidb", "cluster": req.TiDB.ClusterName}), nil
	}
	if req.Operation != "deploy" {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("operation %q is not executable for flavor %q", req.Operation, "tidb")), nil
	}
	if err := validateTiDBArchives(manifest, req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	engineVersion, err := e.deployTiDB(ctx, req)
	if err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	return flavorTaskCompleted(req.TaskID, "tidb single-host deployment completed", map[string]interface{}{"flavor": "tidb", "cluster": req.TiDB.ClusterName, "version": req.Version, "engine_version": engineVersion}), nil
}

func (e *FlavorTaskExecutor) monitorTiDB(ctx context.Context, req FlavorTaskRequest) (map[string]interface{}, error) {
	tiup, err := tidbTiUPPath()
	if err != nil {
		return nil, err
	}
	status, err := e.handlers["tidb"].runner(ctx, tiup, "cluster", "display", req.TiDB.ClusterName)
	if err != nil {
		return nil, fmt.Errorf("collect tidb component status: %w", err)
	}
	version, err := e.handlers["tidb"].runner(ctx, "mysql", tidbClientArgs(req, req.TiDB.RootPassword, "SELECT @@version")...)
	if err != nil {
		return nil, fmt.Errorf("collect tidb version: %w", err)
	}
	return map[string]interface{}{"flavor": "tidb", "cluster": req.TiDB.ClusterName, "component_status": status, "engine_version": strings.TrimSpace(version)}, nil
}

func (e *FlavorTaskExecutor) teardownTiDB(ctx context.Context, req FlavorTaskRequest) error {
	if !req.TiDB.ConfirmUninstall || req.TiDB.Backup == nil || !tidbStoragePath(req.TiDB.Backup.Destination) {
		return fmt.Errorf("tidb teardown requires confirm_uninstall and approved backup destination")
	}
	if _, err := e.executeTiDBLifecycle(ctx, FlavorTaskRequest{TaskID: req.TaskID, Flavor: req.Flavor, Version: req.Version, Operation: "backup", TiDB: &TiDBConfig{ClusterName: req.TiDB.ClusterName, Address: req.TiDB.Address, Architecture: req.TiDB.Architecture, DeployUser: req.TiDB.DeployUser, RootPassword: req.TiDB.RootPassword, Backup: req.TiDB.Backup}}); err != nil {
		return fmt.Errorf("back up tidb before teardown: %w", err)
	}
	tiup, err := tidbTiUPPath()
	if err != nil {
		return err
	}
	if _, err := e.handlers["tidb"].runner(ctx, tiup, "cluster", "destroy", req.TiDB.ClusterName, "--yes"); err != nil {
		return fmt.Errorf("destroy tidb cluster: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) executeTiDBLifecycle(ctx context.Context, req FlavorTaskRequest) (map[string]interface{}, error) {
	if err := validateTiDBLifecycle(req); err != nil {
		return nil, err
	}
	tiup, err := tidbTiUPPath()
	if err != nil {
		return nil, err
	}
	switch req.Operation {
	case "backup":
		args := []string{"br", "backup", "full", "--pd", req.TiDB.Address + ":2379", "--storage", req.TiDB.Backup.Destination}
		if _, err := e.handlers["tidb"].runner(ctx, tiup, args...); err != nil {
			return nil, fmt.Errorf("create tidb BR backup: %w", err)
		}
		if req.TiDB.Backup.LogDestination != "" {
			if _, err := e.handlers["tidb"].runner(ctx, tiup, "br", "log", "start", "--pd", req.TiDB.Address+":2379", "--storage", req.TiDB.Backup.LogDestination); err != nil {
				return nil, fmt.Errorf("start tidb log backup: %w", err)
			}
			if _, err := e.handlers["tidb"].runner(ctx, tiup, "br", "log", "status", "--pd", req.TiDB.Address+":2379"); err != nil {
				return nil, fmt.Errorf("verify tidb log backup: %w", err)
			}
		}
	case "restore":
		r := req.TiDB.Restore
		args := []string{"br", "restore", "full", "--pd", req.TiDB.Address + ":2379", "--storage", r.BackupSource}
		if r.RestoredTS != "" {
			args = []string{"br", "restore", "point", "--pd", req.TiDB.Address + ":2379", "--storage", r.LogBackupSource, "--full-backup-storage", r.BackupSource, "--restored-ts", r.RestoredTS}
		}
		if _, err := e.handlers["tidb"].runner(ctx, tiup, args...); err != nil {
			return nil, fmt.Errorf("restore tidb backup: %w", err)
		}
		return map[string]interface{}{}, e.validateTiDBData(ctx, req, r.ValidationTables)
	case "migrate":
		m := req.TiDB.Migration
		dumplingOutput, err := e.handlers["tidb"].runner(ctx, tiup, "dumpling", "-h", req.TiDB.Address, "-P", "4000", "-uroot", "-p"+req.TiDB.RootPassword, "-o", m.DataSourceDir)
		if err != nil {
			return nil, fmt.Errorf("export tidb data with dumpling: %w", err)
		}
		workDir := filepath.Join(e.tidbControlRoot, req.TiDB.ClusterName, req.Version)
		if err := os.MkdirAll(workDir, 0o750); err != nil {
			return nil, fmt.Errorf("create tidb migration control directory: %w", err)
		}
		config := filepath.Join(workDir, "lightning.toml")
		if err := os.WriteFile(config, []byte(tidbLightningConfig(req)), 0o600); err != nil {
			return nil, fmt.Errorf("write tidb lightning configuration: %w", err)
		}
		lightningOutput, err := e.handlers["tidb"].runner(ctx, tiup, "tidb-lightning", "-config", config)
		if err != nil {
			return nil, fmt.Errorf("import tidb data with lightning: %w", err)
		}
		return map[string]interface{}{"dumpling_output": dumplingOutput, "lightning_output": lightningOutput}, nil
	case "upgrade":
		if _, err := e.executeTiDBLifecycle(ctx, FlavorTaskRequest{TaskID: req.TaskID, Flavor: req.Flavor, Version: req.Version, Operation: "backup", TiDB: &TiDBConfig{ClusterName: req.TiDB.ClusterName, Address: req.TiDB.Address, Architecture: req.TiDB.Architecture, DeployUser: req.TiDB.DeployUser, RootPassword: req.TiDB.RootPassword, Backup: req.TiDB.Backup}}); err != nil {
			return nil, fmt.Errorf("back up tidb before upgrade: %w", err)
		}
		if _, err := e.handlers["tidb"].runner(ctx, tiup, "cluster", "upgrade", req.TiDB.ClusterName, req.TiDB.UpgradeVersion, "--yes"); err != nil {
			return nil, fmt.Errorf("upgrade tidb cluster: %w", err)
		}
		output, err := e.handlers["tidb"].runner(ctx, tiup, "cluster", "display", req.TiDB.ClusterName)
		if err != nil {
			return nil, fmt.Errorf("verify upgraded tidb cluster health: %w", err)
		}
		if !strings.Contains(output, "Up") {
			return nil, fmt.Errorf("upgraded tidb cluster is not Up")
		}
	}
	return map[string]interface{}{}, nil
}

func validateTiDBLifecycle(req FlavorTaskRequest) error {
	c := req.TiDB
	switch req.Operation {
	case "backup":
		if c.Backup == nil || !tidbStoragePath(c.Backup.Destination) || (c.Backup.LogDestination != "" && !tidbStoragePath(c.Backup.LogDestination)) {
			return fmt.Errorf("tidb backup destinations must use local storage under %s", tidbBackupRoot)
		}
	case "restore":
		if c.Restore == nil || !tidbStoragePath(c.Restore.BackupSource) || (c.Restore.RestoredTS != "" && (!tidbStoragePath(c.Restore.LogBackupSource) || strings.ContainsAny(c.Restore.RestoredTS, "\r\n"))) {
			return fmt.Errorf("tidb restore sources or timestamp are invalid")
		}
	case "migrate":
		if c.Migration == nil || !tidbLocalPath(c.Migration.DataSourceDir) || !tidbLocalPath(c.Migration.SortedKVDir) {
			return fmt.Errorf("tidb migration directories must be under %s", tidbBackupRoot)
		}
	case "upgrade":
		if c.UpgradeVersion == "" || !strings.HasPrefix(c.UpgradeVersion, "v") || c.Backup == nil || !tidbStoragePath(c.Backup.Destination) {
			return fmt.Errorf("tidb upgrade requires a target version and approved backup destination")
		}
	}
	return nil
}

func tidbStoragePath(value string) bool {
	return strings.HasPrefix(value, "local://"+tidbBackupRoot+"/") && tidbLocalPath(strings.TrimPrefix(value, "local://"))
}

func tidbLocalPath(value string) bool {
	return filepath.IsAbs(value) && strings.HasPrefix(filepath.Clean(value), tidbBackupRoot+string(filepath.Separator)) && !strings.Contains(value, "..")
}

func tidbTiUPPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve TiUP home: %w", err)
	}
	return filepath.Join(home, ".tiup", "bin", "tiup"), nil
}

func (e *FlavorTaskExecutor) configureTiDB(ctx context.Context, req FlavorTaskRequest) error {
	workDir := filepath.Join(e.tidbControlRoot, req.TiDB.ClusterName, req.Version)
	if err := os.MkdirAll(workDir, 0o750); err != nil {
		return fmt.Errorf("create tidb control directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "topology.yaml"), []byte(tidbTopology(req)), 0o600); err != nil {
		return fmt.Errorf("write tidb topology: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve TiUP home: %w", err)
	}
	if _, err := e.handlers["tidb"].runner(ctx, filepath.Join(home, ".tiup", "bin", "tiup"), "cluster", "reload", req.TiDB.ClusterName, "-R", "tidb,pd,tikv", "--yes"); err != nil {
		return fmt.Errorf("reload tidb cluster configuration: %w", err)
	}
	return nil
}

func validateTiDBConfig(req FlavorTaskRequest) error {
	c := req.TiDB
	if c == nil || !tidbIdentifier.MatchString(c.ClusterName) || !tidbHost.MatchString(c.Address) || !tidbIdentifier.MatchString(c.DeployUser) || !tidbPassword.MatchString(c.RootPassword) {
		return fmt.Errorf("tidb cluster_name, address, deploy_user, and root_password are invalid")
	}
	if c.Architecture != "amd64" && c.Architecture != "arm64" {
		return fmt.Errorf("tidb architecture must be amd64 or arm64")
	}
	for key, value := range c.Parameters {
		if !tidbParameters[key] || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("tidb parameter %q is not allowed", key)
		}
	}
	if req.TLS.Enabled {
		for _, path := range []string{req.TLS.CAFile, req.TLS.CertFile, req.TLS.KeyFile} {
			info, err := os.Stat(path)
			if !filepath.IsAbs(path) || strings.Contains(filepath.Clean(path), "..") || err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("tidb TLS file %q is unavailable", path)
			}
		}
	}
	return nil
}

func validateTiDBArchives(manifest LocalPackageManifest, req FlavorTaskRequest) error {
	server, toolkit := false, false
	archives := tidbArchiveNames(req)
	for _, artifact := range manifest.Packages {
		server = server || artifact.File == archives[0]
		toolkit = toolkit || artifact.File == archives[1]
	}
	if !server || !toolkit {
		return fmt.Errorf("tidb bundle requires official server and toolkit archives for %s", req.TiDB.Architecture)
	}
	return nil
}

func (e *FlavorTaskExecutor) deployTiDB(ctx context.Context, req FlavorTaskRequest) (string, error) {
	c := req.TiDB
	workDir := filepath.Join(e.tidbControlRoot, c.ClusterName, req.Version)
	if err := os.MkdirAll(workDir, 0o750); err != nil {
		return "", fmt.Errorf("create tidb control directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "topology.yaml"), []byte(tidbTopology(req)), 0o600); err != nil {
		return "", fmt.Errorf("write tidb topology: %w", err)
	}
	for _, archive := range tidbArchiveNames(req) {
		if _, err := e.handlers["tidb"].runner(ctx, "tar", "-xzf", filepath.Join(req.PackagePath, archive), "-C", workDir); err != nil {
			return "", fmt.Errorf("extract tidb archive %q: %w", archive, err)
		}
	}
	serverDir := filepath.Join(workDir, "tidb-community-server-"+req.Version+"-linux-"+c.Architecture)
	if _, err := e.handlers["tidb"].runner(ctx, "sh", filepath.Join(serverDir, "local_install.sh")); err != nil {
		return "", fmt.Errorf("install offline tiup: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve TiUP home: %w", err)
	}
	tiup := filepath.Join(home, ".tiup", "bin", "tiup")
	topology := filepath.Join(workDir, "topology.yaml")
	if _, err := e.handlers["tidb"].runner(ctx, tiup, "cluster", "deploy", c.ClusterName, req.Version, topology, "--user", c.DeployUser); err != nil {
		return "", fmt.Errorf("deploy tidb cluster: %w", err)
	}
	startOutput, err := e.handlers["tidb"].runner(ctx, tiup, "cluster", "start", c.ClusterName, "--init")
	if err != nil {
		return "", fmt.Errorf("start tidb cluster: %w", err)
	}
	match := tidbInitialPassword.FindStringSubmatch(startOutput)
	if len(match) != 2 {
		return "", fmt.Errorf("tidb safe start did not return an initial root password")
	}
	if _, err := e.handlers["tidb"].runner(ctx, "mysql", tidbClientArgs(req, match[1], "ALTER USER 'root'@'%' IDENTIFIED BY '"+c.RootPassword+"'")...); err != nil {
		return "", fmt.Errorf("set tidb root password: %w", err)
	}
	output, err := e.handlers["tidb"].runner(ctx, tiup, "cluster", "display", c.ClusterName)
	if err != nil {
		return "", fmt.Errorf("display tidb cluster: %w", err)
	}
	if !strings.Contains(output, "Up") {
		return "", fmt.Errorf("tidb cluster is not Up")
	}
	version, err := e.handlers["tidb"].runner(ctx, "mysql", tidbClientArgs(req, c.RootPassword, "SELECT @@version")...)
	if err != nil || strings.TrimSpace(version) == "" {
		return "", fmt.Errorf("query tidb engine version: %w", err)
	}
	if _, err := e.handlers["tidb"].runner(ctx, "mysql", tidbClientArgs(req, c.RootPassword, "SHOW CONFIG WHERE TYPE='tidb'")...); err != nil {
		return "", fmt.Errorf("verify tidb configuration: %w", err)
	}
	return strings.TrimSpace(version), nil
}

func tidbClientArgs(req FlavorTaskRequest, password, statement string) []string {
	args := []string{"-h", req.TiDB.Address, "-P", "4000", "-uroot", "-p" + password}
	if req.TLS.Enabled {
		args = append(args, "--ssl-mode=VERIFY_CA", "--ssl-ca="+req.TLS.CAFile, "--ssl-cert="+req.TLS.CertFile, "--ssl-key="+req.TLS.KeyFile)
	}
	return append(args, "--execute", statement)
}

func (e *FlavorTaskExecutor) validateTiDBData(ctx context.Context, req FlavorTaskRequest, checks []TiDBTableCheck) error {
	for _, check := range checks {
		if !tidbIdentifier.MatchString(check.Database) || !tidbIdentifier.MatchString(check.Table) || check.ExpectedRowCount < 0 || !regexp.MustCompile(`^[a-fA-F0-9]{64}$`).MatchString(check.ExpectedChecksum) {
			return fmt.Errorf("tidb validation table is invalid")
		}
		count, err := e.handlers["tidb"].runner(ctx, "mysql", tidbClientArgs(req, req.TiDB.RootPassword, "SELECT COUNT(*) FROM `"+check.Database+"`.`"+check.Table+"`")...)
		if err != nil {
			return fmt.Errorf("validate tidb row count for %s.%s: %w", check.Database, check.Table, err)
		}
		if strings.TrimSpace(count) != fmt.Sprintf("%d", check.ExpectedRowCount) {
			return fmt.Errorf("tidb row count does not match for %s.%s", check.Database, check.Table)
		}
		checksum, err := e.handlers["tidb"].runner(ctx, "mysql", tidbClientArgs(req, req.TiDB.RootPassword, "CHECKSUM TABLE `"+check.Database+"`.`"+check.Table)...)
		if err != nil {
			return fmt.Errorf("validate tidb checksum for %s.%s: %w", check.Database, check.Table, err)
		}
		if !strings.Contains(strings.ToLower(checksum), strings.ToLower(check.ExpectedChecksum)) {
			return fmt.Errorf("tidb checksum does not match for %s.%s", check.Database, check.Table)
		}
	}
	return nil
}

func tidbLightningConfig(req FlavorTaskRequest) string {
	m := req.TiDB.Migration
	return "[lightning]\nlevel = \"info\"\n[tikv-importer]\nbackend = \"local\"\nsorted-kv-dir = \"" + m.SortedKVDir + "\"\n[mydumper]\ndata-source-dir = \"" + m.DataSourceDir + "\"\n[tidb]\nhost = \"" + req.TiDB.Address + "\"\nport = 4000\nuser = \"root\"\npassword = \"" + req.TiDB.RootPassword + "\"\nstatus-port = 10080\npd-addr = \"" + req.TiDB.Address + ":2379\"\n"
}

func tidbArchiveNames(req FlavorTaskRequest) []string {
	suffix := req.Version + "-linux-" + req.TiDB.Architecture + ".tar.gz"
	return []string{"tidb-community-server-" + suffix, "tidb-community-toolkit-" + suffix}
}

func tidbTopology(req FlavorTaskRequest) string {
	c := req.TiDB
	lines := []string{
		"global:", "  user: \"" + c.DeployUser + "\"", "  deploy_dir: \"/tidb-deploy\"", "  data_dir: \"/tidb-data\"",
		"pd_servers:", "  - host: " + c.Address,
		"tidb_servers:", "  - host: " + c.Address,
		"tikv_servers:", "  - host: " + c.Address,
		"server_configs:",
	}
	for _, component := range []string{"tidb", "pd", "tikv"} {
		keys := make([]string, 0, len(c.Parameters))
		for key := range c.Parameters {
			if strings.HasPrefix(key, component+".") {
				keys = append(keys, key)
			}
		}
		if req.TLS.Enabled {
			keys = append(keys, tidbTLSKeys(component)...)
		}
		if len(keys) == 0 {
			continue
		}
		sort.Strings(keys)
		lines = append(lines, "  "+component+":")
		for _, key := range keys {
			value := c.Parameters[key]
			if req.TLS.Enabled {
				switch {
				case strings.HasSuffix(key, "ssl-ca"), strings.HasSuffix(key, "cacert-path"), strings.HasSuffix(key, "ca-path"):
					value = req.TLS.CAFile
				case strings.HasSuffix(key, "ssl-cert"), strings.HasSuffix(key, "cert-path"):
					value = req.TLS.CertFile
				case strings.HasSuffix(key, "ssl-key"), strings.HasSuffix(key, "key-path"):
					value = req.TLS.KeyFile
				}
			}
			lines = append(lines, "    "+strings.TrimPrefix(key, component+".")+": \""+value+"\"")
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func tidbTLSKeys(component string) []string {
	switch component {
	case "tidb":
		return []string{"tidb.security.cluster-ssl-ca", "tidb.security.cluster-ssl-cert", "tidb.security.cluster-ssl-key", "tidb.security.ssl-ca", "tidb.security.ssl-cert", "tidb.security.ssl-key"}
	case "pd":
		return []string{"pd.security.cacert-path", "pd.security.cert-path", "pd.security.key-path"}
	case "tikv":
		return []string{"tikv.security.ca-path", "tikv.security.cert-path", "tikv.security.key-path"}
	default:
		return nil
	}
}
