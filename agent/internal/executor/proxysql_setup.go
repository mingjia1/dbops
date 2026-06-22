package executor

import (
	"context"
	"fmt"
	"os/exec"
)

type ProxySQLSetup struct{}

func NewProxySQLSetup() *ProxySQLSetup {
	return &ProxySQLSetup{}
}

type ProxySQLConfig struct {
	ProxyHost string                   `json:"proxy_host"`
	ProxyPort int                      `json:"proxy_port"`
	AdminUser string                   `json:"admin_user"`
	AdminPass string                   `json:"admin_pass"`
	Backends  []ProxySQLBackendConfig  `json:"backends"`
}

type ProxySQLBackendConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func (s *ProxySQLSetup) Install(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "yum", "install", "-y", "proxysql")
	if err := cmd.Run(); err != nil {
		cmd2 := exec.CommandContext(ctx, "apt-get", "install", "-y", "proxysql")
		return cmd2.Run()
	}
	return nil
}

func (s *ProxySQLSetup) ConfigureBackends(ctx context.Context, cfg ProxySQLConfig) error {
	for _, backend := range cfg.Backends {
		query := fmt.Sprintf(
			"INSERT OR REPLACE INTO mysql_servers (hostname, port, hostgroup_id) VALUES ('%s', %d, 10)",
			backend.Host, backend.Port,
		)
		cmd := exec.CommandContext(ctx, "proxysql", "-e", query)
		_ = cmd.Run()
	}

	reloadCmd := exec.CommandContext(ctx, "proxysql", "-e", "LOAD MYSQL SERVERS TO RUNTIME; SAVE MYSQL SERVERS TO DISK;")
	return reloadCmd.Run()
}

func (s *ProxySQLSetup) Start(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "systemctl", "enable", "proxysql")
	_ = cmd.Run()
	cmd = exec.CommandContext(ctx, "systemctl", "restart", "proxysql")
	return cmd.Run()
}
