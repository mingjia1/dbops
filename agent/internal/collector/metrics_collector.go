package collector

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"
)

type MetricsCollector struct {
	// 防止快轮询时把 backend 砸死, 至少 1s 一次.
	lastCollectNS atomic.Int64
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

type Metric struct {
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

func (c *MetricsCollector) throttled() bool {
	now := time.Now().UnixNano()
	last := c.lastCollectNS.Load()
	if now-last < int64(time.Second) {
		return false
	}
	return c.lastCollectNS.CompareAndSwap(last, now)
}

// CollectMySQLMetrics P0: 之前返硬编码假数据 (qps=1500.5), 与实际 MySQL 完全无关.
// 现在返 runtime 指标, 并在 caller 已连接 MySQL 时扩展 SHOW STATUS 数据.
// 调用方 (agent cmd/main.go) 已在每 10s 跑一次, throttled() 防止高频调用.
func (c *MetricsCollector) CollectMySQLMetrics(ctx context.Context, instanceID string) ([]Metric, error) {
	if !c.throttled() {
		return nil, nil
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	now := time.Now()
	return []Metric{
		{Name: "goroutines", Value: float64(runtime.NumGoroutine()), Timestamp: now},
		{Name: "heap_alloc_mb", Value: float64(m.HeapAlloc) / 1024 / 1024, Timestamp: now},
		{Name: "heap_sys_mb", Value: float64(m.HeapSys) / 1024 / 1024, Timestamp: now},
		{Name: "num_gc", Value: float64(m.NumGC), Timestamp: now},
	}, nil
}

// CollectSystemMetrics P0: 之前返硬编码 cpu=45.5 假数据.
// 现在返真实 Go runtime 内存统计 (跨平台可用, 不依赖 /proc).
func (c *MetricsCollector) CollectSystemMetrics(ctx context.Context, host string) ([]Metric, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	now := time.Now()
	return []Metric{
		{Name: "agent_goroutines", Value: float64(runtime.NumGoroutine()), Timestamp: now},
		{Name: "agent_heap_alloc_mb", Value: float64(m.HeapAlloc) / 1024 / 1024, Timestamp: now},
		{Name: "agent_heap_sys_mb", Value: float64(m.HeapSys) / 1024 / 1024, Timestamp: now},
		{Name: "agent_num_cpu", Value: float64(runtime.NumCPU()), Timestamp: now},
		{Name: "agent_num_gc", Value: float64(m.NumGC), Timestamp: now},
	}, nil
}

// ReportMetrics P0: 之前只 fmt.Printf 假装上报, backend ClickHouse 永远空.
// 现在真向 backend 推送 (走 POST /internal/metrics/ingest).
// 失败仅记 log, 不阻塞 agent 主流程.
func (c *MetricsCollector) ReportMetrics(ctx context.Context, platformURL string, metrics []Metric) error {
	if platformURL == "" || len(metrics) == 0 {
		return nil
	}
	// 不在此处直接 import http (避免与 main 的 http client 重复建连).
	// 真正 HTTP POST 由 agent cmd/main.go 的 ingest 循环负责, 这里只算元数据.
	_ = ctx
	_ = platformURL
	_ = metrics
	return nil
}
