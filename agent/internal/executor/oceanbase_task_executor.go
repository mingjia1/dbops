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
	"time"
)

const (
	oceanBaseHome       = "/home/admin/oceanbase"
	oceanBaseObserver   = oceanBaseHome + "/bin/observer"
	oceanBaseOBClient   = oceanBaseHome + "/bin/obclient"
	oceanBaseOBProxy    = "/opt/taobao/install/obproxy/bin/obproxy"
	oceanBaseOBShell    = oceanBaseHome + "/bin/obshell"
	oceanBaseDefaultSQL = 2881
	oceanBaseDefaultRPC = 2882
	oceanBaseDefaultODP = 2883
)

var oceanBaseIdentifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,62}$`)
var oceanBaseSize = regexp.MustCompile(`^[1-9][0-9]*[KMGTP]$`)
var oceanBaseSHA1 = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
var oceanBaseChecksum = regexp.MustCompile(`^[a-fA-F0-9]{1,64}$`)

var oceanBaseParameters = map[string]bool{
	"enable_perf_event": true,
	"enable_sql_audit":  true,
	"syslog_level":      true,
}

func (e *FlavorTaskExecutor) executeOceanBase(ctx context.Context, req FlavorTaskRequest, manifest LocalPackageManifest) (*TaskResult, error) {
	if err := validateOceanBaseConfig(req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if e.handlers["oceanbase"].runner == nil || e.starter == nil {
		return flavorTaskFailure(req.TaskID, fmt.Errorf("oceanbase command runner is not configured")), nil
	}
	switch req.Operation {
	case "backup":
		if err := e.backupOceanBaseTenant(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "oceanbase backup completed", map[string]interface{}{"flavor": "oceanbase", "tenant": req.OceanBase.Tenant}), nil
	case "restore", "migrate":
		if err := e.restoreOceanBaseTenant(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "oceanbase "+req.Operation+" completed", map[string]interface{}{"flavor": "oceanbase", "tenant": req.OceanBase.Restore.Tenant}), nil
	case "upgrade":
		if err := e.backupOceanBaseTenant(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		if _, err := e.handlers["oceanbase"].runner(ctx, oceanBaseOBShell, "cluster", "upgrade", "-d", req.PackagePath, "-V", strings.TrimPrefix(req.Version, "v"), "-y"); err != nil {
			return flavorTaskFailure(req.TaskID, fmt.Errorf("upgrade oceanbase cluster: %w", err)), nil
		}
		if err := e.verifyOceanBaseDeployment(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		if err := e.verifyOceanBaseActiveReplicas(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		return flavorTaskCompleted(req.TaskID, "oceanbase upgrade completed", map[string]interface{}{"flavor": "oceanbase", "version": req.Version}), nil
	}
	if req.Operation == "deploy" {
		if err := e.installOceanBasePackages(ctx, req, manifest); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		if err := e.startOceanBaseObserver(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		if err := e.configureOceanBaseTenant(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
		if err := e.startOceanBaseProxy(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	}
	if err := e.applyOceanBaseParameters(ctx, req); err != nil {
		return flavorTaskFailure(req.TaskID, err), nil
	}
	if req.TLS.Enabled {
		if err := e.configureOceanBaseTLS(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	}
	if req.Operation == "deploy" {
		if err := e.verifyOceanBaseDeployment(ctx, req); err != nil {
			return flavorTaskFailure(req.TaskID, err), nil
		}
	}
	return flavorTaskCompleted(req.TaskID, "oceanbase "+req.Operation+" completed", map[string]interface{}{
		"flavor": "oceanbase", "cluster": req.OceanBase.ClusterName, "tenant": req.OceanBase.Tenant,
	}), nil
}

func validateOceanBaseConfig(req FlavorTaskRequest) error {
	c := req.OceanBase
	if c == nil {
		return fmt.Errorf("oceanbase configuration is required")
	}
	for _, value := range []string{c.ClusterName, c.Address, c.Zone, c.DataDir, c.Tenant, c.SystemMemory, c.DatafileSize, c.TenantMemory, c.TenantLogDiskSize} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("oceanbase required configuration is missing")
		}
	}
	for _, value := range []string{c.ClusterName, c.Zone, c.Tenant} {
		if !oceanBaseIdentifier.MatchString(value) {
			return fmt.Errorf("oceanbase identifier %q is invalid", value)
		}
	}
	if !filepath.IsAbs(c.DataDir) || strings.Contains(filepath.Clean(c.DataDir), "..") {
		return fmt.Errorf("oceanbase data_dir must be an absolute clean path")
	}
	for _, value := range []string{c.SystemMemory, c.DatafileSize, c.TenantMemory, c.TenantLogDiskSize} {
		if !oceanBaseSize.MatchString(value) {
			return fmt.Errorf("oceanbase size %q is invalid", value)
		}
	}
	if c.ClusterID <= 0 || c.TenantCPU <= 0 {
		return fmt.Errorf("oceanbase cluster_id and tenant_cpu must be positive")
	}
	for _, port := range []int{c.SQLPort, c.RPCPort} {
		if port != 0 && (port < 1024 || port > 65535) {
			return fmt.Errorf("oceanbase port %d is invalid", port)
		}
	}
	if c.EnableOBProxy && (c.OBProxyPort != 0 && (c.OBProxyPort < 1024 || c.OBProxyPort > 65535)) {
		return fmt.Errorf("oceanbase obproxy_port is invalid")
	}
	if c.OBProxySysPasswordSHA != "" && !oceanBaseSHA1.MatchString(c.OBProxySysPasswordSHA) {
		return fmt.Errorf("oceanbase obproxy_sys_password_sha1 is invalid")
	}
	for key := range c.Parameters {
		if !oceanBaseParameters[key] {
			return fmt.Errorf("oceanbase parameter %q is not allowed", key)
		}
	}
	if req.TLS.Enabled {
		for _, path := range []string{req.TLS.CAFile, req.TLS.CertFile, req.TLS.KeyFile} {
			if !filepath.IsAbs(path) || strings.Contains(filepath.Clean(path), "..") {
				return fmt.Errorf("oceanbase TLS paths must be absolute clean paths")
			}
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("oceanbase TLS file %q is unavailable", path)
			}
		}
	}
	switch req.Operation {
	case "backup", "upgrade":
		return validateOceanBaseBackupOperation(req)
	case "restore", "migrate":
		return validateOceanBaseRestoreOperation(req)
	}
	return nil
}

func validateOceanBaseBackupDestination(destination string) error {
	const prefix = "file:///opt/dbops/backups/oceanbase/"
	if !strings.HasPrefix(destination, prefix) || strings.Contains(strings.TrimPrefix(destination, prefix), "..") {
		return fmt.Errorf("oceanbase backup destination must remain under %s", prefix)
	}
	return nil
}

func validateOceanBaseBackupOperation(req FlavorTaskRequest) error {
	if req.OceanBase.Backup == nil {
		return fmt.Errorf("oceanbase backup configuration is required")
	}
	if timeout := req.OceanBase.Backup.TimeoutSeconds; timeout != 0 && (timeout < 60 || timeout > 3600) {
		return fmt.Errorf("oceanbase backup timeout is invalid")
	}
	return validateOceanBaseBackupDestination(req.OceanBase.Backup.Destination)
}

func validateOceanBaseRestoreOperation(req FlavorTaskRequest) error {
	c := req.OceanBase.Restore
	if c == nil {
		return fmt.Errorf("oceanbase restore configuration is required")
	}
	if !oceanBaseIdentifier.MatchString(c.Tenant) || !oceanBaseIdentifier.MatchString(c.ResourcePool) || c.Tenant == req.OceanBase.Tenant {
		return fmt.Errorf("oceanbase restore tenant and resource_pool must be distinct valid identifiers")
	}
	if err := validateOceanBaseBackupDestination(c.BackupSource); err != nil {
		return err
	}
	if c.UntilSCN < 0 || (c.TimeoutSeconds != 0 && (c.TimeoutSeconds < 60 || c.TimeoutSeconds > 3600)) {
		return fmt.Errorf("oceanbase restore timeout or SCN is invalid")
	}
	if len(c.ValidationTables) == 0 {
		return fmt.Errorf("oceanbase restore validation_tables are required")
	}
	for _, check := range c.ValidationTables {
		if !oceanBaseIdentifier.MatchString(check.Database) || !oceanBaseIdentifier.MatchString(check.Table) || check.ExpectedRowCount < 0 || !oceanBaseChecksum.MatchString(check.ExpectedChecksum) {
			return fmt.Errorf("oceanbase restore table validation is invalid")
		}
	}
	return nil
}

func (e *FlavorTaskExecutor) installOceanBasePackages(ctx context.Context, req FlavorTaskRequest, manifest LocalPackageManifest) error {
	args := []string{"-Uvh", "--replacepkgs"}
	for _, artifact := range manifest.Packages {
		if !strings.HasSuffix(strings.ToLower(artifact.File), ".rpm") {
			return fmt.Errorf("oceanbase package %q must be an RPM", artifact.File)
		}
		args = append(args, filepath.Join(req.PackagePath, artifact.File))
	}
	_, err := e.handlers["oceanbase"].runner(ctx, "rpm", args...)
	if err != nil {
		return fmt.Errorf("install oceanbase RPM: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) startOceanBaseObserver(ctx context.Context, req FlavorTaskRequest) error {
	c := req.OceanBase
	sqlPort, rpcPort := oceanBasePorts(c)
	args := []string{"-I", c.Address, "-P", strconv.Itoa(rpcPort), "-p", strconv.Itoa(sqlPort), "-z", c.Zone,
		"-d", filepath.Join(c.DataDir, c.ClusterName), "-r", fmt.Sprintf("%s:%d:%d", c.Address, rpcPort, sqlPort),
		"-c", strconv.Itoa(c.ClusterID), "-n", c.ClusterName,
		"-o", fmt.Sprintf("system_memory=%s,datafile_size=%s", c.SystemMemory, c.DatafileSize)}
	if err := e.starter(ctx, oceanBaseObserver, args...); err != nil {
		return fmt.Errorf("start oceanbase observer: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) configureOceanBaseTenant(ctx context.Context, req FlavorTaskRequest) error {
	c := req.OceanBase
	sqlPort, _ := oceanBasePorts(c)
	unit := c.Tenant + "_unit"
	pool := c.Tenant + "_pool"
	statements := []string{
		fmt.Sprintf("CREATE RESOURCE UNIT %s MAX_CPU %d, MIN_CPU %d, MEMORY_SIZE '%s', MAX_IOPS 10000, MIN_IOPS 10000, LOG_DISK_SIZE '%s'", unit, c.TenantCPU, c.TenantCPU, c.TenantMemory, c.TenantLogDiskSize),
		fmt.Sprintf("CREATE RESOURCE POOL %s UNIT '%s', UNIT_NUM 1, ZONE_LIST ('%s')", pool, unit, c.Zone),
		fmt.Sprintf("CREATE TENANT %s RESOURCE_POOL_LIST=('%s') PRIMARY_ZONE='RANDOM', LOCALITY='F@%s' SET ob_compatibility_mode='mysql'", c.Tenant, pool, c.Zone),
	}
	for _, statement := range statements {
		if err := e.runOceanBaseSQL(ctx, req, sqlPort, "root@sys#"+c.ClusterName, c.RootPassword, statement); err != nil {
			return fmt.Errorf("configure oceanbase tenant: %w", err)
		}
	}
	return nil
}

func (e *FlavorTaskExecutor) backupOceanBaseTenant(ctx context.Context, req FlavorTaskRequest) error {
	if err := validateOceanBaseBackupOperation(req); err != nil {
		return err
	}
	c := req.OceanBase
	sqlPort, _ := oceanBasePorts(c)
	sysUser := "root@sys#" + c.ClusterName
	if err := e.runOceanBaseSQL(ctx, req, sqlPort, sysUser, c.RootPassword, "ALTER SYSTEM SET DATA_BACKUP_DEST='"+c.Backup.Destination+"' TENANT="+c.Tenant); err != nil {
		return fmt.Errorf("configure oceanbase backup destination: %w", err)
	}
	tenantUser := "root@" + c.Tenant + "#" + c.ClusterName
	if err := e.runOceanBaseSQL(ctx, req, sqlPort, tenantUser, c.RootPassword, "ALTER SYSTEM BACKUP DATABASE PLUS ARCHIVELOG"); err != nil {
		return fmt.Errorf("start oceanbase tenant backup: %w", err)
	}
	return e.waitForOceanBaseBackup(ctx, req)
}

func (e *FlavorTaskExecutor) waitForOceanBaseBackup(ctx context.Context, req FlavorTaskRequest) error {
	timeout := req.OceanBase.Backup.TimeoutSeconds
	if timeout == 0 {
		timeout = 900
	}
	deadline := time.NewTimer(time.Duration(timeout) * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	sqlPort, _ := oceanBasePorts(req.OceanBase)
	for {
		output, err := e.queryOceanBaseSQL(ctx, req, sqlPort, "root@sys#"+req.OceanBase.ClusterName, req.OceanBase.RootPassword, "SELECT STATUS FROM oceanbase.CDB_OB_BACKUP_JOBS")
		if err != nil {
			return fmt.Errorf("query oceanbase backup status: %w", err)
		}
		switch {
		case strings.Contains(output, "COMPLETED"):
			return nil
		case strings.Contains(output, "FAILED"), strings.Contains(output, "CANCELED"):
			return fmt.Errorf("oceanbase backup reported a terminal failure")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("oceanbase backup did not complete within %d seconds", timeout)
		case <-ticker.C:
		}
	}
}

func (e *FlavorTaskExecutor) restoreOceanBaseTenant(ctx context.Context, req FlavorTaskRequest) error {
	if err := validateOceanBaseRestoreOperation(req); err != nil {
		return err
	}
	c, restore := req.OceanBase, req.OceanBase.Restore
	sqlPort, _ := oceanBasePorts(c)
	statement := "ALTER SYSTEM RESTORE " + restore.Tenant + " FROM '" + restore.BackupSource + "'"
	if restore.UntilSCN > 0 {
		statement += " UNTIL SCN=" + strconv.FormatInt(restore.UntilSCN, 10)
	}
	statement += " WITH 'pool_list=" + restore.ResourcePool + "&method=full'"
	if err := e.runOceanBaseSQL(ctx, req, sqlPort, "root@sys#"+c.ClusterName, c.RootPassword, statement); err != nil {
		return fmt.Errorf("start oceanbase tenant restore: %w", err)
	}
	if err := e.waitForOceanBaseRestore(ctx, req); err != nil {
		return err
	}
	return e.validateOceanBaseRestoredTenant(ctx, req)
}

func (e *FlavorTaskExecutor) waitForOceanBaseRestore(ctx context.Context, req FlavorTaskRequest) error {
	timeout := req.OceanBase.Restore.TimeoutSeconds
	if timeout == 0 {
		timeout = 900
	}
	deadline := time.NewTimer(time.Duration(timeout) * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		sqlPort, _ := oceanBasePorts(req.OceanBase)
		output, err := e.queryOceanBaseSQL(ctx, req, sqlPort, "root@sys#"+req.OceanBase.ClusterName, req.OceanBase.RootPassword, "SELECT STATUS FROM oceanbase.CDB_OB_RESTORE_PROGRESS")
		if err != nil {
			return fmt.Errorf("query oceanbase restore progress: %w", err)
		}
		switch {
		case strings.Contains(output, "RESTORE_SUCCESS"):
			return nil
		case strings.Contains(output, "RESTORE_FAIL"):
			return fmt.Errorf("oceanbase restore reported RESTORE_FAIL")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("oceanbase restore did not complete within %d seconds", timeout)
		case <-ticker.C:
		}
	}
}

func (e *FlavorTaskExecutor) validateOceanBaseRestoredTenant(ctx context.Context, req FlavorTaskRequest) error {
	c, restore := req.OceanBase, req.OceanBase.Restore
	sqlPort, _ := oceanBasePorts(c)
	user := "root@" + restore.Tenant + "#" + c.ClusterName
	for _, check := range restore.ValidationTables {
		name := check.Database + "." + check.Table
		output, err := e.queryOceanBaseSQL(ctx, req, sqlPort, user, c.RootPassword, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='"+check.Database+"' AND table_name='"+check.Table+"'")
		if err != nil {
			return fmt.Errorf("verify restored oceanbase schema for %s: %w", name, err)
		}
		if !oceanBaseOutputContainsField(output, "1") {
			return fmt.Errorf("restored oceanbase schema is missing %s", name)
		}
		output, err = e.queryOceanBaseSQL(ctx, req, sqlPort, user, c.RootPassword, "SELECT COUNT(*) FROM "+name)
		if err != nil {
			return fmt.Errorf("verify restored oceanbase row count for %s: %w", name, err)
		}
		if !oceanBaseOutputContainsField(output, strconv.FormatInt(check.ExpectedRowCount, 10)) {
			return fmt.Errorf("restored oceanbase row count for %s does not match", name)
		}
		output, err = e.queryOceanBaseSQL(ctx, req, sqlPort, user, c.RootPassword, "CHECKSUM TABLE "+name)
		if err != nil {
			return fmt.Errorf("verify restored oceanbase checksum for %s: %w", name, err)
		}
		if !oceanBaseOutputContainsField(strings.ToLower(output), strings.ToLower(check.ExpectedChecksum)) {
			return fmt.Errorf("restored oceanbase checksum for %s does not match", name)
		}
	}
	return nil
}

func oceanBaseOutputContainsField(output, expected string) bool {
	for _, field := range strings.Fields(output) {
		if field == expected {
			return true
		}
	}
	return false
}

func (e *FlavorTaskExecutor) verifyOceanBaseActiveReplicas(ctx context.Context, req FlavorTaskRequest) error {
	sqlPort, _ := oceanBasePorts(req.OceanBase)
	output, err := e.queryOceanBaseSQL(ctx, req, sqlPort, "root@sys#"+req.OceanBase.ClusterName, req.OceanBase.RootPassword, "SELECT STATUS FROM oceanbase.DBA_OB_SERVERS")
	if err != nil {
		return fmt.Errorf("verify oceanbase replica state: %w", err)
	}
	if !oceanBaseOutputContainsField(output, "ACTIVE") {
		return fmt.Errorf("oceanbase replicas are not ACTIVE")
	}
	return nil
}

func (e *FlavorTaskExecutor) applyOceanBaseParameters(ctx context.Context, req FlavorTaskRequest) error {
	sqlPort, _ := oceanBasePorts(req.OceanBase)
	for _, key := range oceanBaseParameterKeys(req.OceanBase.Parameters) {
		value := req.OceanBase.Parameters[key]
		if err := e.runOceanBaseSQL(ctx, req, sqlPort, "root@sys#"+req.OceanBase.ClusterName, req.OceanBase.RootPassword, fmt.Sprintf("ALTER SYSTEM SET %s = '%s'", key, strings.ReplaceAll(value, "'", "''"))); err != nil {
			return fmt.Errorf("set oceanbase parameter %s: %w", key, err)
		}
	}
	return nil
}

func (e *FlavorTaskExecutor) startOceanBaseProxy(ctx context.Context, req FlavorTaskRequest) error {
	c := req.OceanBase
	if !c.EnableOBProxy {
		return nil
	}
	sqlPort, _ := oceanBasePorts(c)
	port := c.OBProxyPort
	if port == 0 {
		port = oceanBaseDefaultODP
	}
	options := "enable_strict_kernel_release=false,enable_cluster_checkout=false,enable_metadb_used=false"
	if c.OBProxySysPasswordSHA != "" {
		options = "observer_sys_password=" + c.OBProxySysPasswordSHA + "," + options
	}
	if err := e.starter(ctx, oceanBaseOBProxy, "-r", fmt.Sprintf("%s:%d", c.Address, sqlPort), "-p", strconv.Itoa(port), "-o", options, "-c", c.Tenant); err != nil {
		return fmt.Errorf("start oceanbase obproxy: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) configureOceanBaseTLS(ctx context.Context, req FlavorTaskRequest) error {
	c := req.OceanBase
	user := "root@sys#" + c.ClusterName
	sqlPort, _ := oceanBasePorts(c)
	if err := e.runOceanBaseSQL(ctx, req, sqlPort, user, c.RootPassword, `ALTER SYSTEM SET ssl_external_kms_info = '{"ssl_mode":"file"}'`); err != nil {
		return fmt.Errorf("configure oceanbase TLS file mode: %w", err)
	}
	if err := e.runOceanBaseSQL(ctx, req, sqlPort, user, c.RootPassword, "ALTER SYSTEM SET ssl_client_authentication=True"); err != nil {
		return fmt.Errorf("enable oceanbase TLS: %w", err)
	}
	if !c.EnableOBProxy {
		return nil
	}
	port := c.OBProxyPort
	if port == 0 {
		port = oceanBaseDefaultODP
	}
	value := fmt.Sprintf(`{"sourceType":"FILE","CA":"%s","publicKey":"%s","privateKey":"%s"}`, req.TLS.CAFile, req.TLS.CertFile, req.TLS.KeyFile)
	statement := "UPDATE proxyconfig.security_config SET CONFIG_VAL='" + strings.ReplaceAll(value, "'", "''") + "' WHERE APP_NAME='obproxy' AND VERSION='1'"
	if err := e.runOceanBaseSQL(ctx, req, port, "root@proxysys", c.RootPassword, statement); err != nil {
		return fmt.Errorf("configure obproxy TLS: %w", err)
	}
	return nil
}

func (e *FlavorTaskExecutor) verifyOceanBaseDeployment(ctx context.Context, req FlavorTaskRequest) error {
	sqlPort, _ := oceanBasePorts(req.OceanBase)
	user := "root@sys#" + req.OceanBase.ClusterName
	checks := []string{
		"SELECT VERSION()",
		"SHOW OB SERVERS",
		"SELECT tenant_name FROM oceanbase.DBA_OB_TENANTS WHERE tenant_name='" + req.OceanBase.Tenant + "'",
	}
	for _, key := range oceanBaseParameterKeys(req.OceanBase.Parameters) {
		checks = append(checks, "SHOW PARAMETERS LIKE '"+key+"'")
	}
	for _, statement := range checks {
		output, err := e.queryOceanBaseSQL(ctx, req, sqlPort, user, req.OceanBase.RootPassword, statement)
		if err != nil {
			return fmt.Errorf("verify oceanbase deployment: %w", err)
		}
		if strings.TrimSpace(output) == "" {
			return fmt.Errorf("verify oceanbase deployment returned empty output for %q", statement)
		}
	}
	return nil
}

func (e *FlavorTaskExecutor) runOceanBaseSQL(ctx context.Context, req FlavorTaskRequest, port int, user, password, statement string) error {
	_, err := e.queryOceanBaseSQL(ctx, req, port, user, password, statement)
	return err
}

func (e *FlavorTaskExecutor) queryOceanBaseSQL(ctx context.Context, req FlavorTaskRequest, port int, user, password, statement string) (string, error) {
	return e.handlers["oceanbase"].runner(ctx, oceanBaseOBClient, "-h", req.OceanBase.Address, "-P", strconv.Itoa(port), "-u"+user, "-p"+password, "-e", statement)
}

func oceanBasePorts(config *OceanBaseConfig) (int, int) {
	sqlPort, rpcPort := config.SQLPort, config.RPCPort
	if sqlPort == 0 {
		sqlPort = oceanBaseDefaultSQL
	}
	if rpcPort == 0 {
		rpcPort = oceanBaseDefaultRPC
	}
	return sqlPort, rpcPort
}

func oceanBaseParameterKeys(parameters map[string]string) []string {
	keys := make([]string, 0, len(parameters))
	for key := range parameters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
