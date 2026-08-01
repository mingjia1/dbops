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
	if err := validateTiDBArchives(manifest, req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	engineVersion, err := e.deployTiDB(ctx, req)
	if err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	return flavorTaskCompleted(req.TaskID, "tidb single-host deployment completed", map[string]interface{}{"flavor": "tidb", "cluster": req.TiDB.ClusterName, "version": req.Version, "engine_version": engineVersion}), nil
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
