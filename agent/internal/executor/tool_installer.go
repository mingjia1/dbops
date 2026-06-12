package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ToolInstaller 负责自动安装MySQL管理工具
type ToolInstaller struct {
	installDir string // 工具安装目录
}

// NewToolInstaller 创建工具安装器
func NewToolInstaller() *ToolInstaller {
	// 根据操作系统确定安装目录
	installDir := "/opt/dbops-tools"
	if runtime.GOOS == "windows" {
		installDir = "C:\\dbops-tools"
	}
	return &ToolInstaller{
		installDir: installDir,
	}
}

// InstallToolsRequest 安装工具请求
type InstallToolsRequest struct {
	Tools       []string `json:"tools"`        // 要安装的工具列表: mysql, mysqld, xtrabackup
	MySQLVersion string  `json:"mysql_version"` // MySQL版本，如 "8.0"
	Force       bool     `json:"force"`        // 是否强制重新安装
}

// InstallToolsResult 安装结果
type InstallToolsResult struct {
	TaskID    string            `json:"task_id"`
	Status    string            `json:"status"`
	Message   string            `json:"message"`
	Installed map[string]string `json:"installed"` // 工具名 -> 安装路径
	Failed    map[string]string `json:"failed"`    // 工具名 -> 失败原因
}

// InstallTools 安装MySQL管理工具
func (t *ToolInstaller) InstallTools(ctx context.Context, req InstallToolsRequest) *InstallToolsResult {
	result := &InstallToolsResult{
		Status:    "running",
		Installed: make(map[string]string),
		Failed:    make(map[string]string),
	}

	// 确保安装目录存在
	if err := os.MkdirAll(t.installDir, 0755); err != nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("Failed to create install directory: %v", err)
		return result
	}

	// 获取操作系统信息
	osType := runtime.GOOS
	_ = runtime.GOARCH // 架构信息暂未使用

	// 处理每个工具
	for _, tool := range req.Tools {
		// 检查工具是否已安装
		if !req.Force {
			if path := t.checkToolInstalled(tool); path != "" {
				result.Installed[tool] = path
				continue
			}
		}

		// 根据操作系统和工具类型安装
		var err error
		var installPath string

		switch osType {
		case "linux":
			installPath, err = t.installToolLinux(ctx, tool, req.MySQLVersion)
		case "windows":
			installPath, err = t.installToolWindows(ctx, tool, req.MySQLVersion)
		case "darwin":
			installPath, err = t.installToolMacOS(ctx, tool, req.MySQLVersion)
		default:
			err = fmt.Errorf("unsupported OS: %s", osType)
		}

		if err != nil {
			result.Failed[tool] = err.Error()
		} else {
			result.Installed[tool] = installPath
		}
	}

	// 确定最终状态
	if len(result.Failed) == 0 {
		result.Status = "success"
		result.Message = fmt.Sprintf("Successfully installed %d tools", len(result.Installed))
	} else if len(result.Installed) > 0 {
		result.Status = "partial"
		result.Message = fmt.Sprintf("Installed %d tools, %d failed", len(result.Installed), len(result.Failed))
	} else {
		result.Status = "failed"
		result.Message = "All tools installation failed"
	}

	return result
}

// checkToolInstalled 检查工具是否已安装
func (t *ToolInstaller) checkToolInstalled(tool string) string {
	// 首先检查系统PATH
	path, err := exec.LookPath(tool)
	if err == nil {
		return path
	}

	// 检查安装目录
	candidates := []string{
		filepath.Join(t.installDir, "bin", tool),
		filepath.Join(t.installDir, "bin", tool+".exe"),
		filepath.Join(t.installDir, tool),
		filepath.Join(t.installDir, tool+".exe"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// installToolLinux 在Linux上安装工具
func (t *ToolInstaller) installToolLinux(ctx context.Context, tool string, mysqlVersion string) (string, error) {
	// 检测Linux发行版
	distro := t.detectLinuxDistro()

	switch tool {
	case "mysql", "mysqld":
		return t.installMySQLLinux(ctx, distro, mysqlVersion)
	case "xtrabackup":
		return t.installXtraBackupLinux(ctx, distro, mysqlVersion)
	default:
		return "", fmt.Errorf("unknown tool: %s", tool)
	}
}

// detectLinuxDistro 检测Linux发行版
func (t *ToolInstaller) detectLinuxDistro() string {
	// 读取 /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}

	content := string(data)
	if strings.Contains(content, "Ubuntu") || strings.Contains(content, "Debian") {
		return "debian"
	}
	if strings.Contains(content, "CentOS") || strings.Contains(content, "Red Hat") || strings.Contains(content, "Rocky") {
		return "rhel"
	}

	return "unknown"
}

// installMySQLLinux 在Linux上安装MySQL
func (t *ToolInstaller) installMySQLLinux(ctx context.Context, distro string, version string) (string, error) {
	var cmd *exec.Cmd

	switch distro {
	case "debian":
		// Ubuntu/Debian
		if version == "" {
			version = "8.0"
		}
		cmd = exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
			export DEBIAN_FRONTEND=noninteractive
			apt-get update -qq
			apt-get install -y -qq mysql-server-%s mysql-client-%s
		`, version, version))

	case "rhel":
		// CentOS/RHEL
		cmd = exec.CommandContext(ctx, "bash", "-c", `
			yum install -y mysql-server mysql
		`)

	default:
		return "", fmt.Errorf("unsupported Linux distribution: %s", distro)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("installation failed: %v, output: %s", err, string(output))
	}

	// 验证安装
	path, err := exec.LookPath("mysqld")
	if err != nil {
		return "", fmt.Errorf("mysql installed but mysqld not found in PATH")
	}

	return path, nil
}

// installXtraBackupLinux 在Linux上安装Percona XtraBackup
func (t *ToolInstaller) installXtraBackupLinux(ctx context.Context, distro string, version string) (string, error) {
	var cmd *exec.Cmd

	switch distro {
	case "debian":
		// Ubuntu/Debian
		if version == "" {
			version = "80" // percona-xtrabackup-80
		} else {
			version = strings.ReplaceAll(version, ".", "")
		}
		cmd = exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf(`
			export DEBIAN_FRONTEND=noninteractive
			codename=$(lsb_release -sc)
			wget -q https://repo.percona.com/apt/percona-release_latest.${codename}_all.deb
			dpkg -i percona-release_latest.${codename}_all.deb 2>/dev/null || true
			rm -f percona-release_latest.${codename}_all.deb
			apt-get update -qq
			apt-get install -y -qq percona-xtrabackup-%s
		`, version))

	case "rhel":
		// CentOS/RHEL
		cmd = exec.CommandContext(ctx, "bash", "-c", `
			yum install -y https://repo.percona.com/yum/percona-release-latest.noarch.rpm
			percona-release enable-only tools release
			yum install -y percona-xtrabackup-80
		`)

	default:
		return "", fmt.Errorf("unsupported Linux distribution: %s", distro)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("installation failed: %v, output: %s", err, string(output))
	}

	// 验证安装
	path, err := exec.LookPath("xtrabackup")
	if err != nil {
		return "", fmt.Errorf("xtrabackup installed but not found in PATH")
	}

	return path, nil
}

// installToolWindows 在Windows上安装工具
func (t *ToolInstaller) installToolWindows(ctx context.Context, tool string, mysqlVersion string) (string, error) {
	// Windows上需要手动下载并解压MySQL二进制包
	// 这里提供说明而不是自动安装，因为Windows没有包管理器

	switch tool {
	case "mysql", "mysqld":
		return "", fmt.Errorf("MySQL installation on Windows requires manual setup. Please:\n" +
			"1. Download MySQL Community Server from https://dev.mysql.com/downloads/mysql/\n" +
			"2. Extract to C:\\mysql\n" +
			"3. Add C:\\mysql\\bin to PATH\n" +
			"Or use Chocolatey: choco install mysql")

	case "xtrabackup":
		return "", fmt.Errorf("XtraBackup installation on Windows requires manual setup. Please:\n" +
			"1. Download Percona XtraBackup from https://www.percona.com/downloads/\n" +
			"2. Extract to C:\\xtrabackup\n" +
			"3. Add C:\\xtrabackup\\bin to PATH")

	default:
		return "", fmt.Errorf("unknown tool: %s", tool)
	}
}

// installToolMacOS 在macOS上安装工具
func (t *ToolInstaller) installToolMacOS(ctx context.Context, tool string, mysqlVersion string) (string, error) {
	// 检查Homebrew是否安装
	if _, err := exec.LookPath("brew"); err != nil {
		return "", fmt.Errorf("Homebrew not found. Please install from https://brew.sh")
	}

	var cmd *exec.Cmd

	switch tool {
	case "mysql", "mysqld":
		if mysqlVersion == "" {
			cmd = exec.CommandContext(ctx, "brew", "install", "mysql")
		} else {
			cmd = exec.CommandContext(ctx, "brew", "install", "mysql@"+mysqlVersion)
		}

	case "xtrabackup":
		cmd = exec.CommandContext(ctx, "brew", "install", "percona-xtrabackup")

	default:
		return "", fmt.Errorf("unknown tool: %s", tool)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("brew install failed: %v, output: %s", err, string(output))
	}

	// 验证安装
	path, err := exec.LookPath(tool)
	if err != nil {
		return "", fmt.Errorf("%s installed but not found in PATH", tool)
	}

	return path, nil
}

// CheckToolsAvailable 检查所需工具是否可用
func (t *ToolInstaller) CheckToolsAvailable(tools []string) map[string]bool {
	result := make(map[string]bool)
	for _, tool := range tools {
		result[tool] = t.checkToolInstalled(tool) != ""
	}
	return result
}
