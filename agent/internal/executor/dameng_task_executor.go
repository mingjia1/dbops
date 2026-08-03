package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var damengPassword = regexp.MustCompile(`^[A-Za-z0-9!@#%+=._-]{12,48}$`)
var damengParameters = map[string]bool{"COMPATIBLE_MODE": true, "ENABLE_AUTO_INACTIVE": true, "SVR_LOG": true}

func (e *FlavorTaskExecutor) executeDameng(ctx context.Context, req FlavorTaskRequest, manifest LocalPackageManifest) (*TaskResult, error) {
	if err := validateDamengConfig(req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.Operation == "configure" {
		if err := e.configureDameng(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "dameng configuration completed", map[string]interface{}{"flavor": "dm"}), nil
	}
	if !damengPackageAvailable(manifest) {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("dameng bundle requires DMInstall.bin")), nil
	}
	config, err := e.writeDamengInstallConfig(req)
	if err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	installer := filepath.Join(req.PackagePath, "DMInstall.bin")
	if _, err := e.handlers["dm"].runner(ctx, "chmod", "755", installer); err != nil {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("prepare dameng installer: %w", err)), nil
	}
	if _, err := e.handlers["dm"].runner(ctx, installer, "-q", config); err != nil {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("install dameng: %w", err)), nil
	}
	if _, err := e.handlers["dm"].runner(ctx, filepath.Join(req.Dameng.InstallDir, "bin", "dminit"), "PATH="+req.Dameng.DataDir, "SYSDBA_PWD="+req.Dameng.SysdbaPassword); err != nil {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("initialize dameng database: %w", err)), nil
	}
	if err := e.starter(ctx, filepath.Join(req.Dameng.InstallDir, "bin", "dmserver"), filepath.Join(req.Dameng.DataDir, "dm.ini")); err != nil {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("start dameng server: %w", err)), nil
	}
	if err := e.configureDameng(ctx, req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	data := map[string]interface{}{"flavor": "dm"}
	if req.TLS.Enabled {
		data["restart_required"] = true
	}
	return flavorTaskCompleted(req.TaskID, "dameng single-instance deployment completed", data), nil
}

func validateDamengConfig(req FlavorTaskRequest) error {
	c := req.Dameng
	if c == nil || !tidbHost.MatchString(c.Address) || c.Port < 1024 || c.Port > 65534 || !damengPassword.MatchString(c.SysdbaPassword) || !damengStrongPassword(c.SysdbaPassword) || !tidbIdentifier.MatchString(c.ApplicationUser) || !damengPassword.MatchString(c.ApplicationPassword) || !damengStrongPassword(c.ApplicationPassword) || !damengPath(c.InstallDir) || !damengPath(c.DataDir) {
		return fmt.Errorf("dameng deployment inputs are invalid")
	}
	for key, value := range c.Parameters {
		if !damengParameters[key] || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("dameng parameter %q is not allowed", key)
		}
	}
	if req.TLS.Enabled {
		for _, path := range []string{req.TLS.CAFile, req.TLS.CertFile, req.TLS.KeyFile} {
			info, err := os.Stat(path)
			if !filepath.IsAbs(path) || err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("dameng TLS file %q is unavailable", path)
			}
		}
	}
	return nil
}

func damengStrongPassword(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool { return r >= 'A' && r <= 'Z' }) >= 0 && strings.IndexFunc(value, func(r rune) bool { return r >= 'a' && r <= 'z' }) >= 0 && strings.IndexFunc(value, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0
}

func damengPath(path string) bool {
	return filepath.IsAbs(path) && strings.HasPrefix(filepath.Clean(path), "/opt/dbops/dm/") && !strings.Contains(path, "..")
}

func damengPackageAvailable(manifest LocalPackageManifest) bool {
	for _, artifact := range manifest.Packages {
		if artifact.File == "DMInstall.bin" {
			return true
		}
	}
	return false
}

func (e *FlavorTaskExecutor) writeDamengInstallConfig(req FlavorTaskRequest) (string, error) {
	dir := filepath.Join("/opt/dbops/dm", req.InstanceID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create dameng control directory: %w", err)
	}
	path := filepath.Join(dir, "install.xml")
	contents := "<?xml version=\"1.0\"?><DATABASE><LANGUAGE>ZH</LANGUAGE><TIME_ZONE>+08:00</TIME_ZONE><INSTALL_TYPE>1</INSTALL_TYPE><INSTALL_PATH>" + req.Dameng.InstallDir + "</INSTALL_PATH><INIT_DB>N</INIT_DB><CREATE_DB_SERVICE>N</CREATE_DB_SERVICE><STARTUP_DB_SERVICE>N</STARTUP_DB_SERVICE></DATABASE>"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		return "", fmt.Errorf("write dameng install configuration: %w", err)
	}
	return path, nil
}

func (e *FlavorTaskExecutor) configureDameng(ctx context.Context, req FlavorTaskRequest) error {
	connection := "SYSDBA/" + req.Dameng.SysdbaPassword + "@" + req.Dameng.Address + fmt.Sprintf(":%d", req.Dameng.Port)
	for key, value := range req.Dameng.Parameters {
		if _, err := e.handlers["dm"].runner(ctx, filepath.Join(req.Dameng.InstallDir, "bin", "disql"), connection, "-e", "ALTER SYSTEM SET "+key+"="+value+" SPFILE"); err != nil {
			return fmt.Errorf("set dameng parameter %s: %w", key, err)
		}
	}
	if req.TLS.Enabled {
		if _, err := e.handlers["dm"].runner(ctx, filepath.Join(req.Dameng.InstallDir, "bin", "disql"), connection, "-e", "SP_SET_PARA_VALUE(2,'ENABLE_ENCRYPT',5)"); err != nil {
			return fmt.Errorf("enable dameng SSL encryption: %w", err)
		}
	}
	if _, err := e.handlers["dm"].runner(ctx, filepath.Join(req.Dameng.InstallDir, "bin", "disql"), connection, "-e", "CREATE USER "+req.Dameng.ApplicationUser+" IDENTIFIED BY \""+req.Dameng.ApplicationPassword+"\""); err != nil {
		return fmt.Errorf("create dameng application user: %w", err)
	}
	if _, err := e.handlers["dm"].runner(ctx, filepath.Join(req.Dameng.InstallDir, "bin", "disql"), connection, "-e", "SELECT VERSION()"); err != nil {
		return fmt.Errorf("verify dameng version: %w", err)
	}
	return nil
}
