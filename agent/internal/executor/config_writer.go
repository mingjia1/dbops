package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type ConfigWriter struct{}

func NewConfigWriter() *ConfigWriter {
	return &ConfigWriter{}
}

func (w *ConfigWriter) WriteMySQLConfig(ctx context.Context, port int, configContent string, configDir string) (string, error) {
	if port == 0 {
		return "", fmt.Errorf("port is required")
	}
	if configContent == "" {
		return "", fmt.Errorf("config content is empty")
	}
	if configDir == "" {
		configDir = "/etc/my.cnf.d"
	}

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir %s: %w", configDir, err)
	}

	fileName := fmt.Sprintf("mysqld_%d.cnf", port)
	filePath := filepath.Join(configDir, fileName)

	if err := os.WriteFile(filePath, []byte(configContent), 0o644); err != nil {
		return "", fmt.Errorf("write config %s: %w", filePath, err)
	}

	return filePath, nil
}

func (w *ConfigWriter) WriteMHAConfig(ctx context.Context, clusterID string, configContent string) (string, error) {
	if configContent == "" {
		return "", fmt.Errorf("config content is empty")
	}

	configDir := "/etc/mha"
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", fmt.Errorf("create MHA config dir: %w", err)
	}

	filePath := filepath.Join(configDir, fmt.Sprintf("%s.cnf", clusterID))
	if err := os.WriteFile(filePath, []byte(configContent), 0o644); err != nil {
		return "", fmt.Errorf("write MHA config: %w", err)
	}

	return filePath, nil
}

func (w *ConfigWriter) ReloadMySQLConfig(ctx context.Context, host string, port int, adminUser, adminPass string) error {
	_, err := runMySQLExec(ctx, host, port, adminUser, adminPass, "FLUSH PRIVILEGES")
	return err
}
