//go:build linux

package executor

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// checkResourcesPlatform 检查系统资源（Linux平台实现）
func (e *EnvironmentChecker) checkResourcesPlatform(info *ResourcesInfo) {
	// 检查内存（读取/proc/meminfo）
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

	// 检查磁盘空间（使用syscall.Statfs，无外部命令）
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		// 可用空间 = 块大小 * 可用块数
		available := stat.Bavail * uint64(stat.Bsize)
		info.DiskSpaceGB = int(available / (1024 * 1024 * 1024))
	}
}
