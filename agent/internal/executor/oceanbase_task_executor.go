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

const (
	oceanBaseHome       = "/home/admin/oceanbase"
	oceanBaseObserver   = oceanBaseHome + "/bin/observer"
	oceanBaseOBClient   = oceanBaseHome + "/bin/obclient"
	oceanBaseOBProxy    = "/opt/taobao/install/obproxy/bin/obproxy"
	oceanBaseDefaultSQL = 2881
	oceanBaseDefaultRPC = 2882
	oceanBaseDefaultODP = 2883
)

var oceanBaseIdentifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,62}$`)
var oceanBaseSize = regexp.MustCompile(`^[1-9][0-9]*[KMGTP]$`)
var oceanBaseSHA1 = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)

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
