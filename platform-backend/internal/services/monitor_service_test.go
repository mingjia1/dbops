package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMonitorService(t *testing.T) {
	service := NewMonitorService()
	assert.NotNil(t, service)
}

func TestQueryMetrics(t *testing.T) {
	service := NewMonitorService()
	
	req := MetricQueryRequest{
		InstanceID: "instance-001",
		Metrics:    []string{"qps", "tps", "connections"},
	}
	
	ctx := context.Background()
	metrics, err := service.QueryMetrics(ctx, req)
	
	assert.NoError(t, err)
	assert.NotEmpty(t, metrics)
	assert.Len(t, metrics, 3)
	
	for _, m := range metrics {
		assert.NotEmpty(t, m.Name)
		assert.GreaterOrEqual(t, m.Value, float64(0))
		assert.NotZero(t, m.Timestamp)
	}
}

func TestQueryMetrics_WithTimeRange(t *testing.T) {
	service := NewMonitorService()
	
	req := MetricQueryRequest{
		InstanceID: "instance-001",
		Metrics:    []string{"qps"},
		StartTime:  time.Now().Add(-1 * time.Hour),
		EndTime:    time.Now(),
	}
	
	ctx := context.Background()
	metrics, err := service.QueryMetrics(ctx, req)
	
	assert.NoError(t, err)
	assert.NotEmpty(t, metrics)
}

func TestCollectMetrics(t *testing.T) {
	service := NewMonitorService()
	
	ctx := context.Background()
	err := service.CollectMetrics(ctx, "instance-001")
	
	assert.NoError(t, err)
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
	service := NewMonitorService()
	ctx := context.Background()
	
	metrics, err := service.QueryMetrics(ctx, MetricQueryRequest{})
	
	assert.NoError(t, err)
	
	qpsMetric := findMetric(metrics, "qps")
	assert.NotNil(t, qpsMetric)
	assert.Equal(t, 1500.5, qpsMetric.Value)
	
	tpsMetric := findMetric(metrics, "tps")
	assert.NotNil(t, tpsMetric)
	assert.Equal(t, 200.3, tpsMetric.Value)
	
	connMetric := findMetric(metrics, "connections")
	assert.NotNil(t, connMetric)
	assert.Equal(t, 50.0, connMetric.Value)
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
	service := NewMonitorService()
	ctx := context.Background()
	
	err := service.CollectMetrics(ctx, "instance-001")
	assert.NoError(t, err)
	
	err = service.CollectMetrics(ctx, "instance-002")
	assert.NoError(t, err)
	
	err = service.CollectMetrics(ctx, "instance-003")
	assert.NoError(t, err)
}

func TestQueryMetrics_EmptyMetrics(t *testing.T) {
	service := NewMonitorService()
	
	req := MetricQueryRequest{
		InstanceID: "instance-001",
		Metrics:    []string{},
	}
	
	ctx := context.Background()
	metrics, err := service.QueryMetrics(ctx, req)
	
	assert.NoError(t, err)
	assert.Len(t, metrics, 3)
}

func TestMetricData_Timestamp(t *testing.T) {
	service := NewMonitorService()
	ctx := context.Background()
	
	metrics, err := service.QueryMetrics(ctx, MetricQueryRequest{})
	
	assert.NoError(t, err)
	for _, m := range metrics {
		assert.True(t, m.Timestamp.Before(time.Now().Add(1*time.Second)))
		assert.True(t, m.Timestamp.After(time.Now().Add(-1*time.Minute)))
	}
}