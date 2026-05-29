package collector

import (
	"context"
	"fmt"
	"time"
)

type MetricsCollector struct {
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

type Metric struct {
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

func (c *MetricsCollector) CollectMySQLMetrics(ctx context.Context, instanceID string) ([]Metric, error) {
	return []Metric{
		{Name: "qps", Value: 1500.5, Timestamp: time.Now()},
		{Name: "tps", Value: 200.3, Timestamp: time.Now()},
		{Name: "threads_connected", Value: 50.0, Timestamp: time.Now()},
		{Name: "threads_running", Value: 5.0, Timestamp: time.Now()},
		{Name: "buffer_pool_usage", Value: 75.5, Timestamp: time.Now()},
		{Name: "slow_queries", Value: 10.0, Timestamp: time.Now()},
	}, nil
}

func (c *MetricsCollector) CollectSystemMetrics(ctx context.Context, host string) ([]Metric, error) {
	return []Metric{
		{Name: "cpu_usage", Value: 45.5, Timestamp: time.Now()},
		{Name: "memory_usage", Value: 60.3, Timestamp: time.Now()},
		{Name: "disk_io_read", Value: 1024.0, Timestamp: time.Now()},
		{Name: "disk_io_write", Value: 512.0, Timestamp: time.Now()},
		{Name: "network_in", Value: 100.0, Timestamp: time.Now()},
		{Name: "network_out", Value: 80.0, Timestamp: time.Now()},
	}, nil
}

func (c *MetricsCollector) ReportMetrics(ctx context.Context, platformURL string, metrics []Metric) error {
	fmt.Printf("Reporting %d metrics to %s\n", len(metrics), platformURL)
	return nil
}