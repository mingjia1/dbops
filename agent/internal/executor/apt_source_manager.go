package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type APTSourceManager struct{}

func NewAPTSourceManager() *APTSourceManager {
	return &APTSourceManager{}
}

type APTSourceConfig struct {
	Flavor   string
	Version  string
	OSCodename string
}

func (m *APTSourceManager) ConfigureAPTSource(ctx context.Context, cfg APTSourceConfig) error {
	if cfg.OSCodename == "" {
		codename, err := m.detectOSCodename(ctx)
		if err != nil {
			return fmt.Errorf("detect OS codename: %w", err)
		}
		cfg.OSCodename = codename
	}

	majorMinor := extractMajorMinor(cfg.Version)

	var repoLine string
	var listFile string

	switch cfg.Flavor {
	case "mariadb":
		repoLine = fmt.Sprintf("deb http://mirror.mariadb.org/repo/%s/debian %s main", cfg.Version, cfg.OSCodename)
		listFile = "/etc/apt/sources.list.d/mariadb.list"
	case "percona":
		repoLine = fmt.Sprintf("deb http://repo.percona.com/ps-80 %s main", cfg.OSCodename)
		listFile = "/etc/apt/sources.list.d/percona.list"
	default:
		repoLine = fmt.Sprintf("deb http://repo.mysql.com/apt/%s/ mysql-%s main", cfg.OSCodename, majorMinor)
		listFile = "/etc/apt/sources.list.d/mysql.list"
	}

	if err := writeSourceListFile(listFile, repoLine); err != nil {
		return err
	}

	return m.runAptUpdate(ctx)
}

func (m *APTSourceManager) detectOSCodename(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "lsb_release", "-cs")
	out, err := cmd.Output()
	if err != nil {
		return "jammy", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func writeSourceListFile(path, content string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' > %s", content, path))
	return cmd.Run()
}

func (m *APTSourceManager) runAptUpdate(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "apt-get", "update", "-y")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt-get update failed: %v: %s", err, string(out))
	}
	return nil
}

func extractMajorMinor(version string) string {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return version
	}
	return parts[0] + "." + parts[1]
}
