package grpc

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStreamer(t *testing.T) {
	s := NewStreamer()
	require.NotNil(t, s)
}

func TestParseStage(t *testing.T) {
	assert.Equal(t, "2/5", parseStage("[2/5] Deploying replicas..."))
	assert.Equal(t, "running", parseStage("some log line without brackets"))
	assert.Equal(t, "1/1", parseStage("[1/1] Done"))
}

func TestParseProgress(t *testing.T) {
	assert.Equal(t, 40, parseProgress("[2/5] step", 0))
	assert.Equal(t, 100, parseProgress("[5/5] final", 0))
	assert.Equal(t, 5, parseProgress("1/200 init", 1))
	assert.Equal(t, 10, parseProgress("no brackets", 0))
}

func TestStreamer_ExecuteAndStream_NilFn(t *testing.T) {
	s := NewStreamer()
	err := s.ExecuteAndStream(context.Background(), "task-001", "echo", []string{"hello"}, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "progress function")
}

func TestStreamer_ExecuteAndStream_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip: echo not available on Windows")
	}
	s := NewStreamer()
	var events []StreamEvent
	err := s.ExecuteAndStream(context.Background(), "task-001", "echo", []string{"hello"}, func(e StreamEvent) {
		events = append(events, e)
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(events), 2)
	assert.Equal(t, "starting", events[0].Stage)
	assert.Equal(t, "completed", events[len(events)-1].Status)
}

func TestStreamer_ExecuteAndStream_Failure(t *testing.T) {
	s := NewStreamer()
	var events []StreamEvent
	err := s.ExecuteAndStream(context.Background(), "task-001", "false", nil, func(e StreamEvent) {
		events = append(events, e)
	})
	assert.Error(t, err)
	found := false
	for _, e := range events {
		if e.Status == "failed" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestNewAgentGRPCServer(t *testing.T) {
	srv := NewAgentGRPCServer(nil)
	assert.NotNil(t, srv)
	assert.Equal(t, 0, srv.ActiveTaskCount())
}

func TestAgentGRPCServer_HealthCheck(t *testing.T) {
	srv := NewAgentGRPCServer(nil)
	status, version := srv.HealthCheck()
	assert.Equal(t, "healthy", status)
	assert.Equal(t, "1.0.0", version)
}

func TestAgentGRPCServer_CancelNonexistentTask(t *testing.T) {
	srv := NewAgentGRPCServer(nil)
	ok := srv.CancelTask("nonexistent")
	assert.False(t, ok)
}
