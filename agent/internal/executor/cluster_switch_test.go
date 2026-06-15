package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteClusterSwitchValidatesRequest(t *testing.T) {
	executor := NewTaskExecutor()

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantMsg string
	}{
		{
			name:    "missing switch type",
			config:  map[string]interface{}{"cluster_type": "mgr", "nodes": []interface{}{map[string]interface{}{"instance_id": "one"}, map[string]interface{}{"instance_id": "two"}}},
			wantMsg: "switch_type is required",
		},
		{
			name:    "missing nodes",
			config:  map[string]interface{}{"switch_type": "single-to-mgr", "cluster_type": "mgr"},
			wantMsg: "cluster switch requires at least two nodes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.ExecuteClusterSwitch(context.Background(), DeployTaskRequest{
				TaskID: "cluster-switch-test",
				Config: tt.config,
			})

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, "failed", result.Status)
			assert.Contains(t, result.Message, tt.wantMsg)
		})
	}
}

func TestExecuteClusterSwitchAcceptsMultiNodeRequest(t *testing.T) {
	executor := NewTaskExecutor()

	result, err := executor.ExecuteClusterSwitch(context.Background(), DeployTaskRequest{
		TaskID: "cluster-switch-test",
		Config: map[string]interface{}{
			"switch_type":  "single-to-mgr",
			"cluster_id":   "mgr-reconfig",
			"cluster_type": "mgr",
			"nodes": []interface{}{
				map[string]interface{}{"instance_id": "one", "role": "primary"},
				map[string]interface{}{"instance_id": "two", "role": "secondary"},
				map[string]interface{}{"instance_id": "three", "role": "secondary"},
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "completed", result.Status)
	assert.Contains(t, result.Message, "mgr cluster switch request accepted for 3 nodes")
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "mgr", data["cluster_type"])
}
