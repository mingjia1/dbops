package executor

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type RelayDownloader struct{}

func NewRelayDownloader() *RelayDownloader {
	return &RelayDownloader{}
}

type RelayDownloadConfig struct {
	RelayURL  string
	Branch    string
	Version   string
	Basedir   string
	OSUser    string
	Checksum  string
}

func (d *RelayDownloader) DownloadFromRelay(ctx context.Context, cfg RelayDownloadConfig) (string, error) {
	if cfg.RelayURL == "" {
		return "", fmt.Errorf("relay URL is required")
	}
	if cfg.Version == "" {
		return "", fmt.Errorf("version is required")
	}

	tarballName := d.tarballName(cfg.Branch, cfg.Version)
	downloadURL := fmt.Sprintf("%s/%s/%s/%s", strings.TrimRight(cfg.RelayURL, "/"), cfg.Branch, cfg.Version, tarballName)

	if cfg.Basedir == "" {
		cfg.Basedir = fmt.Sprintf("/opt/%s-%s", cfg.Branch, cfg.Version)
	}
	if cfg.OSUser == "" {
		cfg.OSUser = "mysql"
	}

	mysqld, err := InstallFromURL(ctx, downloadURL, cfg.Checksum, cfg.Basedir, cfg.OSUser)
	if err != nil {
		return "", fmt.Errorf("install from relay: %w", err)
	}

	if err := d.runLdconfig(ctx); err != nil {
		return mysqld, fmt.Errorf("ldconfig warning: %w", err)
	}

	return mysqld, nil
}

func (d *RelayDownloader) tarballName(branch, version string) string {
	flavor := "mysql"
	if strings.Contains(branch, "mariadb") {
		flavor = "mariadb"
	} else if strings.Contains(branch, "percona") {
		flavor = "percona"
	}

	switch flavor {
	case "mariadb":
		return fmt.Sprintf("mariadb-%s-linux-x86_64.tar.gz", version)
	case "percona":
		return fmt.Sprintf("Percona-Server-%s-Linux.x86_64.glibc2.17.tar.gz", version)
	default:
		return fmt.Sprintf("mysql-%s-linux-glibc2.17-x86_64.tar.xz", version)
	}
}

func (d *RelayDownloader) runLdconfig(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "ldconfig")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ldconfig: %v: %s", err, string(out))
	}
	return nil
}

func (d *RelayDownloader) BinaryPath(basedir, flavor string) string {
	binaryName := "mysqld"
	if strings.Contains(flavor, "mariadb") {
		binaryName = "mariadbd"
	}
	return filepath.Join(basedir, "bin", binaryName)
}
