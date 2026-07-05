package executor

import (
	"os"
	"path/filepath"
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

func TestMySQLRuntimeOptionsForPXCUsrSbinMysqld(t *testing.T) {
	root := t.TempDir()
	basedir := filepath.Join(root, "opt", "dbops-pxc", "usr")
	mysqld := filepath.Join(basedir, "sbin", "mysqld")
	pluginDir := filepath.Join(basedir, "lib64", "mysql", "plugin")

	if err := os.MkdirAll(filepath.Dir(mysqld), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mysqld, []byte(""), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "group_replication.so"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	opts := mysqlRuntimeOptionsFor(mysqld)
	if opts.Basedir != basedir {
		t.Fatalf("basedir = %q, want %q", opts.Basedir, basedir)
	}
	if opts.PluginDir != pluginDir {
		t.Fatalf("plugin dir = %q, want %q", opts.PluginDir, pluginDir)
	}
	if opts.SecureFileDir != "/var/lib/mysql-files" {
		t.Fatalf("secure file dir = %q", opts.SecureFileDir)
	}
}
