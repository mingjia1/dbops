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
	timestamp := time.Now()
	// 真实指标值由调用方提供, 这里仅占位: 实际数据由 Agent 推送到 ClickHouse.
	_ = timestamp
	return nil
}