package executor

import (
	"context"
	"fmt"
)

type AccountManager struct{}

func NewAccountManager() *AccountManager {
	return &AccountManager{}
}

// AccountSetupRequest 初始化账号请求
type AccountSetupRequest struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	AdminUser    string `json:"admin_user"`
	AdminPass    string `json:"admin_pass"`
	RootPassword string `json:"root_password"`
	ReplUser     string `json:"repl_user"`
	ReplPass     string `json:"repl_pass"`
	MonitorUser  string `json:"monitor_user"`
	MonitorPass  string `json:"monitor_pass"`
}

// AccountRotateRequest 密码轮转请求
type AccountRotateRequest struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	AdminUser   string `json:"admin_user"`
	AdminPass   string `json:"admin_pass"`
	TargetUser  string `json:"target_user"`
	NewPassword string `json:"new_password"`
}

func (m *AccountManager) SetupRootAccount(ctx context.Context, host string, port int, password string) error {
	if password == "" {
		return fmt.Errorf("root password cannot be empty")
	}
	_, err := runMySQLExec(ctx, host, port, "root", "",
		fmt.Sprintf("ALTER USER 'root'@'localhost' IDENTIFIED BY '%s'; FLUSH PRIVILEGES;", password))
	return err
}

func (m *AccountManager) SetupReplAccount(ctx context.Context, host string, port int, adminUser, adminPass, replUser, replPass string) error {
	if replUser == "" || replPass == "" {
		return fmt.Errorf("repl user and password cannot be empty")
	}
	queries := []string{
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'", replUser, replPass),
		fmt.Sprintf("GRANT REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO '%s'@'%%'", replUser),
		"FLUSH PRIVILEGES",
	}
	for _, q := range queries {
		if _, err := runMySQLExec(ctx, host, port, adminUser, adminPass, q); err != nil {
			return fmt.Errorf("setup repl: %w", err)
		}
	}
	return nil
}

func (m *AccountManager) SetupMonitorAccount(ctx context.Context, host string, port int, adminUser, adminPass, monitorUser, monitorPass string) error {
	if monitorUser == "" || monitorPass == "" {
		return fmt.Errorf("monitor user and password cannot be empty")
	}
	queries := []string{
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'", monitorUser, monitorPass),
		fmt.Sprintf("GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO '%s'@'%%'", monitorUser),
		"FLUSH PRIVILEGES",
	}
	for _, q := range queries {
		if _, err := runMySQLExec(ctx, host, port, adminUser, adminPass, q); err != nil {
			return fmt.Errorf("setup monitor: %w", err)
		}
	}
	return nil
}

func (m *AccountManager) RotatePassword(ctx context.Context, host string, port int, adminUser, adminPass, targetUser, newPass string) error {
	if targetUser == "" || newPass == "" {
		return fmt.Errorf("target user and new password cannot be empty")
	}
	queries := []string{
		fmt.Sprintf("ALTER USER '%s'@'%%' IDENTIFIED BY '%s'", targetUser, newPass),
		"FLUSH PRIVILEGES",
	}
	for _, q := range queries {
		if _, err := runMySQLExec(ctx, host, port, adminUser, adminPass, q); err != nil {
			return fmt.Errorf("rotate password: %w", err)
		}
	}
	return nil
}
