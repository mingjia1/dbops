package executor

import (
	"testing"
)

func TestCheckToolsAvailable(t *testing.T) {
	installer := NewToolInstaller()

	tools := []string{"mysql", "mysqld", "xtrabackup"}
	result := installer.CheckToolsAvailable(tools)

	t.Logf("Tool availability check:")
	for tool, available := range result {
		status := "❌ Not available"
		if available {
			status = "✅ Available"
		}
		t.Logf("  %s: %s", tool, status)
	}
}

func TestInstallToolsRequest(t *testing.T) {
	// 只测试结构，不实际安装
	req := InstallToolsRequest{
		Tools:        []string{"mysql", "xtrabackup"},
		MySQLVersion: "8.0",
		Force:        false,
	}

	t.Logf("Install request: %+v", req)

	// 测试工具检测
	installer := NewToolInstaller()
	path := installer.checkToolInstalled("go") // go应该存在
	if path != "" {
		t.Logf("Found Go at: %s", path)
	}
}

func TestDetectOSInfo(t *testing.T) {
	installer := NewToolInstaller()
	info := installer.detectOSInfo()
	t.Logf("Detected OS: %s %s (%s/%s), PackageManager: %s", info.Distribution, info.Version, info.OS, info.Arch, info.PackageManager)
}
