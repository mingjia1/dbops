package executor

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"
)

// EnvironmentChecker 负责检查主机环境（仅使用平台API，不调用外部命令）
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

// CheckEnvironment 执行环境检查（仅使用平台API）
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

// checkOS 检查操作系统信息（仅使用文件API）
func (e *EnvironmentChecker) checkOS() OSInfo {
	info := OSInfo{
		Type: runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	switch runtime.GOOS {
	case "linux":
		// 读取 /etc/os-release（纯文件读取，无外部命令）
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
		// 读取系统版本信息文件
		if data, err := os.ReadFile("/System/Library/CoreServices/SystemVersion.plist"); err == nil {
			content := string(data)
			if idx := strings.Index(content, "<string>"); idx >= 0 {
				endIdx := strings.Index(content[idx:], "</string>")
				if endIdx > 0 {
					info.Version = content[idx+8 : idx+endIdx]
				}
			}
		}

	case "windows":
		info.Distribution = "windows"
	}

	return info
}

// checkTools 检查MySQL工具（仅检查文件是否存在）
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

// checkTool 检查单个工具（使用平台API检查可执行性，不执行外部命令）
func (e *EnvironmentChecker) checkTool(tool string) ToolStatus {
	status := ToolStatus{Available: false}

	// 检查工具是否存在
	path := e.toolInstaller.checkToolInstalled(tool)
	if path == "" {
		return status
	}

	// 检查文件是否存在且可执行（os.Access 在 Unix 上检查 X_OK 位，不执行任何命令）
	if info, err := os.Stat(path); err == nil {
		// 检查文件是否具有可执行权限
		executable := false
		if isExecutableMode(info.Mode()) {
			executable = true
		}

		status.Path = path
		status.Available = executable
		if executable {
			status.Version = "available"
		}
	}

	return status
}

// isExecutableMode 检查文件模式是否包含可执行位
func isExecutableMode(mode os.FileMode) bool {
	// 检查所有者、组或其他的可执行位
	return mode&0111 != 0
}

// checkResources 检查系统资源（仅使用平台API）
func (e *EnvironmentChecker) checkResources() ResourcesInfo {
	info := ResourcesInfo{
		CPUCores: runtime.NumCPU(),
	}

	// 获取内存和磁盘信息（平台特定实现）
	e.checkResourcesPlatform(&info)

	// 判断资源是否充足
	// 最低要求：2核、2GB内存、10GB磁盘空间
	info.Sufficient = info.CPUCores >= 2 && info.MemoryMB >= 2048 && info.DiskSpaceGB >= 10

	return info
}

// checkNetwork 检查网络连接（使用net包，无外部命令）
func (e *EnvironmentChecker) checkNetwork() NetworkInfo {
	info := NetworkInfo{}

	// 获取主机名
	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}

	// 检查是否能访问互联网（使用TCP连接检查，无外部命令）
	// 尝试连接到DNS服务器的53端口
	addr := "8.8.8.8:53"
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err == nil {
		conn.Close()
		info.CanReachInternet = true
	}

	return info
}
