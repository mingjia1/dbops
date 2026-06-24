package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// EnvironmentChecker 负责检查主机环境
type EnvironmentChecker struct {
	toolInstaller *ToolInstaller
}

// NewEnvironmentChecker 创建环境检查器
func NewEnvironmentChecker() *EnvironmentChecker {
	return &EnvironmentChecker{
		toolInstaller: NewToolInstaller(),
	}
}

// EnvironmentCheckRequest 环境检查请求
type EnvironmentCheckRequest struct {
	CheckTools     bool `json:"check_tools"`      // 是否检查工具
	CheckResources bool `json:"check_resources"`  // 是否检查资源
	CheckNetwork   bool `json:"check_network"`    // 是否检查网络
}

// EnvironmentCheckResult 环境检查结果
type EnvironmentCheckResult struct {
	Status    string                 `json:"status"`     // success, warning, failed
	Message   string                 `json:"message"`
	OS        OSInfo                 `json:"os"`
	Tools     ToolsInfo              `json:"tools"`
	Resources ResourcesInfo          `json:"resources"`
	Network   NetworkInfo            `json:"network"`
	Issues    []string               `json:"issues"`     // 发现的问题列表
}

// OSInfo 操作系统信息
type OSInfo struct {
	Type         string `json:"type"`          // linux, windows, darwin
	Distribution string `json:"distribution"`  // ubuntu, centos, etc.
	Version      string `json:"version"`
	Arch         string `json:"arch"`
}

// ToolsInfo 工具检查信息
type ToolsInfo struct {
	MySQL       ToolStatus `json:"mysql"`
	MySQLD      ToolStatus `json:"mysqld"`
	XtraBackup  ToolStatus `json:"xtrabackup"`
	AllReady    bool       `json:"all_ready"`
}

// ToolStatus 工具状态
type ToolStatus struct {
	Available bool   `json:"available"`
	Path      string `json:"path,omitempty"`
	Version   string `json:"version,omitempty"`
}

// ResourcesInfo 资源信息
type ResourcesInfo struct {
	CPUCores     int    `json:"cpu_cores"`
	MemoryMB     int    `json:"memory_mb"`
	DiskSpaceGB  int    `json:"disk_space_gb"`
	Sufficient   bool   `json:"sufficient"`
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	CanReachInternet bool   `json:"can_reach_internet"`
	Hostname         string `json:"hostname"`
}

// CheckEnvironment 执行环境检查
func (e *EnvironmentChecker) CheckEnvironment(ctx context.Context, req EnvironmentCheckRequest) *EnvironmentCheckResult {
	result := &EnvironmentCheckResult{
		Status: "success",
		Issues: make([]string, 0),
	}

	// 1. 检查操作系统信息
	result.OS = e.checkOS()

	// 2. 检查工具
	if req.CheckTools {
		result.Tools = e.checkTools()
		if !result.Tools.AllReady {
			result.Status = "warning"
			result.Issues = append(result.Issues, "Some required MySQL tools are missing")
		}
	}

	// 3. 检查资源
	if req.CheckResources {
		result.Resources = e.checkResources()
		if !result.Resources.Sufficient {
			result.Status = "warning"
			result.Issues = append(result.Issues, "System resources may be insufficient for MySQL deployment")
		}
	}

	// 4. 检查网络
	if req.CheckNetwork {
		result.Network = e.checkNetwork()
		if !result.Network.CanReachInternet {
			result.Issues = append(result.Issues, "Cannot reach internet (may affect package installation)")
		}
	}

	// 5. 生成消息
	if len(result.Issues) == 0 {
		result.Message = "Environment check passed: all requirements met"
	} else {
		result.Message = fmt.Sprintf("Environment check completed with %d issue(s)", len(result.Issues))
	}

	return result
}

// checkOS 检查操作系统信息
func (e *EnvironmentChecker) checkOS() OSInfo {
	info := OSInfo{
		Type: runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	switch runtime.GOOS {
	case "linux":
		// 读取 /etc/os-release
		if data, err := os.ReadFile("/etc/os-release"); err == nil {
			content := string(data)

			// 提取发行版
			if strings.Contains(content, "Ubuntu") {
				info.Distribution = "ubuntu"
			} else if strings.Contains(content, "Debian") {
				info.Distribution = "debian"
			} else if strings.Contains(content, "CentOS") {
				info.Distribution = "centos"
			} else if strings.Contains(content, "Red Hat") {
				info.Distribution = "rhel"
			} else if strings.Contains(content, "Rocky") {
				info.Distribution = "rocky"
			}

			// 提取版本号
			for _, line := range strings.Split(content, "\n") {
				if strings.HasPrefix(line, "VERSION_ID=") {
					info.Version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
					break
				}
			}
		}

	case "darwin":
		info.Distribution = "macos"
		if output, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
			info.Version = strings.TrimSpace(string(output))
		}

	case "windows":
		info.Distribution = "windows"
		if output, err := exec.Command("cmd", "/c", "ver").Output(); err == nil {
			info.Version = strings.TrimSpace(string(output))
		}
	}

	return info
}

// checkTools 检查MySQL工具
func (e *EnvironmentChecker) checkTools() ToolsInfo {
	info := ToolsInfo{}

	// 检查mysql客户端
	info.MySQL = e.checkTool("mysql")

	// 检查mysqld服务器
	info.MySQLD = e.checkTool("mysqld")

	// 检查xtrabackup
	info.XtraBackup = e.checkTool("xtrabackup")

	// 判断是否所有工具都就绪
	info.AllReady = info.MySQL.Available && info.MySQLD.Available && info.XtraBackup.Available

	return info
}

// checkTool 检查单个工具
func (e *EnvironmentChecker) checkTool(tool string) ToolStatus {
	status := ToolStatus{Available: false}

	// 检查工具是否存在
	path := e.toolInstaller.checkToolInstalled(tool)
	if path == "" {
		return status
	}

	// Verify the binary is actually executable (not blocked by missing shared libs)
	version := e.getToolVersion(tool, path)
	if version == "" {
		// Binary exists but can't run — report it as found but broken
		status.Path = path
		status.Available = false
		return status
	}

	status.Available = true
	status.Path = path
	status.Version = version

	return status
}

// getToolVersion 获取工具版本
func (e *EnvironmentChecker) getToolVersion(tool string, path string) string {
	var cmd *exec.Cmd

	switch tool {
	case "mysql":
		cmd = exec.Command(path, "--version")
	case "mysqld":
		cmd = exec.Command(path, "--version")
	case "xtrabackup":
		cmd = exec.Command(path, "--version")
	default:
		return ""
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	// 从输出中提取版本号
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		// 简单处理：取第一行，去掉工具名后的部分
		if idx := strings.Index(line, "Ver "); idx >= 0 {
			line = line[idx+4:]
			if idx := strings.Index(line, " "); idx >= 0 {
				line = line[:idx]
			}
		}
		return line
	}

	return ""
}

// checkResources 检查系统资源
func (e *EnvironmentChecker) checkResources() ResourcesInfo {
	info := ResourcesInfo{
		CPUCores: runtime.NumCPU(),
	}

	switch runtime.GOOS {
	case "linux":
		// 检查内存
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			content := string(data)
			for _, line := range strings.Split(content, "\n") {
				if strings.HasPrefix(line, "MemTotal:") {
					// MemTotal:        8192000 kB
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						var memKB int
						fmt.Sscanf(parts[1], "%d", &memKB)
						info.MemoryMB = memKB / 1024
					}
					break
				}
			}
		}

		// 检查磁盘空间 (/)
		if output, err := exec.Command("df", "-BG", "/").Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 4 {
					// 可用空间在第4列
					var diskGB int
					fmt.Sscanf(strings.TrimSuffix(fields[3], "G"), "%d", &diskGB)
					info.DiskSpaceGB = diskGB
				}
			}
		}

	case "darwin":
		// macOS内存检查
		if output, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
			var memBytes int64
			fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &memBytes)
			info.MemoryMB = int(memBytes / 1024 / 1024)
		}

		// macOS磁盘空间检查
		if output, err := exec.Command("df", "-g", "/").Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				fields := strings.Fields(lines[1])
				if len(fields) >= 4 {
					fmt.Sscanf(fields[3], "%d", &info.DiskSpaceGB)
				}
			}
		}

	case "windows":
		// Windows内存检查
		if output, err := exec.Command("wmic", "computersystem", "get", "TotalPhysicalMemory").Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				var memBytes int64
				fmt.Sscanf(strings.TrimSpace(lines[1]), "%d", &memBytes)
				info.MemoryMB = int(memBytes / 1024 / 1024)
			}
		}

		// Windows磁盘空间检查
		if output, err := exec.Command("wmic", "logicaldisk", "where", "DeviceID='C:'", "get", "FreeSpace").Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			if len(lines) >= 2 {
				var freeBytes int64
				fmt.Sscanf(strings.TrimSpace(lines[1]), "%d", &freeBytes)
				info.DiskSpaceGB = int(freeBytes / 1024 / 1024 / 1024)
			}
		}
	}

	// 判断资源是否充足
	// 最低要求：2核、2GB内存、10GB磁盘空间
	info.Sufficient = info.CPUCores >= 2 && info.MemoryMB >= 2048 && info.DiskSpaceGB >= 10

	return info
}

// checkNetwork 检查网络连接
func (e *EnvironmentChecker) checkNetwork() NetworkInfo {
	info := NetworkInfo{}

	// 获取主机名
	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}

	// 检查是否能访问互联网（ping常见的DNS服务器）
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("ping", "-n", "1", "-w", "2000", "8.8.8.8")
	default:
		cmd = exec.Command("ping", "-c", "1", "-W", "2", "8.8.8.8")
	}

	if err := cmd.Run(); err == nil {
		info.CanReachInternet = true
	}

	return info
}
