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

// InstanceCredentialError 表示实例凭据无法解密 (常见原因: 平台加密密钥已更换,
// 旧密文无法用新密钥解密)。与 agent 不可达区分, 供控制器给出针对性的降级提示。
type InstanceCredentialError struct {
	Cause error
}

func (e *InstanceCredentialError) Error() string {
	return "instance credential error: " + e.Cause.Error()
}

func (e *InstanceCredentialError) Unwrap() error {
	return e.Cause
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
		// standalone 模式: 未配置 ClickHouse 时不再直接返回空结果, 改为经 agent
		// 实时拉取实例指标, 避免监控面板永久停留在 "not configured"。
		if strings.TrimSpace(req.InstanceID) == "" || !s.canCollectInstanceMetrics() {
			return []MetricData{}, nil
		}
		return s.collectInstanceMetricsLive(ctx, req.InstanceID)
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
		// standalone 模式: 无 ClickHouse 历史库, agent 周期推送的指标无处落库,
		// 静默丢弃而非报错, 避免内部 ingest 接口持续返回 503。
		return nil
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
	metrics, err := s.collectInstanceMetricsLive(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if err := s.IngestMetrics(ctx, MetricIngestRequest{InstanceID: instanceID, Metrics: metrics}); err != nil {
		return nil, err
	}
	return metrics, nil
}

// collectInstanceMetricsLive 经 agent 实时采集实例指标, 不依赖 ClickHouse 落库。
// 既服务于 ClickHouse 写入链路 (CollectInstanceMetrics), 也作为 standalone 模式下
// 监控面板的直接数据源。
func (s *MonitorService) collectInstanceMetricsLive(ctx context.Context, instanceID string) ([]MetricData, error) {
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
	if !isMySQLProtocolFlavor(inst.Version.Flavor) {
		return nil, fmt.Errorf("metrics collection supports MySQL-protocol engines only; instance flavor is %q", normalizeFlavor(inst.Version.Flavor))
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
			return nil, &InstanceCredentialError{Cause: fmt.Errorf("encryption key not configured for metric collection")}
		}
		password, err = utils.Decrypt(conn.PasswordEncrypted, s.encKey)
		if err != nil {
			return nil, &InstanceCredentialError{Cause: fmt.Errorf("decrypt instance password: %w", err)}
		}
	}
	user := conn.Username
	if user == "" {
		user = "root"
	}
	metrics, err := s.agent.ExecuteMetricsCollectFast(ctx, agentHost, agentPort, instanceID, map[string]interface{}{
		"target_host":        conn.Host,
		"target_port":        conn.Port,
		"target_user":        user,
		"target_pass":        password,
		"target_ssl_enabled": conn.SSLEnabled,
	})
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func (s *MonitorService) canCollectInstanceMetrics() bool {
	return s.instRepo != nil && s.agent != nil
}

func (s *MonitorService) CollectionStatus() string {
	if s.clickhouse != nil {
		return "configured"
	}
	if s.canCollectInstanceMetrics() {
		// standalone: 无 ClickHouse 历史库, 但可经 agent 实时采集
		return "standalone"
	}
	return "not_configured"
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
