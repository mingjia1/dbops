package services

import (
	"context"
	"fmt"
	"time"

	"github.com/monkeycode/mysql-ops-platform/pkg/storage"
)

type MonitorService struct {
	clickhouse *storage.ClickHouse
}

func NewMonitorService(clickhouse *storage.ClickHouse) *MonitorService {
	return &MonitorService{
		clickhouse: clickhouse,
	}
}

type MetricQueryRequest struct {
	InstanceID string   `json:"instance_id" binding:"required"`
	Metrics    []string `json:"metrics" binding:"required"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
}

type MetricData struct {
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// QueryMetrics P0-4: 不再返回写死的假数据. ClickHouse 不可用时返回空集 + warning, 调用方按需处理.
func (s *MonitorService) QueryMetrics(ctx context.Context, req MetricQueryRequest) ([]MetricData, error) {
	if s.clickhouse == nil {
		// 真实返回空, 业务层用日志/告警感知.
		return []MetricData{}, nil
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
	return metrics, nil
}

func (s *MonitorService) CollectMetrics(ctx context.Context, instanceID string) error {
	if s.clickhouse == nil {
		return fmt.Errorf("clickhouse not configured, metric collection skipped for %s", instanceID)
	}
	// P1-1: 之前 clickhouse != nil 时直接 return nil, 静默"成功"但实际啥都没做.
	// 修: 写一行 heartbeat 探针到 clickhouse, 失败立刻报错, 运维能看到 clickhouse 通道是否正常.
	// 真实指标值由 Agent 推, 这里只做探活.
	if err := s.clickhouse.WriteMetric(ctx, instanceID, "agent_heartbeat", 1.0, time.Now()); err != nil {
		return fmt.Errorf("failed to write heartbeat to clickhouse: %w", err)
	}
	return nil
}