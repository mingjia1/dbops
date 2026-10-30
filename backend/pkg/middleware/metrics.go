package middleware

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsStats P1-9: /metrics 需要采集的运行时统计来源.
// 由 main.go 装配时注入, 避免本包反向依赖 repositories/services.
type MetricsStats struct {
	// DBMaxOpen / DBOpen / DBInUse / DBIdle: sql.DB 连接池统计 (可为 nil, 跳过输出).
	DBMaxOpen, DBOpen, DBInUse, DBIdle func() int
	// DBWaitCount / DBWaitDuration: 连接池等待统计 (可为 nil).
	DBWaitCount    func() int64
	DBWaitDuration func() time.Duration
	// StartTime: 进程启动时间 (uptime 计算).
	StartTime time.Time
	// Version: 平台版本号.
	Version string
	// P0-B: 元数据库定时快照统计 (可为 nil, 跳过输出).
	DBBackupSuccessCount func() int64
	DBBackupFailureCount func() int64
	// DBBackupLastSuccess: 最近一次成功快照的 unix 秒, 0 表示从未成功.
	DBBackupLastSuccess func() int64
}

// MetricsHandler 返回 Prometheus text/plain; version=0.0.4 格式的 /metrics 处理器.
// 只输出聚合数值 (内存/句柄/连接池), 不含任何凭据或用户数据, 可匿名抓取;
// 若部署在公网建议由反向代理限制来源.
func MetricsHandler(stats MetricsStats) gin.HandlerFunc {
	return func(c *gin.Context) {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)

		w := c.Writer
		h := w.Header()
		h.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		write := func(name, help, mtype string, value float64) {
			fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s %s\n",
				name, help, name, mtype, name, formatFloat(value))
		}

		write("dbops_process_uptime_seconds", "Seconds since process start.", "gauge",
			time.Since(stats.StartTime).Seconds())
		write("dbops_build_info", "Build metadata.", "gauge", 1) // labels via extra line
		fmt.Fprintf(w, "dbops_build_info{version=%q} 1\n", stats.Version)

		write("dbops_goroutines", "Number of goroutines.", "gauge", float64(runtime.NumGoroutine()))
		write("dbops_memory_alloc_bytes", "Bytes allocated and not yet freed.", "gauge", float64(ms.Alloc))
		write("dbops_memory_sys_bytes", "Bytes obtained from system.", "gauge", float64(ms.Sys))
		write("dbops_gc_runs_total", "Number of completed GC cycles.", "counter", float64(ms.NumGC))
		write("dbops_threads", "Number of OS threads.", "gauge", float64(runtime.NumCPU()))

		if stats.DBOpen != nil {
			write("dbops_db_open_connections", "Open DB connections.", "gauge", float64(stats.DBOpen()))
		}
		if stats.DBInUse != nil {
			write("dbops_db_in_use_connections", "Connections currently in use.", "gauge", float64(stats.DBInUse()))
		}
		if stats.DBIdle != nil {
			write("dbops_db_idle_connections", "Idle DB connections.", "gauge", float64(stats.DBIdle()))
		}
		if stats.DBMaxOpen != nil {
			write("dbops_db_max_open_connections", "Max open DB connections (0=unlimited).", "gauge", float64(stats.DBMaxOpen()))
		}
		if stats.DBWaitCount != nil {
			write("dbops_db_wait_count_total", "Total connections waited for.", "counter", float64(stats.DBWaitCount()))
		}
		if stats.DBWaitDuration != nil {
			write("dbops_db_wait_duration_seconds_total", "Total time waited for connections.", "counter", stats.DBWaitDuration().Seconds())
		}
		if stats.DBBackupSuccessCount != nil {
			write("dbops_db_backup_success_total", "Successful platform metadata DB snapshots.", "counter", float64(stats.DBBackupSuccessCount()))
		}
		if stats.DBBackupFailureCount != nil {
			write("dbops_db_backup_failure_total", "Failed platform metadata DB snapshots.", "counter", float64(stats.DBBackupFailureCount()))
		}
		if stats.DBBackupLastSuccess != nil {
			write("dbops_db_backup_last_success_timestamp_seconds", "Unix time of last successful metadata DB snapshot.", "gauge", float64(stats.DBBackupLastSuccess()))
		}
	}
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
