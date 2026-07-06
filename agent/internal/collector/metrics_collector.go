package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type MetricsCollector struct {
	lastCollectNS atomic.Int64
	httpClient    *http.Client
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

type Metric struct {
	InstanceID string    `json:"instance_id,omitempty"`
	Name       string    `json:"name"`
	Value      float64   `json:"value"`
	Timestamp  time.Time `json:"timestamp"`
}

type metricIngestPayload struct {
	InstanceID string   `json:"instance_id"`
	Metrics    []Metric `json:"metrics"`
}

type MySQLMetricTarget struct {
	InstanceID string
	Host       string
	Port       int
	User       string
	Password   string
}

func (c *MetricsCollector) throttled() bool {
	now := time.Now().UnixNano()
	last := c.lastCollectNS.Load()
	if now-last < int64(time.Second) {
		return false
	}
	return c.lastCollectNS.CompareAndSwap(last, now)
}

func (c *MetricsCollector) CollectMySQLMetrics(ctx context.Context, instanceID string) ([]Metric, error) {
	return nil, fmt.Errorf("mysql metrics require target connection details for %s", instanceID)
}

func (c *MetricsCollector) CollectSystemMetrics(ctx context.Context, host string) ([]Metric, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	now := time.Now()
	instanceID := strings.TrimSpace(host)
	if instanceID == "" {
		instanceID = "agent"
	}
	return []Metric{
		{InstanceID: instanceID, Name: "agent_goroutines", Value: float64(runtime.NumGoroutine()), Timestamp: now},
		{InstanceID: instanceID, Name: "agent_heap_alloc_mb", Value: float64(m.HeapAlloc) / 1024 / 1024, Timestamp: now},
		{InstanceID: instanceID, Name: "agent_heap_sys_mb", Value: float64(m.HeapSys) / 1024 / 1024, Timestamp: now},
		{InstanceID: instanceID, Name: "agent_num_cpu", Value: float64(runtime.NumCPU()), Timestamp: now},
		{InstanceID: instanceID, Name: "agent_num_gc", Value: float64(m.NumGC), Timestamp: now},
	}, nil
}

func (c *MetricsCollector) MetricsFromMySQLStatus(instanceID string, status map[string]string, timestamp time.Time) []Metric {
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	metrics := []Metric{
		{InstanceID: instanceID, Name: "qps", Value: statusFloat(status, "Queries"), Timestamp: timestamp},
		{InstanceID: instanceID, Name: "connections", Value: statusFloat(status, "Threads_connected"), Timestamp: timestamp},
		{InstanceID: instanceID, Name: "threads_connected", Value: statusFloat(status, "Threads_connected"), Timestamp: timestamp},
		{InstanceID: instanceID, Name: "threads_running", Value: statusFloat(status, "Threads_running"), Timestamp: timestamp},
		{InstanceID: instanceID, Name: "slow_queries", Value: statusFloat(status, "Slow_queries"), Timestamp: timestamp},
		{InstanceID: instanceID, Name: "uptime", Value: statusFloat(status, "Uptime"), Timestamp: timestamp},
		{InstanceID: instanceID, Name: "bytes_received", Value: statusFloat(status, "Bytes_received"), Timestamp: timestamp},
		{InstanceID: instanceID, Name: "bytes_sent", Value: statusFloat(status, "Bytes_sent"), Timestamp: timestamp},
		{InstanceID: instanceID, Name: "innodb_rows_inserted", Value: statusFloat(status, "Innodb_rows_inserted"), Timestamp: timestamp},
		{InstanceID: instanceID, Name: "innodb_rows_read", Value: statusFloat(status, "Innodb_rows_read"), Timestamp: timestamp},
		{InstanceID: instanceID, Name: "innodb_rows_updated", Value: statusFloat(status, "Innodb_rows_updated"), Timestamp: timestamp},
		{InstanceID: instanceID, Name: "innodb_rows_deleted", Value: statusFloat(status, "Innodb_rows_deleted"), Timestamp: timestamp},
	}
	tps := statusFloat(status, "Com_commit") + statusFloat(status, "Com_rollback")
	metrics = append(metrics, Metric{InstanceID: instanceID, Name: "tps", Value: tps, Timestamp: timestamp})
	bufferPoolReads := statusFloat(status, "Innodb_buffer_pool_reads")
	bufferPoolReadRequests := statusFloat(status, "Innodb_buffer_pool_read_requests")
	if bufferPoolReadRequests > 0 {
		hitRatio := (1 - bufferPoolReads/bufferPoolReadRequests) * 100
		if hitRatio < 0 {
			hitRatio = 0
		}
		metrics = append(metrics, Metric{InstanceID: instanceID, Name: "innodb_buffer_pool_hit_ratio", Value: hitRatio, Timestamp: timestamp})
	}
	return metrics
}

func (c *MetricsCollector) CollectMySQLStatusMetrics(ctx context.Context, target MySQLMetricTarget) ([]Metric, error) {
	target.InstanceID = strings.TrimSpace(target.InstanceID)
	if target.InstanceID == "" {
		return nil, fmt.Errorf("instance_id is required")
	}
	if target.Host == "" {
		target.Host = "127.0.0.1"
	}
	if target.Port == 0 {
		target.Port = 3306
	}
	if target.Port < 1 || target.Port > 65535 {
		return nil, fmt.Errorf("target_port must be between 1 and 65535")
	}
	if strings.TrimSpace(target.User) == "" {
		target.User = "root"
	}
	cmd := exec.CommandContext(ctx, "mysql",
		"-h", target.Host,
		"-P", strconv.Itoa(target.Port),
		"-u", target.User,
		"--batch",
		"--raw",
		"--skip-column-names",
		"-e", "SHOW GLOBAL STATUS",
	)
	if target.Password != "" {
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+target.Password)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("collect mysql status from %s:%d: %w, output: %s", target.Host, target.Port, err, strings.TrimSpace(string(out)))
	}
	status := parseMySQLStatusOutput(string(out))
	if len(status) == 0 {
		return nil, fmt.Errorf("mysql status output is empty")
	}
	return c.MetricsFromMySQLStatus(target.InstanceID, status, time.Now()), nil
}

func parseMySQLStatusOutput(output string) map[string]string {
	status := map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			status[fields[0]] = fields[1]
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key != "" {
			status[key] = value
		}
	}
	return status
}

func (c *MetricsCollector) ReportMetrics(ctx context.Context, platformURL, agentToken string, metrics []Metric) error {
	if strings.TrimSpace(platformURL) == "" || len(metrics) == 0 {
		return nil
	}
	if strings.TrimSpace(agentToken) == "" {
		return fmt.Errorf("agent token is required for metrics ingest")
	}
	instanceID := firstMetricInstanceID(metrics)
	if instanceID == "" {
		return fmt.Errorf("metric instance_id is required")
	}
	payload := metricIngestPayload{InstanceID: instanceID, Metrics: metrics}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal metrics payload: %w", err)
	}
	endpoint := strings.TrimRight(platformURL, "/") + "/internal/metrics/ingest"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build metrics ingest request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+agentToken)

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post metrics ingest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("metrics ingest returned status %d", resp.StatusCode)
	}
	return nil
}

func firstMetricInstanceID(metrics []Metric) string {
	for _, metric := range metrics {
		if strings.TrimSpace(metric.InstanceID) != "" {
			return strings.TrimSpace(metric.InstanceID)
		}
	}
	return ""
}

func statusFloat(status map[string]string, key string) float64 {
	if status == nil {
		return 0
	}
	value := strings.TrimSpace(status[key])
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}
