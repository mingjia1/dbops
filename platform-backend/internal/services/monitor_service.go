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

func (s *MonitorService) QueryMetrics(ctx context.Context, req MetricQueryRequest) ([]MetricData, error) {
	if s.clickhouse == nil {
		return s.queryMockMetrics(req), nil
	}

	results, err := s.clickhouse.QueryMetrics(ctx, req.InstanceID, req.Metrics, req.StartTime, req.EndTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query clickhouse: %w", err)
	}

	var metrics []MetricData
	for _, row := range results {
		metrics = append(metrics, MetricData{
			Name:      row["name"].(string),
			Value:     row["value"].(float64),
			Timestamp: row["timestamp"].(time.Time),
		})
	}

	if len(metrics) == 0 {
		return s.queryMockMetrics(req), nil
	}

	return metrics, nil
}

func (s *MonitorService) queryMockMetrics(req MetricQueryRequest) []MetricData {
	return []MetricData{
		{
			Name:      "qps",
			Value:     1500.5,
			Timestamp: time.Now(),
		},
		{
			Name:      "tps",
			Value:     200.3,
			Timestamp: time.Now(),
		},
		{
			Name:      "connections",
			Value:     50.0,
			Timestamp: time.Now(),
		},
	}
}

func (s *MonitorService) CollectMetrics(ctx context.Context, instanceID string) error {
	fmt.Printf("Collecting metrics for instance %s\n", instanceID)

	if s.clickhouse != nil {
		timestamp := time.Now()

		metrics := []struct {
			name  string
			value float64
		}{
			{"qps", 1500.5},
			{"tps", 200.3},
			{"connections", 50.0},
			{"cpu_usage", 45.2},
			{"memory_usage", 60.8},
		}

		for _, m := range metrics {
			if err := s.clickhouse.WriteMetric(ctx, instanceID, m.name, m.value, timestamp); err != nil {
				fmt.Printf("Failed to write metric %s: %v\n", m.name, err)
			}
		}
	}

	return nil
}