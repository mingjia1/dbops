package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const openGaussRoot = "/opt/dbops/opengauss"

var openGaussPassword = regexp.MustCompile(`^[A-Za-z0-9!@#%+=._-]{12,128}$`)
var openGaussParameters = map[string]*regexp.Regexp{
	"max_connections":  regexp.MustCompile(`^[1-9][0-9]{0,4}$`),
	"shared_buffers":   regexp.MustCompile(`^[1-9][0-9]{0,4}(kB|MB|GB)$`),
	"log_min_messages": regexp.MustCompile(`^(debug|info|log|notice|warning|error)$`),
}

func (e *FlavorTaskExecutor) executeOpenGauss(ctx context.Context, req FlavorTaskRequest, manifest LocalPackageManifest) (*TaskResult, error) {
	if err := validateOpenGaussConfig(req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.Operation != "deploy" && req.Operation != "configure" {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("operation %q is not executable for flavor %q", req.Operation, "opengauss")), nil
	}
	if req.Operation == "deploy" {
		if !openGaussInstallerAvailable(manifest) {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("opengauss bundle requires manifest-verified install.sh")), nil
		}
		if err := e.deployOpenGauss(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		if err := e.createOpenGaussApplicationUser(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	}
	if err := e.configureOpenGauss(ctx, req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.Operation == "deploy" {
		if err := e.verifyOpenGaussVersion(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	}
	return flavorTaskCompleted(req.TaskID, "opengauss single-node "+req.Operation+" completed", map[string]interface{}{"flavor": "opengauss", "version": req.Version}), nil
}

func validateOpenGaussConfig(req FlavorTaskRequest) error {
	c := req.OpenGauss
	if c == nil || !tidbHost.MatchString(c.Address) || c.Port < 1024 || c.Port > 65534 || !openGaussPath(c.InstallDir) || !openGaussPath(c.DataDir) || !openGaussPassword.MatchString(c.AdminPassword) || !openGaussPassword.MatchString(c.ApplicationPassword) || !tidbIdentifier.MatchString(c.ApplicationUser) {
		return fmt.Errorf("opengauss deployment inputs are invalid")
	}
	for key, value := range c.Parameters {
		validator, allowed := openGaussParameters[key]
		if !allowed || !validator.MatchString(value) {
			return fmt.Errorf("opengauss parameter %q is not allowed", key)
		}
	}
	if req.TLS.Enabled {
		for _, path := range []string{req.TLS.CAFile, req.TLS.CertFile, req.TLS.KeyFile} {
			info, err := os.Lstat(path)
			if !filepath.IsAbs(path) || err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("opengauss TLS file %q is unavailable", path)
			}
		}
	}
	return nil
}

func openGaussPath(path string) bool {
	clean := filepath.Clean(path)
	return filepath.IsAbs(path) && strings.HasPrefix(clean, openGaussRoot+string(filepath.Separator)) && !strings.Contains(path, "..")
}

func openGaussInstallerAvailable(manifest LocalPackageManifest) bool {
	for _, artifact := range manifest.Packages {
		if artifact.File == "install.sh" {
			return true
		}
	}
	return false
}

func (e *FlavorTaskExecutor) deployOpenGauss(ctx context.Context, req FlavorTaskRequest) error {
	if e.inputRunner == nil {
		return fmt.Errorf("opengauss installer input runner is not configured")
	}
	c := req.OpenGauss
	if _, err := e.inputRunner(ctx, c.AdminPassword+"\n", "sh", filepath.Join(req.PackagePath, "install.sh"), "--mode", "single", "-D", c.DataDir, "-R", c.InstallDir, "--start"); err != nil {
		return fmt.Errorf("install opengauss single node: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) configureOpenGauss(ctx context.Context, req FlavorTaskRequest) error {
	c := req.OpenGauss
	keys := make([]string, 0, len(c.Parameters))
	for key := range c.Parameters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := e.handlers["opengauss"].runner(ctx, filepath.Join(c.InstallDir, "bin", "gs_guc"), "set", "-D", c.DataDir, "-c", key+"="+c.Parameters[key]); err != nil {
			return fmt.Errorf("set opengauss parameter %s: %w", key, err)
		}
	}
	if req.TLS.Enabled {
		for _, setting := range []string{"ssl=on", "ssl_cert_file=" + req.TLS.CertFile, "ssl_key_file=" + req.TLS.KeyFile, "ssl_ca_file=" + req.TLS.CAFile} {
			if _, err := e.handlers["opengauss"].runner(ctx, filepath.Join(c.InstallDir, "bin", "gs_guc"), "set", "-D", c.DataDir, "-c", setting); err != nil {
				return fmt.Errorf("configure opengauss TLS: %w", err)
			}
		}
	}
	if _, err := e.handlers["opengauss"].runner(ctx, filepath.Join(c.InstallDir, "bin", "gs_ctl"), "restart", "-D", c.DataDir); err != nil {
		return fmt.Errorf("restart opengauss after configuration: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) createOpenGaussApplicationUser(ctx context.Context, req FlavorTaskRequest) error {
	c := req.OpenGauss
	statement := "CREATE USER " + c.ApplicationUser + " WITH PASSWORD '" + c.ApplicationPassword + "'"
	if _, err := e.handlers["opengauss"].runner(ctx, filepath.Join(c.InstallDir, "bin", "gsql"), openGaussGSQLArgs(req, statement)...); err != nil {
		return fmt.Errorf("create opengauss application user: %w", err)
	}
	statement = "GRANT CONNECT ON DATABASE postgres TO " + c.ApplicationUser
	if _, err := e.handlers["opengauss"].runner(ctx, filepath.Join(c.InstallDir, "bin", "gsql"), openGaussGSQLArgs(req, statement)...); err != nil {
		return fmt.Errorf("grant opengauss application user access: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) verifyOpenGaussVersion(ctx context.Context, req FlavorTaskRequest) error {
	output, err := e.handlers["opengauss"].runner(ctx, filepath.Join(req.OpenGauss.InstallDir, "bin", "gsql"), openGaussGSQLArgs(req, "SELECT version()")...)
	if err != nil {
		return fmt.Errorf("verify opengauss version: %w", err)
	}
	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("verify opengauss version returned empty output")
	}
	if !strings.Contains(output, strings.TrimPrefix(req.Version, "v")) {
		return fmt.Errorf("detected opengauss version %q does not match requested version %q", output, req.Version)
	}
	return nil
}

func openGaussGSQLArgs(req FlavorTaskRequest, statement string) []string {
	c := req.OpenGauss
	return []string{"-d", "postgres", "-h", c.Address, "-p", strconv.Itoa(c.Port), "-U", "omm", "-W", c.AdminPassword, "-c", statement}
}
