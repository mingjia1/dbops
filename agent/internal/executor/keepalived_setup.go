package executor

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type KeepalivedSetup struct{}

func NewKeepalivedSetup() *KeepalivedSetup {
	return &KeepalivedSetup{}
}

type KeepalivedConfig struct {
	VIP          string `json:"vip"`
	VIPInterface string `json:"vip_interface"`
	Priority     int    `json:"priority"`
	Role         string `json:"role"`
	MySQLPort    int    `json:"mysql_port"`
	MySQLUser    string `json:"mysql_user"`
	MySQLPass    string `json:"mysql_pass"`
	VRID         int    `json:"virtual_router_id"`
	AuthPass     string `json:"auth_pass"`
}

var keepalivedConfigPath = "/etc/keepalived/keepalived.conf"

var keepalivedIfacePattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)

func (s *KeepalivedSetup) Install(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "yum", "install", "-y", "keepalived")
	if err := cmd.Run(); err != nil {
		cmd2 := exec.CommandContext(ctx, "apt-get", "install", "-y", "keepalived")
		return cmd2.Run()
	}
	return nil
}

func (s *KeepalivedSetup) GenerateConfig(cfg KeepalivedConfig) (string, error) {
	if err := validateKeepalivedConfig(&cfg); err != nil {
		return "", err
	}
	state := "BACKUP"
	if strings.EqualFold(cfg.Role, "MASTER") {
		state = "MASTER"
	}

	return fmt.Sprintf(`global_defs {
    router_id mysql_%d
}

vrrp_script chk_mysql {
    script "/usr/local/bin/mysqlchk.sh %d"
    interval 2
    weight 2
}

vrrp_instance VI_1 {
    state %s
    interface %s
    virtual_router_id %d
    priority %d
    advert_int 1
    authentication {
        auth_type PASS
        auth_pass %s
    }
    virtual_ipaddress {
        %s
    }
    track_script {
        chk_mysql
    }
}
`, cfg.MySQLPort, cfg.MySQLPort, state, cfg.VIPInterface, cfg.VRID, cfg.Priority, cfg.AuthPass, cfg.VIP), nil
}

func (s *KeepalivedSetup) WriteConfig(ctx context.Context, config string) error {
	_ = ctx
	dir := filepath.Dir(keepalivedConfigPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".keepalived.conf.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(config); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, keepalivedConfigPath)
}

func (s *KeepalivedSetup) Start(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "systemctl", "enable", "keepalived")
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.CommandContext(ctx, "systemctl", "restart", "keepalived")
	return cmd.Run()
}

func validateKeepalivedConfig(cfg *KeepalivedConfig) error {
	if cfg == nil {
		return fmt.Errorf("keepalived config is required")
	}
	cfg.VIP = strings.TrimSpace(cfg.VIP)
	if cfg.VIP == "" || net.ParseIP(cfg.VIP) == nil {
		return fmt.Errorf("valid vip is required")
	}
	cfg.VIPInterface = strings.TrimSpace(cfg.VIPInterface)
	if cfg.VIPInterface == "" {
		cfg.VIPInterface = "eth0"
	}
	if !keepalivedIfacePattern.MatchString(cfg.VIPInterface) {
		return fmt.Errorf("invalid vip_interface")
	}
	if cfg.MySQLPort <= 0 || cfg.MySQLPort > 65535 {
		return fmt.Errorf("mysql_port must be between 1 and 65535")
	}
	if cfg.Priority == 0 {
		cfg.Priority = 100
	}
	if cfg.Priority < 1 || cfg.Priority > 254 {
		return fmt.Errorf("priority must be between 1 and 254")
	}
	if cfg.VRID == 0 {
		cfg.VRID = 51
	}
	if cfg.VRID < 1 || cfg.VRID > 255 {
		return fmt.Errorf("virtual_router_id must be between 1 and 255")
	}
	cfg.AuthPass = strings.TrimSpace(cfg.AuthPass)
	if cfg.AuthPass == "" {
		cfg.AuthPass = fmt.Sprintf("dbops%03d", cfg.VRID)
	}
	if len(cfg.AuthPass) > 8 || !keepalivedIfacePattern.MatchString(cfg.AuthPass) {
		return fmt.Errorf("auth_pass must be 1-8 safe characters")
	}
	return nil
}
