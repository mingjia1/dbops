package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	MySQLVersion    string   `json:"mysql_version"`
	MySQLConfig     map[string]string
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
	if v, ok := config["mysql_version"].(string); ok {
		pc.MySQLVersion = v
	}
	pc.MySQLConfig = deployMySQLConfig(config)

	return pc
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

	if err := installPXCSystemPackage(ctx, e.relayCli, config.MySQLVersion); err != nil {
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

	if err := preparePXCDataDir(ctx, config, req.Config); err != nil {
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

	// Use version-aware auth plugin for SST user
	authPlugin := mysqlAuthPlugin(config.MySQLVersion)
	// MySQL 8.0+ includes BACKUP_ADMIN privilege, 5.7 does not have it
	var sstUserSQL string
	if strings.HasPrefix(config.MySQLVersion, "5.") {
		sstUserSQL = fmt.Sprintf(
			"CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED WITH %s BY '%s'; "+
				"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED WITH %s BY '%s'; "+
				"GRANT RELOAD, LOCK TABLES, PROCESS, REPLICATION CLIENT ON *.* TO '%s'@'localhost'; "+
				"GRANT RELOAD, LOCK TABLES, PROCESS, REPLICATION CLIENT ON *.* TO '%s'@'%%'; FLUSH PRIVILEGES;",
			config.ReplicateUser, authPlugin, config.ReplicatePass,
			config.ReplicateUser, authPlugin, config.ReplicatePass,
			config.ReplicateUser, config.ReplicateUser,
		)
	} else {
		sstUserSQL = fmt.Sprintf(
			"CREATE USER IF NOT EXISTS '%s'@'localhost' IDENTIFIED WITH %s BY '%s'; "+
				"CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED WITH %s BY '%s'; "+
				"GRANT RELOAD, LOCK TABLES, PROCESS, REPLICATION CLIENT, BACKUP_ADMIN ON *.* TO '%s'@'localhost'; "+
				"GRANT RELOAD, LOCK TABLES, PROCESS, REPLICATION CLIENT, BACKUP_ADMIN ON *.* TO '%s'@'%%'; FLUSH PRIVILEGES;",
			config.ReplicateUser, authPlugin, config.ReplicatePass,
			config.ReplicateUser, authPlugin, config.ReplicatePass,
			config.ReplicateUser, config.ReplicateUser,
		)
	}

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

func installPXCSystemPackage(ctx context.Context, relayClient *relayClient, mysqlVersion string) error {
	// Determine PXC repo version (pxc57 for MySQL 5.7, pxc80 for MySQL 8.0+)
	pxcRepoVersion := "pxc80"
	if strings.HasPrefix(mysqlVersion, "5.") {
		pxcRepoVersion = "pxc57"
	}

	// Try relay-based pre-installation if relay is configured
	if relayClient != nil {
		xtraCheck := exec.CommandContext(ctx, "bash", "-c", "command -v xtrabackup >/dev/null 2>&1 && echo installed || echo missing")
		if out, _ := xtraCheck.Output(); strings.TrimSpace(string(out)) != "installed" {
			releaseReq := RelayPackageRequest{
				URL:  "https://repo.percona.com/apt/percona-release_latest.jammy_all.deb",
				Name: "percona-release_latest.jammy_all.deb",
				Type: "percona_release",
			}
			if releaseRes, err := relayClient.fetchPackage(ctx, releaseReq); err == nil && releaseRes.Status == "success" {
				debPath := filepath.Join(os.TempDir(), filepath.Base(releaseRes.FileName))
				if err := relayClient.downloadPackage(ctx, releaseRes.FileName, debPath); err == nil {
					installCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
export DEBIAN_FRONTEND=noninteractive
dpkg -i %s || {
	apt-get update -qq 2>/dev/null || true
	apt-get install -y -qq -f 2>/dev/null || true
}
`, shellQuote(debPath)))
					if out, err := installCmd.CombinedOutput(); err != nil {
						_ = os.Remove(debPath)
						return fmt.Errorf("relay percona-release install failed: %v, output: %s", err, strings.TrimSpace(string(out)))
					}
					_ = os.Remove(debPath)
				}
			}

		}
		for _, pkg := range []string{"qpress", "socat"} {
			aptCmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(
				"export DEBIAN_FRONTEND=noninteractive; apt-get install -y -qq %s 2>/dev/null || true", pkg))
			aptCmd.Run()
		}
	}

	cmd := exec.CommandContext(ctx, "bash", "-lc", fmt.Sprintf(`
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
export APT_LISTCHANGES_FRONTEND=none
export NEEDRESTART_MODE=a
export NEEDRESTART_SUSPEND=1
	PXC_REPO="%s"
	patch_pxc_sst_path() {
	  script=/opt/dbops-pxc/usr/bin/wsrep_sst_xtrabackup-v2
	  if [ -f "$script" ]; then
	    sed -i 's#export PATH="/usr/sbin:/sbin:$PATH"#export PATH="/opt/dbops-pxc/usr/sbin:/opt/dbops-pxc/usr/bin:/usr/sbin:/sbin:$PATH"#' "$script"
	  fi
	}
	link_pxc_sst_tools() {
	  mkdir -p /opt/dbops-pxc/usr/bin
	  for tool in xtrabackup qpress socat; do
	    tool_path=$(command -v "$tool" 2>/dev/null || true)
	    if [ -n "$tool_path" ] && [ ! -x "/opt/dbops-pxc/usr/bin/$tool" ]; then
	      ln -sf "$tool_path" "/opt/dbops-pxc/usr/bin/$tool"
	    fi
	  done
	}
	if [ -x /opt/dbops-pxc/usr/sbin/mysqld ] && [ -f /opt/dbops-pxc/usr/lib/galera4/libgalera_smm.so ] && command -v mysql >/dev/null 2>&1 && command -v mysqladmin >/dev/null 2>&1 && command -v xtrabackup >/dev/null 2>&1 && command -v qpress >/dev/null 2>&1 && command -v socat >/dev/null 2>&1; then
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
  mkdir -p /etc/mysql/conf.d /etc/mysql/mysql.conf.d
  dpkg --force-confdef --force-confold --configure -a || true
  return 0
}
remove_broken_system_mysql_packages() {
  for pkg in mysql-server-8.0 mysql-server mysql-community-server; do
    status=$(dpkg-query -W -f='${db:Status-Abbrev}' "$pkg" 2>/dev/null || true)
    case "$status" in
      iF*|iH*|iU*|*R*)
        echo "removing broken system MySQL package $pkg (status=$status)"
        timeout 120s dpkg --remove --force-depends --force-remove-reinstreq "$pkg" >/dev/null 2>&1 || true
        ;;
    esac
  done
}
apt_install_base_tools() {
  timeout 180s apt-get -o Dpkg::Options::="--force-confdef" -o Dpkg::Options::="--force-confold" install -y wget gnupg2 lsb-release curl debconf-utils || true
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
	  xtrabackup_pkg="percona-xtrabackup-80"
	  if [ "$PXC_REPO" = "pxc57" ]; then
	    xtrabackup_pkg="percona-xtrabackup-24"
	  fi
	  timeout 180s apt-get \
	    -o Dpkg::Options::="--force-confdef" \
	    -o Dpkg::Options::="--force-confold" \
	    -o Dpkg::Options::="--force-overwrite" \
    install -y "$xtrabackup_pkg" qpress socat || {
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
		verify_pxc_runtime() {
		  link_pxc_sst_tools
		  missing=""
		  [ -x /opt/dbops-pxc/usr/sbin/mysqld ] || missing="$missing /opt/dbops-pxc/usr/sbin/mysqld"
		  [ -x /opt/dbops-pxc/usr/bin/mysql ] || missing="$missing /opt/dbops-pxc/usr/bin/mysql"
		  [ -x /opt/dbops-pxc/usr/bin/mysqladmin ] || missing="$missing /opt/dbops-pxc/usr/bin/mysqladmin"
		  [ -f /opt/dbops-pxc/usr/lib/galera4/libgalera_smm.so ] || missing="$missing /opt/dbops-pxc/usr/lib/galera4/libgalera_smm.so"
		  command -v xtrabackup >/dev/null 2>&1 || missing="$missing xtrabackup"
		  command -v qpress >/dev/null 2>&1 || missing="$missing qpress"
		  command -v socat >/dev/null 2>&1 || missing="$missing socat"
		  [ -x /opt/dbops-pxc/usr/bin/xtrabackup ] || missing="$missing /opt/dbops-pxc/usr/bin/xtrabackup"
		  [ -x /opt/dbops-pxc/usr/bin/qpress ] || missing="$missing /opt/dbops-pxc/usr/bin/qpress"
		  [ -x /opt/dbops-pxc/usr/bin/socat ] || missing="$missing /opt/dbops-pxc/usr/bin/socat"
		  if [ -n "$missing" ]; then
		    echo "PXC runtime dependency check failed; missing:$missing"
		    return 1
		  fi
	}
	show_pxc_apt_packages() {
	  echo "PXC packages from configured apt sources:"
	  apt-cache policy percona-xtradb-cluster percona-xtradb-cluster-common percona-xtradb-cluster-client percona-xtradb-cluster-server percona-xtrabackup-80 percona-xtrabackup-24 qpress socat || true
	  apt-cache search '^percona-xtradb-cluster' || true
}
wait_dpkg_lock
disable_expired_mysql_repo_lists
timeout 180s apt-get update
wait_dpkg_lock
dpkg --remove --force-depends --force-remove-reinstreq percona-xtradb-cluster percona-xtradb-cluster-server >/dev/null 2>&1 || true
remove_broken_system_mysql_packages
mkdir -p /etc/mysql/conf.d /etc/mysql/mysql.conf.d
timeout 180s dpkg --force-confdef --force-confold --configure -a || true
timeout 180s apt-get \
  -o Dpkg::Options::="--force-confdef" \
  -o Dpkg::Options::="--force-confold" \
  -o Dpkg::Options::="--force-overwrite" \
  -f install -y
wait_dpkg_lock
apt_install_base_tools
if command -v percona-release >/dev/null 2>&1; then
  percona-release setup -y "$PXC_REPO" || true
else
  echo "percona-release is not installed; using existing apt sources for PXC packages"
fi
wait_dpkg_lock
timeout 180s apt-get update
show_pxc_apt_packages
wait_dpkg_lock
dpkg --remove --force-depends --force-remove-reinstreq percona-xtradb-cluster percona-xtradb-cluster-server >/dev/null 2>&1 || true
remove_broken_system_mysql_packages
mkdir -p /etc/mysql/conf.d /etc/mysql/mysql.conf.d
timeout 180s dpkg --force-confdef --force-confold --configure -a || true
timeout 180s apt-get \
  -o Dpkg::Options::="--force-confdef" \
  -o Dpkg::Options::="--force-confold" \
  -o Dpkg::Options::="--force-overwrite" \
  -f install -y
wait_dpkg_lock
	apt_install_pxc_tools
	download_pxc_payload
	patch_pxc_sst_path
	verify_pxc_runtime
	`, pxcRepoVersion))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func writePXCConfig(ctx context.Context, config PXCConfig) error {
	content := buildPXCConfigContent(config)
	cmd := exec.CommandContext(ctx, "bash", "-lc", "mkdir -p /etc/dbops-pxc /var/log/dbops-pxc && cat > "+shellQuote(pxcConfigPath(config))+" <<'EOF'\n"+content+"EOF\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func buildPXCConfigContent(config PXCConfig) string {
	wsrepPort := config.WSREPPort
	if wsrepPort == 0 {
		wsrepPort = 4567
	}
	wsrepSSTPort := config.WSREPSSTPort
	if wsrepSSTPort == 0 {
		wsrepSSTPort = 4444
	}
	clusterNodes := make([]string, 0, len(config.Nodes))
	for _, node := range config.Nodes {
		if strings.Contains(node, ":") {
			clusterNodes = append(clusterNodes, node)
			continue
		}
		clusterNodes = append(clusterNodes, fmt.Sprintf("%s:%d", node, wsrepPort))
	}
	clusterAddress := "gcomm://" + strings.Join(clusterNodes, ",")
	providerOptions := fmt.Sprintf("gmcast.listen_addr=tcp://0.0.0.0:%d", wsrepPort)
	sstSection := fmt.Sprintf("\n[sst]\nport=%d\n", wsrepSSTPort)
	pxcEncryptClusterTraffic := "ON"
	if !config.WSREPSSLEnabled {
		providerOptions += ";socket.ssl=NO"
		pxcEncryptClusterTraffic = "OFF"
		sstSection = fmt.Sprintf("\n[sst]\nencrypt=0\nport=%d\n", wsrepSSTPort)
	}
	serverID := 1000 + config.MySQLPort%50000 + len(config.NodeHost)
	pluginDir := pxcPluginDir()
	pluginDirLine := ""
	if pluginDir != "" {
		pluginDirLine = fmt.Sprintf("plugin-dir=%s\n", pluginDir)
	}
	providerPath := pxcProviderPath()
	customLines := ""
	if lines := deployMySQLConfigLines(config.MySQLConfig); len(lines) > 0 {
		customLines = strings.Join(lines, "\n") + "\n"
	}
	return fmt.Sprintf(`[mysqld]
server_id=%d
port=%d
bind-address=0.0.0.0
basedir=/opt/dbops-pxc/usr
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
%s
wsrep_provider=%s
wsrep_cluster_name=%s
wsrep_cluster_address=%s
wsrep_node_address=%s
wsrep_node_name=%s-%s
wsrep_sst_method=%s
pxc_encrypt_cluster_traffic=%s
wsrep_provider_options=%s
%s`, serverID, config.MySQLPort, config.DataDir, config.DataDir, config.DataDir, pxcErrorLogPath(config), pluginDirLine, customLines, providerPath, config.ClusterName, clusterAddress, config.NodeHost, config.ClusterName, safeNodeName(config.NodeHost), defaultString(config.SSTMethod, "xtrabackup-v2"), pxcEncryptClusterTraffic, providerOptions, sstSection)
}

func safeNodeName(host string) string {
	replacer := strings.NewReplacer(".", "-", ":", "-")
	return replacer.Replace(host)
}

func preparePXCDataDir(ctx context.Context, config PXCConfig, rawConfig map[string]interface{}) error {
	forceClean := config.Bootstrap || configBool(rawConfig, "force")
	cmd := exec.CommandContext(ctx, "bash", "-lc", fmt.Sprintf(`
	set -euo pipefail
	datadir=%s
	force_clean=%s
	case "$datadir" in
	  /data/mysql/pxc-*|/tmp/dbops-pxc*) ;;
	  *) echo "refuse to prepare unsafe PXC datadir: $datadir"; exit 1 ;;
	esac
	mkdir -p "$datadir" /var/log/dbops-pxc /etc/dbops-pxc
	chmod o+x "$(dirname "$datadir")" 2>/dev/null || true
	if [ "$force_clean" = "true" ]; then
	  find "$datadir" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
	elif [ -n "$(find "$datadir" -mindepth 1 -maxdepth 1 -print -quit)" ]; then
	  if [ ! -d "$datadir/mysql" ] || [ ! -f "$datadir/grastate.dat" ]; then
	    echo "PXC datadir $datadir is not empty; set force=true to rebuild it"
	    exit 1
	  fi
	fi
	chown -R mysql:mysql "$datadir" /var/log/dbops-pxc
	`, shellQuote(config.DataDir), shellQuote(strconv.FormatBool(forceClean))))
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
	cmd := exec.CommandContext(ctx, "bash", "-lc", fmt.Sprintf("nohup env PATH=%s:$PATH %s --defaults-file=%s --user=mysql%s >> %s 2>&1 &", shellQuote("/opt/dbops-pxc/usr/sbin:/opt/dbops-pxc/usr/bin"), shellQuote(pxcMysqldPath()), shellQuote(pxcConfigPath(config)), extra, shellQuote(pxcStartupLogPath(config))))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func waitForPXCMySQL(ctx context.Context, config PXCConfig) error {
	socket := filepathJoin(config.DataDir, "mysql.sock")
	deadline := time.Now().Add(10 * time.Minute)
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
	return fmt.Errorf("timeout waiting for mysqld on port %d: %s", config.MySQLPort, diagnosePXCStartupFailure(ctx, config))
}

func pxcRecentLogs(ctx context.Context, config PXCConfig) string {
	cmd := exec.CommandContext(ctx, "bash", "-lc", fmt.Sprintf(`for f in %s %s; do
  if [ -s "$f" ]; then
    echo "== $f =="
    tail -80 "$f"
  fi
done`, shellQuote(pxcStartupLogPath(config)), shellQuote(pxcErrorLogPath(config))))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("failed to read PXC logs: %v, output: %s", err, strings.TrimSpace(string(out)))
	}
	logs := strings.TrimSpace(string(out))
	if logs == "" {
		return "PXC startup and error logs are empty"
	}
	return logs
}

func diagnosePXCStartupFailure(ctx context.Context, config PXCConfig) string {
	logs := pxcRecentLogs(ctx, config)
	diagnosis := classifyPXCStartupLogs(logs)
	if diagnosis == "" {
		return logs
	}
	return diagnosis + "\n" + logs
}

func classifyPXCStartupLogs(logs string) string {
	lower := strings.ToLower(logs)
	switch {
	case strings.Contains(lower, "unknown variable") && strings.Contains(lower, "wsrep_sst_auth"):
		return "detected unsupported PXC 8 wsrep_sst_auth option; DBOps no longer writes this option, remove stale custom config and retry"
	case strings.Contains(lower, "state transfer request failed") || strings.Contains(lower, "sst request failed"):
		return "detected SST failure; check donor reachability, wsrep/sst ports, xtrabackup/qpress/socat availability, and SST user grants"
	case strings.Contains(lower, "permission denied"):
		return "detected filesystem permission failure; DBOps will chown the managed datadir/logdir before start, verify parent directory execute permission"
	case strings.Contains(lower, "address already in use"):
		return "detected port conflict; check mysql_port/wsrep_port/wsrep_sst_port and stop the conflicting process"
	case strings.Contains(lower, "non-primary") || strings.Contains(lower, "failed to open gcomm backend connection"):
		return "detected Galera connectivity/quorum failure; verify all node_host values and firewall access for wsrep ports"
	case strings.Contains(lower, "xtrabackup") && strings.Contains(lower, "not found"):
		return "detected missing xtrabackup during SST; package installation now verifies xtrabackup before starting PXC"
	case strings.Contains(lower, "socat") && strings.Contains(lower, "not found"):
		return "detected missing socat during SST; package installation now verifies and links socat before starting PXC"
	default:
		return ""
	}
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
