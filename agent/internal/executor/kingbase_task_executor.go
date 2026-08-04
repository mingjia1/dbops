package executor

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const kingbaseRoot = "/opt/dbops/kingbase"

var kingbasePassword = regexp.MustCompile(`^[A-Za-z0-9!@#%+=._-]{12,128}$`)
var kingbaseParameters = map[string]*regexp.Regexp{
	"max_connections":  regexp.MustCompile(`^[1-9][0-9]{0,4}$`),
	"shared_buffers":   regexp.MustCompile(`^[1-9][0-9]{0,4}(kB|MB|GB)$`),
	"log_min_messages": regexp.MustCompile(`^(debug|info|log|notice|warning|error)$`),
}

func (e *FlavorTaskExecutor) executeKingbase(ctx context.Context, req FlavorTaskRequest, manifest LocalPackageManifest) (*TaskResult, error) {
	if err := validateKingbaseConfig(req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.Operation != "deploy" && req.Operation != "configure" {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("kingbase KES V9R1C10 only supports deploy and configure; operation %q is refused", req.Operation)), nil
	}
	if req.Operation == "deploy" {
		if !kingbaseBundleAvailable(manifest) {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("kingbase bundle requires manifest-verified setup.sh, silent.conf, and required license")), nil
		}
		if err := e.deployKingbase(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	}
	if err := e.configureKingbase(ctx, req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.Operation == "deploy" {
		if err := e.createKingbaseApplicationUser(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		if err := e.verifyKingbase(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	}
	return flavorTaskCompleted(req.TaskID, "kingbase single-node "+req.Operation+" completed", map[string]interface{}{"flavor": "kingbase", "version": req.Version}), nil
}

func validateKingbaseConfig(req FlavorTaskRequest) error {
	c := req.Kingbase
	if c == nil || !tidbHost.MatchString(c.Address) || c.Port < 1024 || c.Port > 65534 || !kingbasePath(c.InstallDir) || !kingbasePath(c.DataDir) || !kingbasePassword.MatchString(c.SuperuserPassword) || !tidbIdentifier.MatchString(c.ApplicationUser) || !kingbasePassword.MatchString(c.ApplicationPassword) {
		return fmt.Errorf("kingbase deployment inputs are invalid")
	}
	for key, value := range c.Parameters {
		validator, allowed := kingbaseParameters[key]
		if !allowed || !validator.MatchString(value) {
			return fmt.Errorf("kingbase parameter %q is not allowed", key)
		}
	}
	if req.TLS.Enabled {
		if !kingbaseCIDR(c.TLSCIDR) {
			return fmt.Errorf("kingbase TLS CIDR must be a canonical, non-global network")
		}
		for _, path := range []string{req.TLS.CAFile, req.TLS.CertFile, req.TLS.KeyFile} {
			info, err := os.Lstat(path)
			if !filepath.IsAbs(path) || err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("kingbase TLS file %q must be an available regular absolute file", path)
			}
		}
	}
	return nil
}

func kingbasePath(path string) bool {
	clean := filepath.Clean(path)
	return filepath.IsAbs(path) && strings.HasPrefix(clean, kingbaseRoot+string(filepath.Separator)) && !strings.Contains(path, "..")
}

func kingbaseCIDR(value string) bool {
	ip, network, err := net.ParseCIDR(value)
	if err != nil || network.String() != value || !ip.Equal(network.IP) {
		return false
	}
	ones, bits := network.Mask.Size()
	return ones > 0 && bits > 0
}

func kingbaseBundleAvailable(manifest LocalPackageManifest) bool {
	if !manifest.RequiresLicense || manifest.LicenseFile == "" {
		return false
	}
	needed := map[string]bool{"setup.sh": false, "silent.conf": false}
	for _, artifact := range manifest.Packages {
		if _, ok := needed[artifact.File]; ok {
			needed[artifact.File] = true
		}
	}
	return needed["setup.sh"] && needed["silent.conf"]
}

func (e *FlavorTaskExecutor) deployKingbase(ctx context.Context, req FlavorTaskRequest) error {
	if e.inputRunner == nil {
		return fmt.Errorf("kingbase input runner is not configured")
	}
	c := req.Kingbase
	if _, err := e.handlers["kingbase"].runner(ctx, filepath.Join(req.PackagePath, "setup.sh"), "-i", "silent", "-f", filepath.Join(req.PackagePath, "silent.conf")); err != nil {
		return fmt.Errorf("install kingbase using silent setup: %w", err)
	}
	passwordFile := filepath.Join(c.DataDir, ".initdb-password")
	if _, err := e.inputRunner(ctx, c.SuperuserPassword+"\n", "sh", "-c", "umask 077; cat > \"$1\"", "sh", passwordFile); err != nil {
		return fmt.Errorf("write kingbase initialization password file: %w", err)
	}
	if _, err := e.handlers["kingbase"].runner(ctx, kingbaseBinary(req, "initdb"), "-D", c.DataDir, "-U", "superuser", "-A", "scram-sha-256", "--pwfile", passwordFile); err != nil {
		return fmt.Errorf("initialize kingbase database: %w", err)
	}
	if _, err := e.handlers["kingbase"].runner(ctx, kingbaseBinary(req, "sys_ctl"), "start", "-D", c.DataDir); err != nil {
		return fmt.Errorf("start kingbase database: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) configureKingbase(ctx context.Context, req FlavorTaskRequest) error {
	c := req.Kingbase
	keys := make([]string, 0, len(c.Parameters))
	for key := range c.Parameters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := e.kingbaseSQL(ctx, req, "ALTER SYSTEM SET "+key+" = '"+c.Parameters[key]+"';"); err != nil {
			return fmt.Errorf("set kingbase parameter %s: %w", key, err)
		}
	}
	if req.TLS.Enabled {
		for _, setting := range []string{"ssl = 'on'", "ssl_ca_file = '" + kingbaseSQLLiteral(req.TLS.CAFile) + "'", "ssl_cert_file = '" + kingbaseSQLLiteral(req.TLS.CertFile) + "'", "ssl_key_file = '" + kingbaseSQLLiteral(req.TLS.KeyFile) + "'"} {
			if err := e.kingbaseSQL(ctx, req, "ALTER SYSTEM SET "+setting+";"); err != nil {
				return fmt.Errorf("configure kingbase TLS: %w", err)
			}
		}
		entry := "hostssl all all " + c.TLSCIDR + " cert\n"
		if _, err := e.inputRunner(ctx, entry, "sh", "-c", "cat >> \"$1\"", "sh", filepath.Join(c.DataDir, "sys_hba.conf")); err != nil {
			return fmt.Errorf("append kingbase hostssl rule: %w", err)
		}
	}
	if err := e.kingbaseSQL(ctx, req, "SELECT sys_reload_conf();"); err != nil {
		return fmt.Errorf("reload kingbase configuration: %w", err)
	}
	if _, err := e.handlers["kingbase"].runner(ctx, kingbaseBinary(req, "sys_ctl"), "reload", "-D", c.DataDir); err != nil {
		return fmt.Errorf("reload kingbase service configuration: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) createKingbaseApplicationUser(ctx context.Context, req FlavorTaskRequest) error {
	c := req.Kingbase
	return e.kingbaseSQL(ctx, req, "CREATE USER "+c.ApplicationUser+" WITH PASSWORD '"+kingbaseSQLLiteral(c.ApplicationPassword)+"';")
}

func (e *FlavorTaskExecutor) verifyKingbase(ctx context.Context, req FlavorTaskRequest) error {
	if err := e.kingbaseSQL(ctx, req, "SELECT version();"); err != nil {
		return fmt.Errorf("verify kingbase version: %w", err)
	}
	if err := e.kingbaseSQL(ctx, req, "SELECT name, setting FROM sys_settings WHERE name IN ('max_connections', 'ssl');"); err != nil {
		return fmt.Errorf("verify kingbase parameters: %w", err)
	}
	if err := e.kingbaseSQL(ctx, req, "SELECT usename FROM sys_user WHERE usename = '"+req.Kingbase.ApplicationUser+"';"); err != nil {
		return fmt.Errorf("verify kingbase application user: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) kingbaseSQL(ctx context.Context, req FlavorTaskRequest, statement string) error {
	if e.inputRunner == nil {
		return fmt.Errorf("kingbase SQL input runner is not configured")
	}
	c := req.Kingbase
	_, err := e.inputRunner(ctx, statement+"\n", kingbaseBinary(req, "ksql"), "-d", "kingbase", "-h", c.Address, "-p", strconv.Itoa(c.Port), "-U", "superuser")
	return err
}

func kingbaseBinary(req FlavorTaskRequest, name string) string {
	return filepath.Join(req.Kingbase.InstallDir, "bin", name)
}

func kingbaseSQLLiteral(value string) string { return strings.ReplaceAll(value, "'", "''") }
