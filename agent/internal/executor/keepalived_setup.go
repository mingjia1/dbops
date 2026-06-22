package executor

import (
	"context"
	"fmt"
	"os/exec"
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
}

func (s *KeepalivedSetup) Install(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "yum", "install", "-y", "keepalived")
	if err := cmd.Run(); err != nil {
		cmd2 := exec.CommandContext(ctx, "apt-get", "install", "-y", "keepalived")
		return cmd2.Run()
	}
	return nil
}

func (s *KeepalivedSetup) GenerateConfig(cfg KeepalivedConfig) string {
	state := "BACKUP"
	if cfg.Role == "MASTER" {
		state = "MASTER"
	}
	iface := cfg.VIPInterface
	if iface == "" {
		iface = "eth0"
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
    virtual_router_id 51
    priority %d
    advert_int 1
    authentication {
        auth_type PASS
        auth_pass mysqlha
    }
    virtual_ipaddress {
        %s
    }
    track_script {
        chk_mysql
    }
}
`, cfg.MySQLPort, cfg.MySQLPort, state, iface, cfg.Priority, cfg.VIP)
}

func (s *KeepalivedSetup) WriteConfig(ctx context.Context, config string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("echo '%s' > /etc/keepalived/keepalived.conf", config))
	return cmd.Run()
}

func (s *KeepalivedSetup) Start(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "systemctl", "enable", "keepalived")
	_ = cmd.Run()
	cmd = exec.CommandContext(ctx, "systemctl", "restart", "keepalived")
	return cmd.Run()
}
