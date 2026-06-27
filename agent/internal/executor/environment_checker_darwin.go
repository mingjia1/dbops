//go:build darwin

package executor

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// checkResourcesPlatform 检查系统资源（macOS平台实现）
func (e *EnvironmentChecker) checkResourcesPlatform(info *ResourcesInfo) {
	// macOS内存检查 - 通过 sysctlbyname("hw.memsize") 获取
	info.MemoryMB = getDarwinMemoryMB()

	// macOS磁盘空间检查（使用syscall.Statfs）
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		available := stat.Bavail * uint64(stat.Bsize)
		info.DiskSpaceGB = int(available / (1024 * 1024 * 1024))
	}
}

// getDarwinMemoryMB 获取macOS内存大小（MB）
// 使用 Go 标准库 syscall.SysctlUint64 获取 hw.memsize，无需外部命令
func getDarwinMemoryMB() int {
	memBytes, err := syscall.SysctlUint64("hw.memsize")
	if err == nil {
		return int(memBytes / (1024 * 1024))
	}

	// 备选：尝试读取 /proc/meminfo（某些环境可能有）
	if data, readErr := os.ReadFile("/proc/meminfo"); readErr == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					var memKB int
					fmt.Sscanf(parts[1], "%d", &memKB)
					return memKB / 1024
				}
			}
		}
	}

	return 0
}
