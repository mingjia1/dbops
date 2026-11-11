//go:build linux

package collector

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// collectHostResourceMetrics: 采集 agent 所在主机的 CPU/内存/磁盘使用率 (%).
// 远端主机为 Linux (Ubuntu 22.04), 用 /proc + statfs 纯标准库实现, 不引入
// gopsutil 依赖. 任何一项读取失败只跳过该项, 不影响 MySQL status 指标返回.
func collectHostResourceMetrics(instanceID string) []Metric {
	now := time.Now()
	metrics := make([]Metric, 0, 3)
	if usage, err := cpuUsagePercent(); err == nil {
		metrics = append(metrics, Metric{InstanceID: instanceID, Name: "cpu", Value: clampPercent(usage), Timestamp: now})
	}
	if usage, err := memoryUsagePercent(); err == nil {
		metrics = append(metrics, Metric{InstanceID: instanceID, Name: "memory", Value: clampPercent(usage), Timestamp: now})
	}
	if usage, err := diskUsagePercent("/"); err == nil {
		metrics = append(metrics, Metric{InstanceID: instanceID, Name: "disk", Value: clampPercent(usage), Timestamp: now})
	}
	return metrics
}

func clampPercent(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// cpuUsagePercent: /proc/stat 首行是自启动以来的累计 jiffies, 两次短间隔采样
// 差分得到真实使用率: usage = (1 - dIdle/dTotal) * 100, idle 含 iowait.
func cpuUsagePercent() (float64, error) {
	idle1, total1, err := readCPUJiffies()
	if err != nil {
		return 0, err
	}
	time.Sleep(200 * time.Millisecond)
	idle2, total2, err := readCPUJiffies()
	if err != nil {
		return 0, err
	}
	dTotal := total2 - total1
	if dTotal <= 0 {
		return 0, err
	}
	return (1 - (idle2-idle1)/dTotal) * 100, nil
}

func readCPUJiffies() (idle, total float64, err error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		var sum, idleSum float64
		for i, f := range fields {
			v, perr := strconv.ParseFloat(f, 64)
			if perr != nil {
				return 0, 0, perr
			}
			sum += v
			// Linux /proc/stat 列序: user nice system idle iowait irq softirq steal...
			// idle 列 (i=3) 与 iowait 列 (i=4) 都算空闲.
			if i == 3 || i == 4 {
				idleSum += v
			}
		}
		return idleSum, sum, nil
	}
	return 0, 0, os.ErrNotExist
}

func memoryUsagePercent() (float64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	total, available := 0.0, -1.0
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = meminfoKB(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			available = meminfoKB(line)
		}
	}
	if total <= 0 || available < 0 {
		return 0, os.ErrInvalid
	}
	return (1 - available/total) * 100, nil
}

func meminfoKB(line string) float64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return -1
	}
	v, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return -1
	}
	return v
}

// diskUsagePercent: 指定挂载点的使用率 (%). 取 "/" (agent 与 MySQL 同机部署,
// 数据目录通常在根分区); LVM/独立数据盘可后续按实例扩展.
func diskUsagePercent(path string) (float64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	if st.Blocks == 0 {
		return 0, os.ErrInvalid
	}
	return float64(st.Blocks-st.Bfree) / float64(st.Blocks) * 100, nil
}
