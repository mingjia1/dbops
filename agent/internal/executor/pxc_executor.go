package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type PXCConfig struct {
	ClusterName     string   `json:"cluster_name"`
	Nodes           []string `json:"nodes"`
	MySQLPort       int      `json:"mysql_port"`
	WSREPPort       int      `json:"wsrep_port"`
	SSTMethod       string   `json:"sst_method"`
	ReplicateUser   string   `json:"replicate_user"`
	ReplicatePass   string   `json:"replicate_pass"`
	DataDir         string   `json:"data_dir"`
	Bootstrap       bool     `json:"bootstrap"`
	WSREPSSTPort    int      `json:"wsrep_sst_port"`
	WSREPSSLEnabled bool     `json:"wsrep_ssl_enabled"`
	MySQLUser       string   `json:"mysql_user"`
	MySQLPassword   string   `json:"mysql_password"`
	NodeHost        string   `json:"node_host"`
}

type GaleraConfig struct {
	WSREPClusterAddress string `json:"wsrep_cluster_address"`
	WSREPNodeAddress    string `json:"wsrep_node_address"`
	WSREPNodeName       string `json:"wsrep_node_name"`
	WSREPSSTMethod      string `json:"wsrep_sst_method"`
	WSREPProvider       string `json:"wsrep_provider"`
	GCacheSize          string `json:"gcache_size"`
	GCachePageSize      string `json:"gcache_page_size"`
	GCacheKeepPagesSize string `json:"gcache_keep_pages_size"`
}

type PXCSyncStatus struct {
	ClusterSize       int    `json:"cluster_size"`
	ClusterStatus     string `json:"cluster_status"`
	LocalIndex        int    `json:"local_index"`
	LocalState        string `json:"local_state"`
	Ready             bool   `json:"ready"`
	FlowControlPaused bool   `json:"flow_control_paused"`
	InnoDBBufferPool  string `json:"innodb_buffer_pool"`
}

type SplitBrainInfo struct {
	Detected         bool     `json:"detected"`
	IsolatedNodes    []string `json:"isolated_nodes"`
	ActiveNodes      []string `json:"active_nodes"`
	QuorumStatus     string   `json:"quorum_status"`
	PrimaryComponent bool     `json:"primary_component"`
}

func parsePXCConfig(config map[string]interface{}) PXCConfig {
	pc := PXCConfig{
		ClusterName:     "pxc-cluster",
		MySQLPort:       3306,
		WSREPPort:       4567,
		SSTMethod:       "xtrabackup-v2",
		ReplicateUser:   "sstuser",
		ReplicatePass:   "sstpass",
		DataDir:         "/var/lib/mysql",
		Bootstrap:       false,
		WSREPSSTPort:    4444,
		WSREPSSLEnabled: false,
		MySQLUser:       "root",
	}

	if v, ok := config["cluster_name"].(string); ok {
		pc.ClusterName = v
	}
	if v, ok := config["nodes"].([]string); ok {
		pc.Nodes = v
	} else if nodes, ok := config["nodes"].([]interface{}); ok {
		for _, node := range nodes {
			if s, ok := node.(string); ok {
				pc.Nodes = append(pc.Nodes, s)
			}
		}
	}
	if v := configInt(config, "wsrep_port"); v != 0 {
		pc.WSREPPort = v
	}
	if v := configInt(config, "mysql_port"); v != 0 {
		pc.MySQLPort = v
	}
	if v, ok := config["sst_method"].(string); ok {
		pc.SSTMethod = v
	}
	if v, ok := config["replicate_user"].(string); ok {
		pc.ReplicateUser = v
	}
	if v, ok := config["replicate_pass"].(string); ok {
		pc.ReplicatePass = v
	}
	if v, ok := config["data_dir"].(string); ok {
		pc.DataDir = v
	}
	if v, ok := config["bootstrap"].(bool); ok {
		pc.Bootstrap = v
	}
	if v := configInt(config, "wsrep_sst_port"); v != 0 {
		pc.WSREPSSTPort = v
	}
	if v, ok := config["wsrep_ssl_enabled"].(bool); ok {
		pc.WSREPSSLEnabled = v
	}
	if v, ok := config["mysql_user"].(string); ok {
		pc.MySQLUser = v
	}
	if v, ok := config["mysql_password"].(string); ok {
		pc.MySQLPassword = v
	}
	if v, ok := config["node_host"].(string); ok {
		pc.NodeHost = v
	}

	return pc
}

func parseGaleraConfig(config map[string]interface{}) GaleraConfig {
	gc := GaleraConfig{
		WSREPSSTMethod:      "xtrabackup-v2",
		WSREPProvider:       "/usr/lib/galera4/libgalera_smm.so",
		GCacheSize:          "1G",
		GCachePageSize:      "1G",
		GCacheKeepPagesSize: "0",
	}

	if v, ok := config["wsrep_cluster_address"].(string); ok {
		gc.WSREPClusterAddress = v
	}
	if v, ok := config["wsrep_node_address"].(string); ok {
		gc.WSREPNodeAddress = v
	}
	if v, ok := config["wsrep_node_name"].(string); ok {
		gc.WSREPNodeName = v
	}
	if v, ok := config["wsrep_sst_method"].(string); ok {
		gc.WSREPSSTMethod = v
	}
	if v, ok := config["wsrep_provider"].(string); ok {
		gc.WSREPProvider = v
	}
	if v, ok := config["gcache_size"].(string); ok {
		gc.GCacheSize = v
	}
	if v, ok := config["gcache_page_size"].(string); ok {
		gc.GCachePageSize = v
	}
	if v, ok := config["gcache_keep_pages_size"].(string); ok {
		gc.GCacheKeepPagesSize = v
	}

	return gc
}

func (e *TaskExecutor) DeployPXC(ctx context.Context, req DeployTaskRequest) (*TaskResult, error) {
	config := parsePXCConfig(req.Config)
	config.ReplicatePass = pxcOptionSafePassword(config.ReplicatePass)

	if len(config.Nodes) == 0 {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   "PXC cluster requires at least one node",
			Timestamp: time.Now(),
		}, nil
	}
	if config.NodeHost == "" {
		config.NodeHost = config.Nodes[0]
	}
	if config.MySQLPassword == "" {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  0,
			Message:   "PXC deployment requires mysql_password",
			Timestamp: time.Now(),
		}, nil
	}

	if err := installPXCSystemPackage(ctx); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  20,
			Message:   fmt.Sprintf("PXC package installation failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	if err := stopStandalonePXC(ctx, config); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  30,
			Message:   fmt.Sprintf("Failed to stop existing standalone PXC process: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	if err := preparePXCDataDir(ctx, config); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  35,
			Message:   fmt.Sprintf("PXC data directory preparation failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	if err := writePXCConfig(ctx, config); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  45,
			Message:   fmt.Sprintf("PXC configuration failed: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	if config.Bootstrap {
		if err := initializePXCBootstrapDataDir(ctx, config); err != nil {
			return &TaskResult{
				TaskID:    req.TaskID,
				Status:    "failed",
				Progress:  55,
				Message:   fmt.Sprintf("Failed to initialize PXC bootstrap datadir %s: %v", config.DataDir, err),
				Timestamp: time.Now(),
			}, nil
		}
	}
	if err := startStandalonePXC(ctx, config); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("Failed to start PXC node %s: %v", config.NodeHost, err),
			Timestamp: time.Now(),
		}, nil
	}

	if err := waitForPXCMySQL(ctx, config); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  70,
			Message:   fmt.Sprintf("PXC mysqld did not become reachable on %s:%d: %v", config.NodeHost, config.MySQLPort, err),
			Timestamp: time.Now(),
		}, nil
	}

	sstUserSQL := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED BY '%s'; "+
			"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'; "+
			"GRANT RELOAD, LOCK TABLES, PROCESS, REPLICATION CLIENT ON *.* TO '%s'@'localhost'; "+
			"GRANT RELOAD, LOCK TABLES, PROCESS, REPLICATION CLIENT ON *.* TO '%s'@'%%'; FLUSH PRIVILEGES;",
		config.ReplicateUser, config.ReplicatePass, config.ReplicateUser, config.ReplicatePass, config.ReplicateUser, config.ReplicateUser,
	)

	nodeCmd := pxcMysqlExecCommand(ctx, "127.0.0.1", config.MySQLPort, config.MySQLUser, config.MySQLPassword, sstUserSQL)
	if out, err := nodeCmd.CombinedOutput(); err != nil {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("Failed to create SST user on node %s: %v, output: %s", config.NodeHost, err, strings.TrimSpace(string(out))),
			Timestamp: time.Now(),
		}, nil
	}

	statusResult := e.monitorPXCSyncWithAuth(ctx, "127.0.0.1", config.MySQLPort, config.MySQLUser, config.MySQLPassword)
	if statusResult.ClusterStatus != "Primary" || !statusResult.Ready {
		return &TaskResult{
			TaskID:    req.TaskID,
			Status:    "failed",
			Progress:  90,
			Message:   fmt.Sprintf("PXC cluster sync check failed: cluster_status=%s, ready=%v", statusResult.ClusterStatus, statusResult.Ready),
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:    req.TaskID,
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("PXC node %s joined cluster '%s' status=%s size=%d", config.NodeHost, config.ClusterName, statusResult.ClusterStatus, statusResult.ClusterSize),
		Timestamp: time.Now(),
	}, nil
}

func installPXCSystemPackage(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "bash", "-lc", `
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
export NEEDRESTART_MODE=a
export NEEDRESTART_SUSPEND=1
if [ -x /opt/dbops-pxc/usr/sbin/mysqld ] && [ -f /opt/dbops-pxc/usr/lib/galera4/libgalera_smm.so ] && command -v mysql >/dev/null 2>&1 && command -v xtrabackup >/dev/null 2>&1; then
  echo "PXC payload and client tools are already installed; skipping package installation"
  exit 0
fi
cleanup_policy_rcd() { rm -f /usr/sbin/policy-rc.d; }
restore_mysql_repo_lists() {
  if [ -d /tmp/dbops-disabled-mysql-apt-lists ]; then
    find /tmp/dbops-disabled-mysql-apt-lists -type f -name '*.list' -print0 2>/dev/null | while IFS= read -r -d '' file; do
      rel=${file#/tmp/dbops-disabled-mysql-apt-lists/}
      mkdir -p "/$(dirname "$rel")"
      mv "$file" "/$rel"
    done
    rm -rf /tmp/dbops-disabled-mysql-apt-lists
  fi
}
disable_expired_mysql_repo_lists() {
  mkdir -p /tmp/dbops-disabled-mysql-apt-lists
  find /etc/apt -type f -name '*.list' -print0 2>/dev/null | while IFS= read -r -d '' file; do
    if grep -q 'repo\.mysql\.com' "$file"; then
      target="/tmp/dbops-disabled-mysql-apt-lists${file}"
      mkdir -p "$(dirname "$target")"
      mv "$file" "$target"
      echo "temporarily disabled apt source with expired MySQL repo signature: $file"
    fi
  done
}
cleanup() {
  restore_mysql_repo_lists
  cleanup_policy_rcd
}
trap cleanup EXIT
cat > /usr/sbin/policy-rc.d <<'EOF_POLICY'
#!/bin/sh
exit 101
EOF_POLICY
chmod +x /usr/sbin/policy-rc.d
wait_dpkg_lock() {
  for i in $(seq 1 120); do
    if ! fuser /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/cache/apt/archives/lock >/dev/null 2>&1; then
      return 0
    fi
    echo "waiting for apt/dpkg lock ($i/120)"
    fuser -v /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/cache/apt/archives/lock 2>&1 || true
    sleep 5
  done
  echo "apt/dpkg lock still held after waiting; attempting stale apt/dpkg cleanup"
  ps -eo pid,etimes,comm,args | awk '$2 > 900 && ($3 ~ /^(apt|apt-get|dpkg|dpkg-deb|unattended-upgrade)$/) {print $1}' | xargs -r kill -TERM
  sleep 10
  ps -eo pid,etimes,comm,args | awk '$2 > 900 && ($3 ~ /^(apt|apt-get|dpkg|dpkg-deb|unattended-upgrade)$/) {print $1}' | xargs -r kill -KILL
  if fuser /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/cache/apt/archives/lock >/dev/null 2>&1; then
    echo "apt/dpkg lock still held after stale cleanup"
    fuser -v /var/lib/dpkg/lock-frontend /var/lib/dpkg/lock /var/cache/apt/archives/lock 2>&1 || true
    return 1
  fi
  dpkg --configure -a || true
  return 0
}
apt_install_base_tools() {
  apt-get -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install -y wget gnupg2 lsb-release curl debconf-utils || true
  if ! command -v wget >/dev/null 2>&1 && [ ! -x /usr/bin/wget ]; then
    echo "required command wget is missing after apt install"
    return 1
  fi
  if ! command -v curl >/dev/null 2>&1 && [ ! -x /usr/bin/curl ]; then
    echo "required command curl is missing after apt install"
    return 1
  fi
  echo "base apt tools are ready"
}
apt_install_pxc_tools() {
  apt-get -o Dpkg::Options::="--force-overwrite" install -y percona-xtrabackup-80 qpress socat || {
    command -v xtrabackup >/dev/null 2>&1 &&
    command -v qpress >/dev/null 2>&1 &&
    command -v socat >/dev/null 2>&1
  }
}
download_pxc_payload() {
  echo "Extracting PXC common/client/server payload without running package maintainer scripts"
  mkdir -p /opt/dbops-pxc /tmp/dbops-pxc-debs
  cd /tmp/dbops-pxc-debs
  versions=$(apt-cache madison percona-xtradb-cluster-server | awk '{print $3}')
  if [ -z "$versions" ]; then
    versions=$(apt-cache policy percona-xtradb-cluster-server | awk '/^[[:space:]]+[0-9]+:/{print $1}')
  fi
  if [ -z "$versions" ]; then
    echo "no PXC server package versions found in configured apt sources"
    return 1
  fi
  downloaded=0
  for version in $versions; do
    echo "trying PXC package version $version"
    rm -f percona-xtradb-cluster-*.deb
    if apt-get -o Acquire::Retries=3 download \
      percona-xtradb-cluster-common="$version" \
      percona-xtradb-cluster-client="$version" \
      percona-xtradb-cluster-server="$version"; then
      downloaded=1
      break
    fi
    echo "PXC package version $version download failed; trying next available version"
    sleep 3
  done
  if [ "$downloaded" != "1" ]; then
    echo "failed to download any matching PXC common/client/server package set"
    return 1
  fi
  for deb in percona-xtradb-cluster-*.deb; do
    dpkg-deb -x "$deb" /opt/dbops-pxc
  done
  test -x /opt/dbops-pxc/usr/sbin/mysqld
  test -x /opt/dbops-pxc/usr/bin/mysql
  test -x /opt/dbops-pxc/usr/bin/mysqladmin
  test -f /opt/dbops-pxc/usr/lib/galera4/libgalera_smm.so
}
show_pxc_apt_packages() {
  echo "PXC packages from configured apt sources:"
  apt-cache policy percona-xtradb-cluster percona-xtradb-cluster-common percona-xtradb-cluster-client percona-xtradb-cluster-server percona-xtrabackup-80 qpress socat || true
  apt-cache search '^percona-xtradb-cluster' || true
}
wait_dpkg_lock
disable_expired_mysql_repo_lists
apt-get update
wait_dpkg_lock
dpkg --remove --force-depends --force-remove-reinstreq percona-xtradb-cluster percona-xtradb-cluster-server >/dev/null 2>&1 || true
dpkg --configure -a || true
apt-get -o Dpkg::Options::="--force-overwrite" -f install -y
wait_dpkg_lock
apt_install_base_tools
if command -v percona-release >/dev/null 2>&1; then
  percona-release setup -y pxc80 || true
else
  echo "percona-release is not installed; using existing apt sources for PXC packages"
fi
wait_dpkg_lock
apt-get update
show_pxc_apt_packages
wait_dpkg_lock
dpkg --remove --force-depends --force-remove-reinstreq percona-xtradb-cluster percona-xtradb-cluster-server >/dev/null 2>&1 || true
dpkg --configure -a || true
apt-get -o Dpkg::Options::="--force-overwrite" -f install -y
wait_dpkg_lock
apt_install_pxc_tools
download_pxc_payload
`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func writePXCConfig(ctx context.Context, config PXCConfig) error {
	clusterAddress := "gcomm://" + strings.Join(config.Nodes, ",")
	serverID := 1000 + config.MySQLPort%50000 + len(config.NodeHost)
	pluginDir := pxcPluginDir()
	pluginDirLine := ""
	if pluginDir != "" {
		pluginDirLine = fmt.Sprintf("plugin-dir=%s\n", pluginDir)
	}
	providerPath := pxcProviderPath()
	content := fmt.Sprintf(`[mysqld]
server_id=%d
port=%d
bind-address=0.0.0.0
datadir=%s
socket=%s/mysql.sock
pid-file=%s/mysql.pid
log-error=%s
%s
log-bin=mysql-bin
mysqlx=0
binlog_format=ROW
default_storage_engine=InnoDB
innodb_autoinc_lock_mode=2
log_slave_updates
pxc_strict_mode=PERMISSIVE
wsrep_provider=%s
wsrep_cluster_name=%s
wsrep_cluster_address=%s
wsrep_node_address=%s
wsrep_node_name=%s-%s
wsrep_sst_method=%s
wsrep_sst_auth="%s:%s"
`, serverID, config.MySQLPort, config.DataDir, config.DataDir, config.DataDir, pxcErrorLogPath(config), pluginDirLine, providerPath, config.ClusterName, clusterAddress, config.NodeHost, config.ClusterName, safeNodeName(config.NodeHost), defaultString(config.SSTMethod, "xtrabackup-v2"), config.ReplicateUser, config.ReplicatePass)
	cmd := exec.CommandContext(ctx, "bash", "-lc", "mkdir -p /etc/dbops-pxc /var/log/dbops-pxc && cat > "+shellQuote(pxcConfigPath(config))+" <<'EOF'\n"+content+"EOF\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func safeNodeName(host string) string {
	replacer := strings.NewReplacer(".", "-", ":", "-")
	return replacer.Replace(host)
}

func preparePXCDataDir(ctx context.Context, config PXCConfig) error {
	cmd := exec.CommandContext(ctx, "bash", "-lc", fmt.Sprintf(`
set -euo pipefail
datadir=%s
case "$datadir" in
  /data/mysql/pxc-*|/tmp/dbops-pxc*) ;;
  *) echo "refuse to prepare unsafe PXC datadir: $datadir"; exit 1 ;;
esac
mkdir -p "$datadir" /var/log/dbops-pxc /etc/dbops-pxc
find "$datadir" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
chown -R mysql:mysql "$datadir" /var/log/dbops-pxc
`, shellQuote(config.DataDir)))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func initializePXCBootstrapDataDir(ctx context.Context, config PXCConfig) error {
	clean := exec.CommandContext(ctx, "bash", "-lc", fmt.Sprintf(`
set -euo pipefail
datadir=%s
case "$datadir" in
  /data/mysql/pxc-*|/tmp/dbops-pxc*) ;;
  *) echo "refuse to clean unsafe PXC datadir: $datadir"; exit 1 ;;
esac
if [ -d "$datadir/mysql" ] && [ ! -f "$datadir/mysql.ibd" ]; then
  find "$datadir" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
fi
`, shellQuote(config.DataDir)))
	if out, err := clean.CombinedOutput(); err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	check := exec.CommandContext(ctx, "bash", "-lc", "test -f "+shellQuote(filepathJoin(config.DataDir, "mysql.ibd")))
	if err := check.Run(); err == nil {
		return nil
	}
	if err := writePXCInitConfig(ctx, config); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "bash", "-lc", fmt.Sprintf("%s --defaults-file=%s --initialize-insecure --user=mysql", shellQuote(pxcMysqldPath()), shellQuote(pxcInitConfigPath(config))))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func writePXCInitConfig(ctx context.Context, config PXCConfig) error {
	pluginDirLine := ""
	if pluginDir := pxcPluginDir(); pluginDir != "" {
		pluginDirLine = fmt.Sprintf("plugin-dir=%s\n", pluginDir)
	}
	content := fmt.Sprintf(`[mysqld]
datadir=%s
socket=%s/mysql.sock
pid-file=%s/mysql.pid
log-error=%s
%smysqlx=0
`, config.DataDir, config.DataDir, config.DataDir, pxcErrorLogPath(config), pluginDirLine)
	cmd := exec.CommandContext(ctx, "bash", "-lc", "mkdir -p /etc/dbops-pxc /var/log/dbops-pxc && cat > "+shellQuote(pxcInitConfigPath(config))+" <<'EOF'\n"+content+"EOF\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func stopStandalonePXC(ctx context.Context, config PXCConfig) error {
	cmd := exec.CommandContext(ctx, "bash", "-lc", fmt.Sprintf(`
pid_file=%s
if [ -f "$pid_file" ]; then
  pid=$(cat "$pid_file" 2>/dev/null || true)
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    for i in $(seq 1 30); do kill -0 "$pid" 2>/dev/null || exit 0; sleep 1; done
    kill -KILL "$pid" 2>/dev/null || true
  fi
  rm -f "$pid_file"
fi
`, shellQuote(filepathJoin(config.DataDir, "mysql.pid"))))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func startStandalonePXC(ctx context.Context, config PXCConfig) error {
	extra := ""
	if config.Bootstrap {
		extra = " --wsrep-new-cluster"
	}
	cmd := exec.CommandContext(ctx, "bash", "-lc", fmt.Sprintf("nohup %s --defaults-file=%s --user=mysql%s >> %s 2>&1 &", shellQuote(pxcMysqldPath()), shellQuote(pxcConfigPath(config)), extra, shellQuote(pxcStartupLogPath(config))))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func waitForPXCMySQL(ctx context.Context, config PXCConfig) error {
	socket := filepathJoin(config.DataDir, "mysql.sock")
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		for _, pass := range []string{config.MySQLPassword, ""} {
			pingCmd := exec.CommandContext(ctx, pxcMysqladminPath(), "--socket", socket, "-u", "root", "ping")
			if pass != "" {
				pingCmd.Env = append(os.Environ(), "MYSQL_PWD="+pass)
			}
			if out, err := pingCmd.CombinedOutput(); err == nil && strings.Contains(string(out), "mysqld is alive") {
				if pass == "" && config.MySQLPassword != "" {
					_ = setPXCRootPassword(ctx, config)
				}
				return nil
			}
		}
		if config.MySQLPassword != "" {
			_ = setPXCRootPassword(ctx, config)
			pingCmd := exec.CommandContext(ctx, pxcMysqladminPath(), "-h", "127.0.0.1", "-P", strconv.Itoa(config.MySQLPort), "-u", "root", "ping")
			pingCmd.Env = append(os.Environ(), "MYSQL_PWD="+config.MySQLPassword)
			if out, err := pingCmd.CombinedOutput(); err == nil && strings.Contains(string(out), "mysqld is alive") {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	return fmt.Errorf("timeout waiting for mysqld on port %d", config.MySQLPort)
}

func setPXCRootPassword(ctx context.Context, config PXCConfig) error {
	sql := fmt.Sprintf("ALTER USER 'root'@'localhost' IDENTIFIED BY '%s'; CREATE USER IF NOT EXISTS 'root'@'%%' IDENTIFIED BY '%s'; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%%' WITH GRANT OPTION; FLUSH PRIVILEGES;", escapeSQL(config.MySQLPassword), escapeSQL(config.MySQLPassword))
	cmd := exec.CommandContext(ctx, pxcMysqlPath(), "--socket", filepathJoin(config.DataDir, "mysql.sock"), "-u", "root", "-e", sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func filepathJoin(base, name string) string {
	base = strings.TrimRight(base, "/")
	if base == "" {
		return name
	}
	return base + "/" + name
}

func pxcOptionSafePassword(password string) string {
	if password == "" {
		return password
	}
	replacer := strings.NewReplacer("#", "_", "\n", "_", "\r", "_")
	return replacer.Replace(password)
}

func pxcConfigPath(config PXCConfig) string {
	return fmt.Sprintf("/etc/dbops-pxc/dbops-pxc-%d.cnf", config.MySQLPort)
}

func pxcInitConfigPath(config PXCConfig) string {
	return fmt.Sprintf("/etc/dbops-pxc/dbops-pxc-%d-init.cnf", config.MySQLPort)
}

func pxcErrorLogPath(config PXCConfig) string {
	return fmt.Sprintf("/var/log/dbops-pxc/pxc-%d-error.log", config.MySQLPort)
}

func pxcStartupLogPath(config PXCConfig) string {
	return fmt.Sprintf("/var/log/dbops-pxc/pxc-%d-startup.log", config.MySQLPort)
}

func pxcMysqldPath() string {
	for _, candidate := range []string{"/opt/dbops-pxc/usr/sbin/mysqld", "/usr/sbin/mysqld"} {
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return "/usr/sbin/mysqld"
}

func pxcMysqlPath() string {
	for _, candidate := range []string{"/opt/dbops-pxc/usr/bin/mysql", "/usr/bin/mysql"} {
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return "mysql"
}

func pxcMysqladminPath() string {
	for _, candidate := range []string{"/opt/dbops-pxc/usr/bin/mysqladmin", "/usr/bin/mysqladmin"} {
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return "mysqladmin"
}

func pxcMysqlExecCommand(ctx context.Context, host string, port int, user string, password string, sql string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, pxcMysqlPath(),
		"-h", host,
		"-P", fmt.Sprintf("%d", port),
		"-u", defaultString(user, "root"),
		"-e", sql)
	if password != "" {
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+password)
	}
	return cmd
}

func pxcPluginDir() string {
	for _, candidate := range []string{"/opt/dbops-pxc/usr/lib/mysql/plugin", "/usr/lib/mysql/plugin"} {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
	}
	return ""
}

func pxcProviderPath() string {
	for _, candidate := range []string{"/opt/dbops-pxc/usr/lib/galera4/libgalera_smm.so", "/usr/lib/galera4/libgalera_smm.so"} {
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return "/usr/lib/galera4/libgalera_smm.so"
}

func (e *TaskExecutor) ConfigureGalera(ctx context.Context, nodeHost string, nodePort int, config map[string]interface{}) (*TaskResult, error) {
	gc := parseGaleraConfig(config)

	if gc.WSREPClusterAddress == "" {
		return &TaskResult{
			TaskID:    "configure-galera",
			Status:    "failed",
			Progress:  0,
			Message:   "wsrep_cluster_address is required for Galera configuration",
			Timestamp: time.Now(),
		}, nil
	}

	configSQL := fmt.Sprintf(
		"SET GLOBAL wsrep_cluster_address='%s'; "+
			"SET GLOBAL wsrep_node_address='%s:%d'; "+
			"SET GLOBAL wsrep_node_name='%s'; "+
			"SET GLOBAL wsrep_sst_method='%s'; "+
			"SET GLOBAL wsrep_provider='%s';",
		gc.WSREPClusterAddress, gc.WSREPNodeAddress, nodePort, gc.WSREPNodeName,
		gc.WSREPSSTMethod, gc.WSREPProvider,
	)

	cmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", configSQL)

	if err := cmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    "configure-galera",
			Status:    "failed",
			Progress:  30,
			Message:   fmt.Sprintf("Failed to set Galera variables: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	gcacheSQL := fmt.Sprintf(
		"SET GLOBAL wsrep_provider_options='gcache.size=%s; gcache.page_size=%s; gcache.keep_pages_size=%s';",
		gc.GCacheSize, gc.GCachePageSize, gc.GCacheKeepPagesSize,
	)

	gcacheCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", gcacheSQL)

	if err := gcacheCmd.Run(); err != nil {
		return &TaskResult{
			TaskID:    "configure-galera",
			Status:    "failed",
			Progress:  60,
			Message:   fmt.Sprintf("Failed to set Galera gcache options: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	verifyCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", "SHOW VARIABLES LIKE 'wsrep_%';")

	output, err := verifyCmd.Output()
	if err != nil {
		return &TaskResult{
			TaskID:    "configure-galera",
			Status:    "failed",
			Progress:  80,
			Message:   fmt.Sprintf("Failed to verify Galera configuration: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	if !strings.Contains(string(output), gc.WSREPClusterAddress) {
		return &TaskResult{
			TaskID:    "configure-galera",
			Status:    "warning",
			Progress:  100,
			Message:   "Galera configuration applied but verification incomplete",
			Timestamp: time.Now(),
		}, nil
	}

	return &TaskResult{
		TaskID:    "configure-galera",
		Status:    "completed",
		Progress:  100,
		Message:   fmt.Sprintf("Galera configuration applied successfully to node %s:%d", nodeHost, nodePort),
		Timestamp: time.Now(),
	}, nil
}

func (e *TaskExecutor) MonitorPXCSync(ctx context.Context, nodeHost string, nodePort int) *PXCSyncStatus {
	return e.monitorPXCSyncWithAuth(ctx, nodeHost, nodePort, "root", "")
}

func (e *TaskExecutor) monitorPXCSyncWithAuth(ctx context.Context, nodeHost string, nodePort int, user, pass string) *PXCSyncStatus {
	statusCmd := pxcMysqlExecCommand(ctx, nodeHost, nodePort, defaultString(user, "root"), pass, "SHOW STATUS LIKE 'wsrep_%';")
	output, err := statusCmd.Output()
	if err != nil {
		return &PXCSyncStatus{
			ClusterSize:   0,
			ClusterStatus: "disconnected",
			Ready:         false,
		}
	}

	outputStr := string(output)
	status := &PXCSyncStatus{}

	if strings.Contains(outputStr, "wsrep_cluster_size") {
		sizeStart := strings.Index(outputStr, "wsrep_cluster_size")
		if sizeStart != -1 {
			sizeLine := outputStr[sizeStart:]
			sizeEnd := strings.Index(sizeLine, "\n")
			if sizeEnd != -1 {
				sizeLine = sizeLine[:sizeEnd]
				var size int
				fmt.Sscanf(sizeLine, "wsrep_cluster_size\t%d", &size)
				status.ClusterSize = size
			}
		}
	}

	if strings.Contains(outputStr, "wsrep_cluster_status") {
		statusStart := strings.Index(outputStr, "wsrep_cluster_status")
		if statusStart != -1 {
			statusLine := outputStr[statusStart:]
			statusEnd := strings.Index(statusLine, "\n")
			if statusEnd != -1 {
				statusLine = statusLine[:statusEnd]
				var clusterStatus string
				fmt.Sscanf(statusLine, "wsrep_cluster_status\t%s", &clusterStatus)
				status.ClusterStatus = clusterStatus
			}
		}
	}

	if strings.Contains(outputStr, "wsrep_local_index") {
		indexStart := strings.Index(outputStr, "wsrep_local_index")
		if indexStart != -1 {
			indexLine := outputStr[indexStart:]
			indexEnd := strings.Index(indexLine, "\n")
			if indexEnd != -1 {
				indexLine = indexLine[:indexEnd]
				var localIndex int
				fmt.Sscanf(indexLine, "wsrep_local_index\t%d", &localIndex)
				status.LocalIndex = localIndex
			}
		}
	}

	if strings.Contains(outputStr, "wsrep_local_state") {
		stateStart := strings.Index(outputStr, "wsrep_local_state")
		if stateStart != -1 {
			stateLine := outputStr[stateStart:]
			stateEnd := strings.Index(stateLine, "\n")
			if stateEnd != -1 {
				stateLine = stateLine[:stateEnd]
				var localState string
				fmt.Sscanf(stateLine, "wsrep_local_state\t%s", &localState)
				status.LocalState = localState
			}
		}
	}

	if strings.Contains(outputStr, "wsrep_ready") {
		if strings.Contains(outputStr, "wsrep_ready\tON") {
			status.Ready = true
		}
	}

	if strings.Contains(outputStr, "wsrep_flow_control_paused") {
		fcStart := strings.Index(outputStr, "wsrep_flow_control_paused")
		if fcStart != -1 {
			fcLine := outputStr[fcStart:]
			fcEnd := strings.Index(fcLine, "\n")
			if fcEnd != -1 {
				fcLine = fcLine[:fcEnd]
				var fcPaused string
				fmt.Sscanf(fcLine, "wsrep_flow_control_paused\t%s", &fcPaused)
				status.FlowControlPaused = (fcPaused != "0.000000")
			}
		}
	}

	return status
}

func (e *TaskExecutor) DetectSplitBrain(ctx context.Context, nodeHost string, nodePort int) (*SplitBrainInfo, error) {
	splitInfo := &SplitBrainInfo{
		Detected:         false,
		IsolatedNodes:    []string{},
		ActiveNodes:      []string{},
		QuorumStatus:     "unknown",
		PrimaryComponent: false,
	}

	statusCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", "SHOW STATUS LIKE 'wsrep_cluster_status';")

	statusOutput, err := statusCmd.Output()
	if err != nil {
		splitInfo.QuorumStatus = "disconnected"
		return splitInfo, nil
	}

	statusStr := string(statusOutput)
	if strings.Contains(statusStr, "Non-Primary") {
		splitInfo.Detected = true
		splitInfo.QuorumStatus = "non-primary"
		splitInfo.PrimaryComponent = false
	} else if strings.Contains(statusStr, "Primary") {
		splitInfo.QuorumStatus = "primary"
		splitInfo.PrimaryComponent = true
	}

	sizeCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", "SHOW STATUS LIKE 'wsrep_cluster_size';")

	sizeOutput, err := sizeCmd.Output()
	if err != nil {
		return splitInfo, nil
	}

	var actualClusterSize int
	sizeStr := string(sizeOutput)
	fmt.Sscanf(sizeStr, "wsrep_cluster_size\t%d", &actualClusterSize)

	addressCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", "SHOW VARIABLES LIKE 'wsrep_cluster_address';")

	addressOutput, err := addressCmd.Output()
	if err != nil {
		return splitInfo, nil
	}

	addressStr := string(addressOutput)
	configuredNodes := parseWSREPAddress(addressStr)

	if len(configuredNodes) > 0 && actualClusterSize < len(configuredNodes) {
		splitInfo.Detected = true
		splitInfo.IsolatedNodes = identifyIsolatedNodes(configuredNodes, actualClusterSize)
	}

	stateCmd := exec.CommandContext(ctx, "mysql", "-h", nodeHost,
		"-P", fmt.Sprintf("%d", nodePort),
		"-u", "root", "-e", "SHOW STATUS LIKE 'wsrep_local_state_comment';")

	stateOutput, err := stateCmd.Output()
	if err != nil {
		return splitInfo, nil
	}

	stateStr := string(stateOutput)
	if strings.Contains(stateStr, "Synced") {
		splitInfo.ActiveNodes = append(splitInfo.ActiveNodes, nodeHost)
	}

	if splitInfo.Detected {
		splitInfo.ActiveNodes = identifyActiveNodes(configuredNodes, splitInfo.IsolatedNodes)
	}

	return splitInfo, nil
}

func parseWSREPAddress(addressStr string) []string {
	var nodes []string

	startIdx := strings.Index(addressStr, "gcomm://")
	if startIdx == -1 {
		return nodes
	}

	addressPart := addressStr[startIdx+8:]
	endIdx := strings.Index(addressPart, "\n")
	if endIdx != -1 {
		addressPart = addressPart[:endIdx]
	}

	if addressPart == "" {
		return nodes
	}

	nodeList := strings.Split(addressPart, ",")
	for _, node := range nodeList {
		node = strings.TrimSpace(node)
		if node != "" {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

func identifyIsolatedNodes(configuredNodes []string, activeSize int) []string {
	if activeSize >= len(configuredNodes) {
		return []string{}
	}

	isolatedCount := len(configuredNodes) - activeSize
	if isolatedCount > 0 && isolatedCount < len(configuredNodes) {
		return configuredNodes[activeSize:]
	}

	return []string{}
}

func identifyActiveNodes(configuredNodes []string, isolatedNodes []string) []string {
	isolatedMap := make(map[string]bool)
	for _, node := range isolatedNodes {
		isolatedMap[node] = true
	}

	var activeNodes []string
	for _, node := range configuredNodes {
		if !isolatedMap[node] {
			activeNodes = append(activeNodes, node)
		}
	}

	return activeNodes
}
