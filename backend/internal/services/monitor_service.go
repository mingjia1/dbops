package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/pkg/storage"
	"github.com/jackcode/mysql-ops-platform/pkg/utils"
)

type metricStore interface {
	WriteMetric(ctx context.Context, instanceID, metricName string, value float64, timestamp time.Time) error
	QueryMetrics(ctx context.Context, instanceID string, metricNames []string, startTime, endTime time.Time) ([]map[string]interface{}, error)
}

type metricHostRepository interface {
	GetByID(ctx context.Context, id string) (*models.Host, error)
}

type MonitorService struct {
	clickhouse metricStore
	instRepo   InstanceRepositoryInterface
	hostRepo   metricHostRepository
	agent      *AgentClient
	encKey     string
}

func NewMonitorService(clickhouse *storage.ClickHouse) *MonitorService {
	service := &MonitorService{}
	if clickhouse != nil {
		service.clickhouse = clickhouse
	}
	return service
}

func (s *MonitorService) SetCollectionDependencies(instRepo InstanceRepositoryInterface, hostRepo metricHostRepository, agent *AgentClient, encryptionKey string) {
	s.instRepo = instRepo
	s.hostRepo = hostRepo
	s.agent = agent
	s.encKey = encryptionKey
}

type MetricQueryRequest struct {
	InstanceID string    `json:"instance_id" binding:"required"`
	Metrics    []string  `json:"metrics" binding:"required"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
}

type MetricData struct {
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

type MetricIngestRequest struct {
	InstanceID string       `json:"instance_id" binding:"required"`
	Metrics    []MetricData `json:"metrics" binding:"required"`
}

func (s *MonitorService) QueryMetrics(ctx context.Context, req MetricQueryRequest) ([]MetricData, error) {
	if s.clickhouse == nil {
		return []MetricData{}, nil
	}
	if req.EndTime.IsZero() {
		req.EndTime = time.Now()
	}
	if req.StartTime.IsZero() {
		req.StartTime = req.EndTime.Add(-1 * time.Hour)
	}
	if len(req.Metrics) == 0 {
		req.Metrics = defaultMonitorMetrics()
	}

	results, err := s.clickhouse.QueryMetrics(ctx, req.InstanceID, req.Metrics, req.StartTime, req.EndTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query clickhouse: %w", err)
	}

	metrics := make([]MetricData, 0, len(results))
	for _, row := range results {
		name, _ := row["name"].(string)
		value, _ := row["value"].(float64)
		ts, _ := row["timestamp"].(time.Time)
		metrics = append(metrics, MetricData{Name: name, Value: value, Timestamp: ts})
	}
	if len(metrics) == 0 && strings.TrimSpace(req.InstanceID) != "" && s.canCollectInstanceMetrics() {
		collected, err := s.CollectInstanceMetrics(ctx, req.InstanceID)
		if err != nil {
			return nil, err
		}
		if len(collected) > 0 {
			return collected, nil
		}
	}
	return metrics, nil
}

func (s *MonitorService) IngestMetrics(ctx context.Context, req MetricIngestRequest) error {
	if s.clickhouse == nil {
		return fmt.Errorf("clickhouse not configured, metric ingest skipped for %s", req.InstanceID)
	}
	req.InstanceID = strings.TrimSpace(req.InstanceID)
	if req.InstanceID == "" {
		return fmt.Errorf("instance_id is required")
	}
	if len(req.Metrics) == 0 {
		return fmt.Errorf("metrics is required")
	}
	for _, metric := range req.Metrics {
		name := strings.TrimSpace(metric.Name)
		if name == "" {
			return fmt.Errorf("metric name is required")
		}
		ts := metric.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		if err := s.clickhouse.WriteMetric(ctx, req.InstanceID, name, metric.Value, ts); err != nil {
			return err
		}
	}
	return nil
}

func (s *MonitorService) CollectMetrics(ctx context.Context, instanceID string) error {
	if s.clickhouse == nil {
		return fmt.Errorf("clickhouse not configured, metric collection skipped for %s", instanceID)
	}
	if err := s.clickhouse.WriteMetric(ctx, instanceID, "agent_heartbeat", 1.0, time.Now()); err != nil {
		return fmt.Errorf("failed to write heartbeat to clickhouse: %w", err)
	}
	return nil
}

func (s *MonitorService) CollectInstanceMetrics(ctx context.Context, instanceID string) ([]MetricData, error) {
	if s.clickhouse == nil {
		return nil, fmt.Errorf("clickhouse not configured, metric collection skipped for %s", instanceID)
	}
	if !s.canCollectInstanceMetrics() {
		return nil, fmt.Errorf("metric collection dependencies not configured")
	}
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return nil, fmt.Errorf("instance_id is required")
	}
	inst, err := s.instRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	conn, err := s.instRepo.GetConnection(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(conn.Host) == "" {
		return nil, fmt.Errorf("instance %s connection host is required", instanceID)
	}
	agentHost := conn.Host
	agentPort := 9090
	if inst.HostID != nil && strings.TrimSpace(*inst.HostID) != "" && s.hostRepo != nil {
		host, err := s.hostRepo.GetByID(ctx, *inst.HostID)
		if err != nil {
			return nil, err
		}
		if host.Address != "" {
			agentHost = host.Address
		}
		if host.AgentPort != 0 {
			agentPort = host.AgentPort
		}
	}
	password := ""
	if conn.PasswordEncrypted != "" {
		if strings.TrimSpace(s.encKey) == "" {
			return nil, fmt.Errorf("encryption key not configured for metric collection")
		}
		password, err = utils.Decrypt(conn.PasswordEncrypted, s.encKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt instance password: %w", err)
		}
	}
	user := conn.Username
	if user == "" {
		user = "root"
	}
	metrics, err := s.agent.ExecuteMetricsCollect(ctx, agentHost, agentPort, instanceID, map[string]interface{}{
		"target_host": conn.Host,
		"target_port": conn.Port,
		"target_user": user,
		"target_pass": password,
	})
	if err != nil {
		return nil, err
	}
	if err := s.IngestMetrics(ctx, MetricIngestRequest{InstanceID: instanceID, Metrics: metrics}); err != nil {
		return nil, err
	}
	return metrics, nil
}

func (s *MonitorService) canCollectInstanceMetrics() bool {
	return s.instRepo != nil && s.agent != nil
}

func (s *MonitorService) CollectionStatus() string {
	if s.clickhouse == nil {
		return "not_configured"
	}
	return "configured"
}

func defaultMonitorMetrics() []string {
	return []string{
		"qps", "tps", "connections", "threads_connected", "threads_running",
		"cpu", "cpu_usage", "memory", "memory_usage", "disk", "disk_usage",
		"slow_queries", "uptime", "bytes_received", "bytes_sent",
		"innodb_buffer_pool_hit_ratio", "innodb_rows_inserted", "innodb_rows_read",
		"innodb_rows_updated", "innodb_rows_deleted",
		"agent_goroutines", "agent_heap_alloc_mb", "agent_heap_sys_mb", "agent_num_cpu", "agent_num_gc",
	}
}
