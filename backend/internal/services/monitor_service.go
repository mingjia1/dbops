package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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
	// sampleDB: standalone 模式下指标历史样本的 SQLite 落库 (可 nil).
	// 解决"监控面板只有一个点": 采集/推送的样本落库, 查询按时间范围返回序列.
	sampleDB *sql.DB
}

// SetSampleDB: 注入 standalone 指标历史存储 (main.go 传 *repositories.Database 的 Pool).
func (s *MonitorService) SetSampleDB(db *sql.DB) {
	s.sampleDB = db
}

// saveSamples: 落一批样本; 顺带清理 8 天前旧数据 (低频写入下代价可忽略).
func (s *MonitorService) saveSamples(ctx context.Context, instanceID string, metrics []MetricData) {
	if s.sampleDB == nil || instanceID == "" || len(metrics) == 0 {
		return
	}
	for _, m := range metrics {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			continue
		}
		ts := m.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		_, _ = s.sampleDB.ExecContext(ctx,
			`INSERT INTO metric_samples (id, instance_id, name, value, ts) VALUES (?, ?, ?, ?, ?)`,
			uuid.New().String(), instanceID, name, m.Value, sampleTime(ts))
	}
	cutoff := time.Now().AddDate(0, 0, -8)
	_, _ = s.sampleDB.ExecContext(ctx, `DELETE FROM metric_samples WHERE ts < ?`, sampleTime(cutoff))
}

// querySamples: 查窗口内历史序列 (按指标名过滤, names 为空则全部).
func (s *MonitorService) querySamples(ctx context.Context, instanceID string, names []string, start, end time.Time) ([]MetricData, error) {
	if s.sampleDB == nil {
		return nil, nil
	}
	query := `SELECT name, value, ts FROM metric_samples WHERE instance_id = ? AND ts BETWEEN ? AND ?`
	// 时间边界以字符串传入并与存储格式一致, 避开 driver 绑定 time.Time 时的格式差异.
	args := []interface{}{instanceID, sampleTime(start), sampleTime(end)}
	if len(names) > 0 {
		placeholders := make([]string, 0, len(names))
		for _, n := range names {
			placeholders = append(placeholders, "?")
			args = append(args, n)
		}
		query += " AND name IN (" + strings.Join(placeholders, ",") + ")"
	}
	query += " ORDER BY ts ASC"
	rows, err := s.sampleDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MetricData, 0, 64)
	for rows.Next() {
		var name, tsStr string
		var value float64
		if err := rows.Scan(&name, &value, &tsStr); err != nil {
			return nil, err
		}
		ts, perr := parseSampleTime(tsStr)
		if perr != nil {
			ts = time.Now().UTC()
		}
		out = append(out, MetricData{Name: name, Value: value, Timestamp: ts})
	}
	return out, rows.Err()
}

// sampleTimeLayout: metric_samples.ts 的统一存储格式 (UTC).
const sampleTimeLayout = "2006-01-02 15:04:05.000000000"

// sampleTimeFormats: metric_samples.ts 的合法读取格式, 按优先级排列.
// 陷阱: ts 列声明为 TIMESTAMP 时, modernc.org/sqlite 会在读取层把存储的
// 文本解析为 time.Time, database/sql 再将其 Scan 进 *string 时格式化为
// RFC3339Nano ("2006-01-02T15:04:05.999999999Z07:00") —— 与存储格式不同,
// 只按存储格式解析会全部失败并误落到兜底时间. 因此三种格式都要兼容.
var sampleTimeFormats = []string{
	sampleTimeLayout,                            // 直存文本 (Scan 进 string 不经 driver 转换时)
	time.RFC3339Nano,                            // driver 读取层 TIMESTAMP→time.Time→string
	"2006-01-02 15:04:05.999999999 -0700 MST",   // 历史 Go 默认格式脏数据
}

func parseSampleTime(s string) (time.Time, error) {
	for _, layout := range sampleTimeFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized sample time format: %q", s)
}

func sampleTime(t time.Time) string {
	return t.UTC().Format(sampleTimeLayout)
}

// StartStandaloneCollector: standalone 模式下周期采集全部 MySQL 实例指标并落库,
// 形成真实历史序列 (否则只有前端查询触发的单点). 每 30s 一轮; ctx 取消即停止.
func (s *MonitorService) StartStandaloneCollector(ctx context.Context, interval time.Duration) {
	if s.clickhouse != nil || s.sampleDB == nil || !s.canCollectInstanceMetrics() {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				instances, err := s.instRepo.List(ctx, 1000, 0)
				if err != nil {
					continue
				}
				for _, inst := range instances {
					if !isMySQLProtocolFlavor(inst.Version.Flavor) {
						continue
					}
					metrics, err := s.collectInstanceMetricsLive(ctx, inst.ID)
					if err != nil {
						continue
					}
					s.saveSamples(ctx, inst.ID, metrics)
				}
			}
		}
	}()
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
		// standalone 模式: 历史样本落 SQLite, 查询返回窗口内完整序列;
		// 再实时采一点补尾部, 保证最新值可见 (采到也落库供后续查询).
		if strings.TrimSpace(req.InstanceID) == "" || !s.canCollectInstanceMetrics() {
			return []MetricData{}, nil
		}
		if req.EndTime.IsZero() {
			req.EndTime = time.Now()
		}
		if req.StartTime.IsZero() {
			req.StartTime = req.EndTime.Add(-1 * time.Hour)
		}
		history, qerr := s.querySamples(ctx, req.InstanceID, req.Metrics, req.StartTime, req.EndTime)
		if qerr != nil {
			return nil, fmt.Errorf("failed to query metric samples: %w", qerr)
		}
		live, lerr := s.collectInstanceMetricsLive(ctx, req.InstanceID)
		if lerr == nil && len(live) > 0 {
			s.saveSamples(ctx, req.InstanceID, live)
			history = append(history, live...)
		}
		return history, nil
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
		// standalone 模式: agent 周期推送的指标落 SQLite 历史库 (原为静默丢弃,
		// 导致监控面板永远只有实时单点). 落库失败不报错, 避免内部 ingest 返回 503。
		s.saveSamples(ctx, req.InstanceID, req.Metrics)
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
