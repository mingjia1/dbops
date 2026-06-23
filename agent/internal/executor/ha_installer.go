package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type HAInstaller struct {
	SkipOSUserCheck bool
}

func NewHAInstaller() *HAInstaller {
	return &HAInstaller{}
}

type HAInstallRequest struct {
	Flavor    string `json:"flavor"`
	Version   string `json:"version"`
	Port      int    `json:"port"`
	ArchType  string `json:"arch_type"`
	DataDir   string `json:"data_dir"`
	Basedir   string `json:"basedir"`
	OSUser    string `json:"os_user"`
}

type HAInstallResult struct {
	Status      string   `json:"status"`
	Message     string   `json:"message"`
	BinaryPath  string   `json:"binary_path"`
	ConfigPath  string   `json:"config_path"`
	Steps       []string `json:"steps"`
}

func (h *HAInstaller) InstallReplicationSetup(ctx context.Context, req HAInstallRequest) (*HAInstallResult, error) {
	if req.Port == 0 {
		req.Port = 3306
	}
	if req.DataDir == "" {
		req.DataDir = fmt.Sprintf("/data/mysql_%d", req.Port)
	}

	var steps []string

	if !h.SkipOSUserCheck {
		user := "mysql"
		if req.OSUser != "" {
			user = req.OSUser
		}
		if err := ensureOSUser(user); err != nil {
			return nil, fmt.Errorf("ensure user: %w", err)
		}
	}
	steps = append(steps, "OS user ensured")

	if err := os.MkdirAll(req.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create datadir: %w", err)
	}
	steps = append(steps, "data directory created")

	osUser := "mysql"
	if req.OSUser != "" {
		osUser = req.OSUser
	}
	svcContent := generateReplicaService(req.Port, req.Basedir, req.DataDir, osUser)
	svcName := fmt.Sprintf("mysqld_%d.service", req.Port)
	svcPath := filepath.Join("/etc/systemd/system", svcName)
	if err := writeFile(svcPath, svcContent); err != nil {
		return nil, fmt.Errorf("write service: %w", err)
	}
	steps = append(steps, "systemd service created")

	cnfContent := generateReplicaCnf(req.Flavor, req.Port, req.DataDir, req.Basedir)
	cnfPath := fmt.Sprintf("/etc/my.cnf.d/mysqld_%d.cnf", req.Port)
	if err := writeFile(cnfPath, cnfContent); err != nil {
		return nil, fmt.Errorf("write my.cnf: %w", err)
	}
	steps = append(steps, "my.cnf generated")

	binary := "mysqld"
	if req.Flavor == "mariadb" {
		binary = "mariadbd"
	}
	binaryPath := filepath.Join(req.Basedir, "bin", binary)

	return &HAInstallResult{
		Status:     "completed",
		Message:    fmt.Sprintf("Replication instance installed on port %d", req.Port),
		BinaryPath: binaryPath,
		ConfigPath: cnfPath,
		Steps:      steps,
	}, nil
}

func (h *HAInstaller) InstallMHA(ctx context.Context, req HAInstallRequest) (*HAInstallResult, error) {
	var steps []string

	mhaCnf := generateMHACnf(req.ArchType, req.Port)
	cnfPath := "/etc/mha/app1.cnf"
	if err := os.MkdirAll("/etc/mha", 0o755); err != nil {
		return nil, fmt.Errorf("create /etc/mha: %w", err)
	}
	if err := writeFile(cnfPath, mhaCnf); err != nil {
		return nil, fmt.Errorf("write mha.cnf: %w", err)
	}
	steps = append(steps, "MHA config generated")

	checkScript := generateMHACheckScript()
	if err := writeFile("/usr/local/bin/mha_check_repl.sh", checkScript); err != nil {
		return nil, fmt.Errorf("write check script: %w", err)
	}
	steps = append(steps, "MHA check script created")

	return &HAInstallResult{
		Status:     "completed",
		Message:    "MHA configuration installed",
		ConfigPath: cnfPath,
		Steps:      steps,
	}, nil
}

func (h *HAInstaller) InstallMGR(ctx context.Context, req HAInstallRequest) (*HAInstallResult, error) {
	var steps []string

	if !h.SkipOSUserCheck {
		if err := ensureOSUser("mysql"); err != nil {
			return nil, fmt.Errorf("ensure user: %w", err)
		}
	}

	cnfContent := generateMGRCnf(req.Flavor, req.Port, req.DataDir, req.Basedir)
	cnfPath := fmt.Sprintf("/etc/my.cnf.d/mysqld_%d.cnf", req.Port)
	if err := writeFile(cnfPath, cnfContent); err != nil {
		return nil, fmt.Errorf("write my.cnf: %w", err)
	}
	steps = append(steps, "MGR my.cnf generated")

	svcContent := generateMGRService(req.Port, req.Basedir, req.DataDir)
	svcName := fmt.Sprintf("mysqld_%d.service", req.Port)
	svcPath := filepath.Join("/etc/systemd/system", svcName)
	if err := writeFile(svcPath, svcContent); err != nil {
		return nil, fmt.Errorf("write service: %w", err)
	}
	steps = append(steps, "MGR systemd service created")

	return &HAInstallResult{
		Status:     "completed",
		Message:    fmt.Sprintf("MGR instance installed on port %d", req.Port),
		ConfigPath: cnfPath,
		Steps:      steps,
	}, nil
}

func (h *HAInstaller) InstallPXC(ctx context.Context, req HAInstallRequest) (*HAInstallResult, error) {
	var steps []string

	cnfContent := generatePXCCnf(req.Port, req.DataDir, req.Basedir)
	cnfPath := fmt.Sprintf("/etc/my.cnf.d/mysqld_%d.cnf", req.Port)
	if err := writeFile(cnfPath, cnfContent); err != nil {
		return nil, fmt.Errorf("write my.cnf: %w", err)
	}
	steps = append(steps, "PXC my.cnf generated")

	svcContent := generatePXCService(req.Port, req.Basedir, req.DataDir)
	svcName := fmt.Sprintf("mysqld_%d.service", req.Port)
	svcPath := filepath.Join("/etc/systemd/system", svcName)
	if err := writeFile(svcPath, svcContent); err != nil {
		return nil, fmt.Errorf("write service: %w", err)
	}
	steps = append(steps, "PXC systemd service created")

	return &HAInstallResult{
		Status:     "completed",
		Message:    fmt.Sprintf("PXC instance installed on port %d", req.Port),
		ConfigPath: cnfPath,
		Steps:      steps,
	}, nil
}

func generateReplicaService(port int, basedir, datadir, user string) string {
	if basedir == "" { basedir = "/opt/mysql" }
	return fmt.Sprintf(`[Unit]
Description=MySQL Server on port %d
After=network.target

[Service]
Type=notify
User=%s
ExecStart=%s/bin/mysqld --defaults-file=/etc/my.cnf.d/mysqld_%d.cnf
Restart=on-failure
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
`, port, user, basedir, port)
}

func generateMGRService(port int, basedir, datadir string) string {
	if basedir == "" { basedir = "/opt/mysql" }
	return fmt.Sprintf(`[Unit]
Description=MySQL Group Replication on port %d
After=network.target

[Service]
Type=notify
User=mysql
ExecStart=%s/bin/mysqld --defaults-file=/etc/my.cnf.d/mysqld_%d.cnf
Restart=on-failure
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
`, port, basedir, port)
}

func generatePXCService(port int, basedir, datadir string) string {
	if basedir == "" { basedir = "/opt/percona-xtradb-cluster" }
	return fmt.Sprintf(`[Unit]
Description=Percona XtraDB Cluster on port %d
After=network.target

[Service]
Type=notify
User=mysql
ExecStart=%s/bin/mysqld --defaults-file=/etc/my.cnf.d/mysqld_%d.cnf
Restart=on-failure
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
`, port, basedir, port)
}

func generateReplicaCnf(flavor string, port int, datadir, basedir string) string {
	gtid := "gtid_mode=ON\nenforce_gtid_consistency=ON"
	if flavor == "mariadb" {
		gtid = fmt.Sprintf("gtid_domain_id=%d", port)
	}
	return fmt.Sprintf(`[mysqld]
port=%d
server-id=%d
datadir=%s
basedir=%s
socket=/tmp/mysql_%d.sock
pid-file=/tmp/mysql_%d.pid

%s

log_bin=%s/binlog
binlog_format=ROW
relay_log=%s/relay-log
report_host=127.0.0.1
report_port=%d

character-set-server=utf8mb4
max_connections=500
innodb_buffer_pool_size=1G
`, port, port, datadir, basedir, port, port, gtid, datadir, datadir, port)
}

func generateMGRCnf(flavor string, port int, datadir, basedir string) string {
	base := generateReplicaCnf(flavor, port, datadir, basedir)
	return base + `
# Group Replication
plugin_load_add='group_replication.so'
group_replication_start_on_boot=OFF
group_replication_single_primary_mode=ON
group_replication_enforce_update_everywhere_checks=OFF
`
}

func generatePXCCnf(port int, datadir, basedir string) string {
	return fmt.Sprintf(`[mysqld]
port=%d
server-id=%d
datadir=%s
basedir=%s
socket=/tmp/mysql_%d.sock
pid-file=/tmp/mysql_%d.pid

# PXC Galera
wsrep_on=ON
wsrep_provider=%s/lib/libgalera_smm.so
wsrep_cluster_name=pxc_cluster
wsrep_sst_method=xtrabackup-v2
wsrep_node_address=127.0.0.1:%d

binlog_format=ROW
default_storage_engine=InnoDB
innodb_autoinc_lock_mode=2

character-set-server=utf8mb4
max_connections=500
innodb_buffer_pool_size=1G
`, port, port, datadir, basedir, port, port, basedir, port)
}

func generateMHACnf(archType string, port int) string {
	return fmt.Sprintf(`[server default]
user=mha
password=mha_pass
ssh_user=root
repl_user=repl
repl_password=repl_pass
master_binlog_dir=/data/mysql_%d
candidate_master=1

[server1]
hostname=127.0.0.1
port=%d

[server2]
hostname=127.0.0.1
port=%d

[server3]
hostname=127.0.0.1
port=%d
`, port, port, port+1, port+2)
}

func generateMHACheckScript() string {
	return `#!/bin/bash
# MHA replication check helper
# Usage: mha_check_repl.sh <host> <port> <user> <pass>
HOST=${1:-127.0.0.1}
PORT=${2:-3306}
USER=${3:-repl}
PASS=${4:-}

if [ -z "$PASS" ]; then
  mysql -h "$HOST" -P "$PORT" -u "$USER" -e "SELECT 1" > /dev/null 2>&1
else
  mysql -h "$HOST" -P "$PORT" -u "$USER" -p"$PASS" -e "SELECT 1" > /dev/null 2>&1
fi
exit $?
`
}

func writeFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644)
}
