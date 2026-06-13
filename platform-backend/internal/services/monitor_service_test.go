package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMonitorService(t *testing.T) {
	service := NewMonitorService(nil)
	assert.NotNil(t, service)
}

func TestQueryMetrics(t *testing.T) {
	service := NewMonitorService(nil)

	req := MetricQueryRequest{
		InstanceID: "instance-001",
		Metrics:    []string{"qps", "tps", "connections"},
	}

	ctx := context.Background()
	metrics, err := service.QueryMetrics(ctx, req)

	// P0-4: 无 ClickHouse 时返回空 slice, 不再返回写死假数据.
	assert.NoError(t, err)
	assert.Empty(t, metrics)
}

func TestQueryMetrics_WithTimeRange(t *testing.T) {
	service := NewMonitorService(nil)

	req := MetricQueryRequest{
		InstanceID: "instance-001",
		Metrics:    []string{"qps"},
		StartTime:  time.Now().Add(-1 * time.Hour),
		EndTime:    time.Now(),
	}

	ctx := context.Background()
	metrics, err := service.QueryMetrics(ctx, req)

	// P0-4: 无 ClickHouse 时返回空.
	assert.NoError(t, err)
	assert.Empty(t, metrics)
}

func TestCollectMetrics(t *testing.T) {
	service := NewMonitorService(nil)

	ctx := context.Background()
	err := service.CollectMetrics(ctx, "instance-001")

	// 无 ClickHouse 时, 显式报错而不是默默吞掉 — 运维需要感知.
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "clickhouse not configured")
}

func TestMetricQueryRequest_Fields(t *testing.T) {
	req := MetricQueryRequest{
		InstanceID: "instance-001",
		Metrics:    []string{"qps", "tps"},
		StartTime:  time.Now(),
		EndTime:    time.Now(),
	}

	assert.Equal(t, "instance-001", req.InstanceID)
	assert.Len(t, req.Metrics, 2)
	assert.Contains(t, req.Metrics, "qps")
	assert.Contains(t, req.Metrics, "tps")
	assert.NotZero(t, req.StartTime)
	assert.NotZero(t, req.EndTime)
}

func TestMetricData_Fields(t *testing.T) {
	metric := MetricData{
		Name:      "qps",
		Value:     1500.5,
		Timestamp: time.Now(),
	}

	assert.Equal(t, "qps", metric.Name)
	assert.Equal(t, 1500.5, metric.Value)
	assert.NotZero(t, metric.Timestamp)
}

func TestQueryMetrics_SpecificValues(t *testing.T) {
	service := NewMonitorService(nil)
	ctx := context.Background()

	metrics, err := service.QueryMetrics(ctx, MetricQueryRequest{})

	// P0-4: 无 ClickHouse 时不再返回写死的 1500.5/200.3/50.0 假数据.
	assert.NoError(t, err)
	assert.Empty(t, metrics)
}

func findMetric(metrics []MetricData, name string) *MetricData {
	for _, m := range metrics {
		if m.Name == name {
			return &m
		}
	}
	return nil
}

func TestCollectMetrics_MultipleInstances(t *testing.T) {
	service := NewMonitorService(nil)
	ctx := context.Background()

	err := service.CollectMetrics(ctx, "instance-001")
	assert.Error(t, err)

	err = service.CollectMetrics(ctx, "instance-002")
	assert.Error(t, err)

	err = service.CollectMetrics(ctx, "instance-003")
	assert.Error(t, err)
}

func TestQueryMetrics_EmptyMetrics(t *testing.T) {
	service := NewMonitorService(nil)

	req := MetricQueryRequest{
		InstanceID: "instance-001",
		Metrics:    []string{},
	}

	ctx := context.Background()
	metrics, err := service.QueryMetrics(ctx, req)

	assert.NoError(t, err)
	assert.Empty(t, metrics)
}

func TestMetricData_Timestamp(t *testing.T) {
	service := NewMonitorService(nil)
	ctx := context.Background()

	metrics, err := service.QueryMetrics(ctx, MetricQueryRequest{})

	// P0-4: 无 ClickHouse 时空 list.
	assert.NoError(t, err)
	assert.Empty(t, metrics)
}
