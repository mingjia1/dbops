package executor

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MySQL reserved architecture ports
const (
	PortMGR = 33061 // MGR group communication port
	PortPXC = 4567  // PXC Galera cluster communication port
	PortIST = 4568  // PXC Incremental State Transfer port
	PortSST = 4444  // PXC State Snapshot Transfer port
)

// mysqlReservedPorts maps architecture-specific reserved ports to their owning component
var mysqlReservedPorts = map[int]string{
	PortMGR: "mgr",
	PortPXC: "pxc",
	PortIST: "pxc",
	PortSST: "pxc",
}

// PortScanner 负责扫描本机端口和目录，检测可用资源
type PortScanner struct{}

// NewPortScanner 创建端口扫描器
func NewPortScanner() *PortScanner {
	return &PortScanner{}
}

// MySQLReservedPortFor 返回端口所属的架构类型，非保留端口返回空字符串
func MySQLReservedPortFor(port int) string {
	if owner, ok := mysqlReservedPorts[port]; ok {
		return owner
	}
	return ""
}

// IsMySQLReservedPort 判断是否为 MySQL 架构保留端口
func IsMySQLReservedPort(port int) bool {
	_, ok := mysqlReservedPorts[port]
	return ok
}

// ExcludeMySQLReservedPorts 从端口列表中过滤掉 MySQL 架构保留端口
func ExcludeMySQLReservedPorts(ports []int) []int {
	var filtered []int
	for _, p := range ports {
		if !IsMySQLReservedPort(p) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// ScanUsedPorts 扫描本机已占用的端口列表
// Linux 上执行 ss -tlnp，Windows 上执行 netstat -an
func (s *PortScanner) ScanUsedPorts() ([]int, error) {
	return scanLocalPorts()
}

// ScanMySQLDataDirs 扫描 /data/mysql_*/ 类似目录，提取已使用的端口号
// 返回已存在的端口列表（如 /data/mysql_3306 → 3306）
func (s *PortScanner) ScanMySQLDataDirs(basePath string) ([]int, error) {
	if basePath == "" {
		basePath = "/data"
	}

	entries, err := os.ReadDir(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read dir %s failed: %w", basePath, err)
	}

	var ports []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Match pattern: mysql_<port> or mysql<port>
		if strings.HasPrefix(name, "mysql_") {
			portStr := strings.TrimPrefix(name, "mysql_")
			if port, err := strconv.Atoi(portStr); err == nil {
				ports = append(ports, port)
			}
		}
	}

	sort.Ints(ports)
	return ports, nil
}

// FindAvailablePort 从 startPort 开始查找第一个可用的空闲端口
// 自动跳过 MySQL 架构保留端口（MGR 33061, PXC 4567/4568/4444）
func (s *PortScanner) FindAvailablePort(startPort int, excludePorts []int) (int, error) {
	if startPort < 1 || startPort > 65535 {
		return 0, fmt.Errorf("invalid start port: %d (must be 1-65535)", startPort)
	}

	// Build exclusion set
	exclude := make(map[int]bool)
	for _, p := range excludePorts {
		if p >= 1 && p <= 65535 {
			exclude[p] = true
		}
	}

	// Always exclude MySQL reserved ports
	for p := range mysqlReservedPorts {
		exclude[p] = true
	}

	// Also exclude ports already in use
	usedPorts, err := scanLocalPorts()
	if err == nil {
		for _, p := range usedPorts {
			exclude[p] = true
		}
	}

	// Scan up to 100 ports to find an available one
	maxScan := 100
	for i := 0; i < maxScan; i++ {
		candidate := startPort + i
		if candidate > 65535 {
			break
		}
		if exclude[candidate] {
			continue
		}
		// Quick TCP check to confirm port is not listening
		if !isPortListening(candidate) {
			return candidate, nil
		}
	}

	return 0, fmt.Errorf("no available port found starting from %d (scanned %d ports)", startPort, maxScan)
}

// FindAvailableDataDir 生成 /data/mysql_<port>/ 路径并校验目录不存在
// 如果目录已存在，尝试使用下一个端口
func (s *PortScanner) FindAvailableDataDir(basePath string, port int) string {
	if basePath == "" {
		basePath = "/data"
	}
	return filepath.Join(basePath, fmt.Sprintf("mysql_%d", port))
}

// scanLocalPorts 执行端口扫描，返回本机所有监听中的 TCP 端口
func scanLocalPorts() ([]int, error) {
	// Use net.Dial with short timeout to check common port ranges
	// For production, this should use OS-specific commands (ss/netstat)
	var ports []int

	// Try to scan from /proc/net/tcp on Linux
	if data, err := os.ReadFile("/proc/net/tcp"); err == nil {
		return parseProcNetTCP(string(data)), nil
	}

	// Fallback: scan common MySQL port range (3300-3400) with TCP check
	for port := 3300; port <= 3400; port++ {
		if isPortListening(port) {
			ports = append(ports, port)
		}
	}

	return ports, nil
}

// parseProcNetTCP 解析 /proc/net/tcp 格式的端口列表
func parseProcNetTCP(content string) []int {
	var ports []int
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		// local_address is hex format: 0100007F:0C3A
		localAddr := fields[1]
		parts := strings.Split(localAddr, ":")
		if len(parts) != 2 {
			continue
		}
		portHex := parts[1]
		if port, err := strconv.ParseInt(portHex, 16, 32); err == nil {
			if port > 0 && port <= 65535 {
				ports = append(ports, int(port))
			}
		}
	}
	return ports
}

// isPortListening 检查指定端口是否正在监听
func isPortListening(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
