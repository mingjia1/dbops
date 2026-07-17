package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentTaskPathNonIdempotent(t *testing.T) {
	if !agentTaskPathNonIdempotent("/agent/tasks/backup") {
		t.Fatal("backup should be non-idempotent")
	}
	if !agentTaskPathNonIdempotent("/agent/tasks/restore") {
		t.Fatal("restore should be non-idempotent")
	}
	if agentTaskPathNonIdempotent("/agent/tasks/health-check") {
		t.Fatal("health-check should remain retriable")
	}
}

func TestAgentClientCallAgentRejectsEmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/deploy", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data":    nil,
		})
	}))
	defer server.Close()
	host, port := mustParseTestServer(t, server.URL)

	result, err := NewAgentClient("").callAgent(context.Background(), host, port, "/agent/tasks/deploy", map[string]interface{}{})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "empty data")
}

func TestAgentClientCallAgentGetRejectsBusinessError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/health-check", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    500,
			"message": "agent task failed",
			"data":    nil,
		})
	}))
	defer server.Close()
	host, port := mustParseTestServer(t, server.URL)

	result, err := NewAgentClient("").ExecuteHealthCheck(context.Background(), host, port, "instance-001")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "agent error")
}

func TestAgentClientGetTaskProgressRejectsEmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agent/tasks/task-001/progress", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    200,
			"message": "success",
			"data":    nil,
		})
	}))
	defer server.Close()
	host, port := mustParseTestServer(t, server.URL)

	result, err := NewAgentClient("").GetTaskProgress(context.Background(), host, port, "task-001")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "empty data")
}

func mustParseTestServer(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	return u.Hostname(), port
}
