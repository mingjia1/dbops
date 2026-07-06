package executor

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type ProxySQLSetup struct{}

func NewProxySQLSetup() *ProxySQLSetup {
	return &ProxySQLSetup{}
}

type ProxySQLConfig struct {
	ProxyHost   string                  `json:"proxy_host"`
	ProxyPort   int                     `json:"proxy_port"`
	AdminHost   string                  `json:"admin_host"`
	AdminPort   int                     `json:"admin_port"`
	AdminUser   string                  `json:"admin_user"`
	AdminPass   string                  `json:"admin_pass"`
	HostgroupID int                     `json:"hostgroup_id"`
	Backends    []ProxySQLBackendConfig `json:"backends"`
}

type ProxySQLBackendConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

var proxySQLHostPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

func (s *ProxySQLSetup) Install(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "yum", "install", "-y", "proxysql")
	if err := cmd.Run(); err != nil {
		cmd2 := exec.CommandContext(ctx, "apt-get", "install", "-y", "proxysql")
		return cmd2.Run()
	}
	return nil
}

func (s *ProxySQLSetup) ConfigureBackends(ctx context.Context, cfg ProxySQLConfig) error {
	normalizeProxySQLConfig(&cfg)
	if err := validateProxySQLConfig(cfg); err != nil {
		return err
	}
	for _, backend := range cfg.Backends {
		query := fmt.Sprintf(
			"INSERT OR REPLACE INTO mysql_servers (hostname, port, hostgroup_id) VALUES ('%s', %d, %d)",
			proxySQLQuote(backend.Host), backend.Port, cfg.HostgroupID,
		)
		cmd := proxySQLAdminCommand(ctx, cfg, query)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("configure backend %s:%d: %w, output: %s", backend.Host, backend.Port, err, strings.TrimSpace(string(out)))
		}
	}

	reloadCmd := proxySQLAdminCommand(ctx, cfg, "LOAD MYSQL SERVERS TO RUNTIME; SAVE MYSQL SERVERS TO DISK;")
	if out, err := reloadCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("reload proxysql servers: %w, output: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *ProxySQLSetup) Start(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "systemctl", "enable", "proxysql")
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.CommandContext(ctx, "systemctl", "restart", "proxysql")
	return cmd.Run()
}

func normalizeProxySQLConfig(cfg *ProxySQLConfig) {
	if cfg.AdminHost == "" {
		cfg.AdminHost = "127.0.0.1"
	}
	if cfg.AdminPort == 0 {
		cfg.AdminPort = 6032
	}
	if cfg.AdminUser == "" {
		cfg.AdminUser = "admin"
	}
	if cfg.HostgroupID == 0 {
		cfg.HostgroupID = 10
	}
}

func validateProxySQLConfig(cfg ProxySQLConfig) error {
	if cfg.AdminPort <= 0 || cfg.AdminPort > 65535 {
		return fmt.Errorf("admin_port must be between 1 and 65535")
	}
	if cfg.ProxyPort <= 0 || cfg.ProxyPort > 65535 {
		return fmt.Errorf("proxy_port must be between 1 and 65535")
	}
	if cfg.AdminHost == "" || !proxySQLHostPattern.MatchString(cfg.AdminHost) {
		return fmt.Errorf("invalid admin_host")
	}
	if strings.TrimSpace(cfg.AdminUser) == "" {
		return fmt.Errorf("admin_user is required")
	}
	if cfg.HostgroupID <= 0 {
		return fmt.Errorf("hostgroup_id must be positive")
	}
	if len(cfg.Backends) == 0 {
		return fmt.Errorf("at least one backend is required")
	}
	for _, backend := range cfg.Backends {
		if backend.Host == "" || !proxySQLHostPattern.MatchString(backend.Host) {
			return fmt.Errorf("invalid backend host %q", backend.Host)
		}
		if backend.Port <= 0 || backend.Port > 65535 {
			return fmt.Errorf("invalid backend port for %s", backend.Host)
		}
	}
	return nil
}

func proxySQLAdminCommand(ctx context.Context, cfg ProxySQLConfig, query string) *exec.Cmd {
	args := []string{"-u", cfg.AdminUser, "-h", cfg.AdminHost, "-P", fmt.Sprintf("%d", cfg.AdminPort)}
	if cfg.AdminPass != "" {
		args = append(args, "-p"+cfg.AdminPass)
	}
	args = append(args, "-e", query)
	return exec.CommandContext(ctx, "proxysql", args...)
}

func proxySQLQuote(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
