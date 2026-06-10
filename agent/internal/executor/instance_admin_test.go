package executor

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteInstanceAdminServiceControlValidatesMetadata(t *testing.T) {
	executor := NewTaskExecutor()

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantMsg string
	}{
		{
			name: "status requires datadir",
			config: map[string]interface{}{
				"action": "service_control",
				"verb":   "status",
			},
			wantMsg: "service control requires instance datadir metadata",
		},
		{
			name: "start requires port",
			config: map[string]interface{}{
				"action":  "service_control",
				"verb":    "start",
				"datadir": "/data/mysql/3307",
			},
			wantMsg: "service control start requires instance port metadata",
		},
		{
			name: "restart requires port",
			config: map[string]interface{}{
				"action":  "service_control",
				"verb":    "restart",
				"datadir": "/data/mysql/3307",
			},
			wantMsg: "service control restart requires instance port metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.ExecuteInstanceAdmin(context.Background(), DeployTaskRequest{
				TaskID: "admin-test",
				Config: tt.config,
			})

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "failed", result.Status)
			assert.Equal(t, 100, result.Progress)
			assert.Contains(t, result.Message, tt.wantMsg)
		})
	}
}

func TestExecuteInstanceAdminServiceControlDefaultsTrimmedStatus(t *testing.T) {
	executor := NewTaskExecutor()
	datadir := t.TempDir()

	result, err := executor.ExecuteInstanceAdmin(context.Background(), DeployTaskRequest{
		TaskID: "admin-test",
		Config: map[string]interface{}{
			"action":      "service_control",
			"verb":        " STATUS ",
			"datadir":     datadir,
			"target_port": 3307,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "completed", result.Status)
	assert.Contains(t, strings.ToLower(result.Message), "stopped")
	assert.Contains(t, result.Message, datadir)
}
