package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReportMetricsPostsToBackendIngest(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotPayload metricIngestPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	collector := NewMetricsCollector()
	err := collector.ReportMetrics(context.Background(), server.URL, "agent-token", []Metric{
		{InstanceID: "instance-1", Name: "qps", Value: 12.5, Timestamp: time.Now()},
	})

	if err != nil {
		t.Fatalf("ReportMetrics returned error: %v", err)
	}
	if gotPath != "/internal/metrics/ingest" {
		t.Fatalf("path = %q, want /internal/metrics/ingest", gotPath)
	}
	if gotAuth != "Bearer agent-token" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if gotPayload.InstanceID != "instance-1" || len(gotPayload.Metrics) != 1 || gotPayload.Metrics[0].Name != "qps" {
		t.Fatalf("unexpected payload: %+v", gotPayload)
	}
}

func TestMetricsFromMySQLStatusBuildsStandardMetrics(t *testing.T) {
	collector := NewMetricsCollector()
	now := time.Now()

	metrics := collector.MetricsFromMySQLStatus("mysql-1", map[string]string{
		"Queries":                          "1000",
		"Com_commit":                       "80",
		"Com_rollback":                     "5",
		"Threads_connected":                "12",
		"Threads_running":                  "3",
		"Slow_queries":                     "2",
		"Innodb_buffer_pool_reads":         "10",
		"Innodb_buffer_pool_read_requests": "1000",
	}, now)

	values := map[string]float64{}
	for _, metric := range metrics {
		if metric.InstanceID != "mysql-1" {
			t.Fatalf("InstanceID = %q", metric.InstanceID)
		}
		values[metric.Name] = metric.Value
	}
	if values["qps"] != 1000 {
		t.Fatalf("qps = %v", values["qps"])
	}
	if values["tps"] != 85 {
		t.Fatalf("tps = %v", values["tps"])
	}
	if values["threads_connected"] != 12 || values["threads_running"] != 3 {
		t.Fatalf("thread metrics = %+v", values)
	}
	if values["innodb_buffer_pool_hit_ratio"] != 99 {
		t.Fatalf("buffer pool hit ratio = %v", values["innodb_buffer_pool_hit_ratio"])
	}
}

func TestReportMetricsRequiresTokenAndInstanceID(t *testing.T) {
	collector := NewMetricsCollector()
	if err := collector.ReportMetrics(context.Background(), "http://127.0.0.1", "", []Metric{{InstanceID: "i", Name: "qps"}}); err == nil {
		t.Fatal("expected missing token error")
	}
	if err := collector.ReportMetrics(context.Background(), "http://127.0.0.1", "token", []Metric{{Name: "qps"}}); err == nil {
		t.Fatal("expected missing instance_id error")
	}
}

func TestReportMetricsReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	collector := NewMetricsCollector()
	if err := collector.ReportMetrics(context.Background(), server.URL, "bad-token", []Metric{{InstanceID: "i", Name: "qps"}}); err == nil {
		t.Fatal("expected non-2xx error")
	}
}
