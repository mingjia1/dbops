package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CgroupManager struct{}

func NewCgroupManager() *CgroupManager {
	return &CgroupManager{}
}

type CgroupConfig struct {
	Port          int
	MemoryLimitMB int
	CPUQuotaPct   int
	ServiceName   string
}

func (m *CgroupManager) GenerateSystemdOverride(cfg CgroupConfig) (string, string, error) {
	if cfg.Port == 0 {
		return "", "", fmt.Errorf("port is required")
	}
	svcName := cfg.ServiceName
	if svcName == "" {
		svcName = fmt.Sprintf("mysqld_%d", cfg.Port)
	}
	overrideDir := filepath.Join("/etc/systemd/system", svcName+".service.d")
	overrideFile := filepath.Join(overrideDir, "limits.conf")

	var lines []string
	lines = append(lines, "[Service]")
	if cfg.MemoryLimitMB > 0 {
		lines = append(lines, fmt.Sprintf("MemoryMax=%dM", cfg.MemoryLimitMB))
	}
	if cfg.CPUQuotaPct > 0 {
		lines = append(lines, fmt.Sprintf("CPUQuota=%d%%", cfg.CPUQuotaPct))
	}
	content := strings.Join(lines, "\n") + "\n"
	return overrideDir, overrideFile, m.writeFileAtomic(overrideFile, content)
}

func (m *CgroupManager) writeFileAtomic(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	return os.Rename(tmp, path)
}

func CalculateMemoryLimit(totalMemoryMB int, instanceCount int) int {
	if instanceCount <= 0 {
		instanceCount = 1
	}
	perInstance := totalMemoryMB * 70 / 100 / instanceCount
	if perInstance < 256 {
		perInstance = 256
	}
	return perInstance
}

func CalculateCPUQuota(cpuCores int) int {
	return cpuCores * 100
}
