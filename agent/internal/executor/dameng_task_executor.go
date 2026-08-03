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

const damengBackupRoot = "/opt/dbops/backups/dm"

func (e *FlavorTaskExecutor) executeDameng(ctx context.Context, req FlavorTaskRequest, manifest LocalPackageManifest) (*TaskResult, error) {
	if err := validateDamengConfig(req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.Operation == "monitor" {
		data, err := e.monitorDameng(ctx, req)
		if err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "dameng monitoring snapshot collected", data), nil
	}
	if req.Operation == "ha" || req.Operation == "replication" || req.Operation == "failover" || req.Operation == "scale" {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("dameng single-instance executor does not support %s", req.Operation)), nil
	}
	if req.Operation == "teardown" {
		if err := e.teardownDameng(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "dameng instance uninstalled", map[string]interface{}{"flavor": "dm"}), nil
	}
	if req.Operation == "backup" || req.Operation == "restore" || req.Operation == "migrate" {
		if err := e.executeDamengLifecycle(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "dameng "+req.Operation+" completed", map[string]interface{}{"flavor": "dm"}), nil
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

func (e *FlavorTaskExecutor) monitorDameng(ctx context.Context, req FlavorTaskRequest) (map[string]interface{}, error) {
	bin := filepath.Join(req.Dameng.InstallDir, "bin", "disql")
	instance, err := e.handlers["dm"].runner(ctx, bin, damengConnection(req), "-e", "SELECT STATUS FROM V$INSTANCE")
	if err != nil {
		return nil, fmt.Errorf("collect dameng instance status: %w", err)
	}
	archive, err := e.handlers["dm"].runner(ctx, bin, damengConnection(req), "-e", "SELECT STATUS FROM V$ARCH_STATUS")
	if err != nil {
		return nil, fmt.Errorf("collect dameng archive status: %w", err)
	}
	return map[string]interface{}{"flavor": "dm", "instance_status": instance, "archive_status": archive}, nil
}

func (e *FlavorTaskExecutor) teardownDameng(ctx context.Context, req FlavorTaskRequest) error {
	if !req.Dameng.ConfirmUninstall || req.Dameng.Backup == nil || !damengBackupPath(req.Dameng.Backup.Destination) {
		return fmt.Errorf("dameng teardown requires confirm_uninstall and approved backup destination")
	}
	backupReq := req
	backupReq.Operation = "backup"
	if err := e.executeDamengLifecycle(ctx, backupReq); err != nil {
		return fmt.Errorf("back up dameng before teardown: %w", err)
	}
	if _, err := e.handlers["dm"].runner(ctx, filepath.Join(req.Dameng.InstallDir, "uninstall.sh"), "-i"); err != nil {
		return fmt.Errorf("uninstall dameng instance: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) executeDamengLifecycle(ctx context.Context, req FlavorTaskRequest) error {
	if err := validateDamengLifecycle(req); err != nil {
		return err
	}
	bin := filepath.Join(req.Dameng.InstallDir, "bin")
	ini := filepath.Join(req.Dameng.DataDir, "dm.ini")
	switch req.Operation {
	case "backup":
		if req.Dameng.Backup.Offline {
			if _, err := e.handlers["dm"].runner(ctx, filepath.Join(bin, "dmrman"), "CTLSTMT=\"BACKUP DATABASE '"+ini+"' FULL TO BACKUPSET '"+req.Dameng.Backup.Destination+"'\""); err != nil {
				return fmt.Errorf("create dameng offline backup: %w", err)
			}
			return nil
		}
		if _, err := e.handlers["dm"].runner(ctx, filepath.Join(bin, "disql"), damengConnection(req), "-e", "BACKUP DATABASE FULL BACKUPSET '"+req.Dameng.Backup.Destination+"'"); err != nil {
			return fmt.Errorf("create dameng online backup: %w", err)
		}
	case "restore":
		r := req.Dameng.Restore
		if _, err := e.handlers["dm"].runner(ctx, filepath.Join(bin, "dmrman"), "CTLSTMT=\"CHECK BACKUPSET '"+r.BackupSource+"'\""); err != nil {
			return fmt.Errorf("validate dameng backup set: %w", err)
		}
		if _, err := e.handlers["dm"].runner(ctx, filepath.Join(bin, "dmrman"), "CTLSTMT=\"RESTORE DATABASE '"+ini+"' FROM BACKUPSET '"+r.BackupSource+"'\""); err != nil {
			return fmt.Errorf("restore dameng database: %w", err)
		}
		if r.ArchiveSource != "" {
			if _, err := e.handlers["dm"].runner(ctx, filepath.Join(bin, "dmrman"), "CTLSTMT=\"RESTORE ARCHIVE LOG FROM BACKUPSET '"+r.ArchiveSource+"' TO ARCHIVEDIR '"+filepath.Join(req.Dameng.DataDir, "arch")+"'\""); err != nil {
				return fmt.Errorf("restore dameng archive logs: %w", err)
			}
			if _, err := e.handlers["dm"].runner(ctx, filepath.Join(bin, "dmrman"), "CTLSTMT=\"RECOVER DATABASE '"+ini+"' WITH ARCHIVEDIR '"+filepath.Join(req.Dameng.DataDir, "arch")+"' UPDATE DB_MAGIC\""); err != nil {
				return fmt.Errorf("recover dameng database with archive logs: %w", err)
			}
		}
	case "migrate":
		m := req.Dameng.Migration
		if _, err := e.handlers["dm"].runner(ctx, filepath.Join(bin, "dexp"), damengConnection(req), "FILE="+m.DumpFile); err != nil {
			return fmt.Errorf("export dameng logical backup: %w", err)
		}
		if _, err := e.handlers["dm"].runner(ctx, filepath.Join(bin, "dimp"), damengConnection(req), "FILE="+m.DumpFile); err != nil {
			return fmt.Errorf("import dameng logical backup: %w", err)
		}
	}
	return nil
}

func validateDamengLifecycle(req FlavorTaskRequest) error {
	switch req.Operation {
	case "backup":
		if req.Dameng.Backup == nil || !damengBackupPath(req.Dameng.Backup.Destination) {
			return fmt.Errorf("dameng backup destination must be under %s", damengBackupRoot)
		}
	case "restore":
		if req.Dameng.Restore == nil || !damengBackupPath(req.Dameng.Restore.BackupSource) || (req.Dameng.Restore.ArchiveSource != "" && !damengBackupPath(req.Dameng.Restore.ArchiveSource)) {
			return fmt.Errorf("dameng restore sources must be under %s", damengBackupRoot)
		}
	case "migrate":
		if req.Dameng.Migration == nil || !damengBackupPath(req.Dameng.Migration.DumpFile) {
			return fmt.Errorf("dameng migration dump must be under %s", damengBackupRoot)
		}
	}
	return nil
}

func damengBackupPath(path string) bool {
	return filepath.IsAbs(path) && strings.HasPrefix(filepath.Clean(path), damengBackupRoot+string(filepath.Separator)) && !strings.Contains(path, "..")
}

func damengConnection(req FlavorTaskRequest) string {
	return "SYSDBA/" + req.Dameng.SysdbaPassword + "@" + req.Dameng.Address + fmt.Sprintf(":%d", req.Dameng.Port)
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
	connection := damengConnection(req)
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
