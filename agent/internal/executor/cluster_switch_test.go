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

func TestBuildClusterSwitchDeployRequestForMGR(t *testing.T) {
	req := DeployTaskRequest{
		TaskID:     "cluster-switch-test",
		InstanceID: "two",
		Config: map[string]interface{}{
			"switch_type":       "single-to-mgr",
			"cluster_id":        "mgr-reconfig",
			"cluster_type":      "mgr",
			"replication_user":  "repl",
			"replication_pass":  "repl@1234556",
			"mysql_user":        "root",
			"mysql_password":    "VFR$3edcXSW@1qaz",
			"force_reconfigure": true,
		},
	}
	nodes := []map[string]any{
		{"instance_id": "one", "host": "10.1.81.16", "port": 3306, "role": "primary"},
		{"instance_id": "two", "host": "10.1.81.17", "port": 3306, "role": "secondary"},
		{"instance_id": "three", "host": "10.1.81.18", "port": 3306, "role": "secondary"},
	}

	adapted, err := buildClusterSwitchDeployRequest(req, "mgr", nodes)

	require.NoError(t, err)
	assert.Equal(t, "cluster-switch-test", adapted.TaskID)
	assert.Equal(t, "single-primary", adapted.Config["deploy_mode"])
	assert.Equal(t, false, adapted.Config["bootstrap"])
	assert.Equal(t, "10.1.81.17", adapted.Config["local_address"])
	assert.Equal(t, 3306, adapted.Config["mysql_port"])
	assert.Equal(t, "10.1.81.16", adapted.Config["primary_host"])
	assert.Equal(t, "repl", adapted.Config["replicate_user"])
	assert.Equal(t, "repl@1234556", adapted.Config["replicate_pass"])
	assert.Equal(t, "VFR$3edcXSW@1qaz", adapted.Config["mysql_password"])
	seeds, ok := adapted.Config["group_seeds"].([]any)
	require.True(t, ok)
	assert.Len(t, seeds, 3)
	assert.Equal(t, "10.1.81.16:33067", seeds[0])
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, adapted.Config["group_name"])
}

func TestBuildClusterSwitchDeployRequestForPXC(t *testing.T) {
	req := DeployTaskRequest{
		TaskID:     "cluster-switch-test",
		InstanceID: "one",
		Config: map[string]interface{}{
			"switch_type":      "single-to-pxc",
			"cluster_id":       "pxc-reconfig",
			"cluster_type":     "pxc",
			"replication_user": "repl",
			"replication_pass": "repl@1234556",
			"mysql_password":   "VFR$3edcXSW@1qaz",
		},
	}
	nodes := []map[string]any{
		{"instance_id": "one", "host": "10.1.81.16", "port": 3306, "role": "primary", "datadir": "/data/mysql/3306"},
		{"instance_id": "two", "host": "10.1.81.17", "port": 3306, "role": "secondary"},
	}

	adapted, err := buildClusterSwitchDeployRequest(req, "pxc", nodes)

	require.NoError(t, err)
	assert.Equal(t, "pxc", adapted.Config["deploy_mode"])
	assert.Equal(t, true, adapted.Config["bootstrap"])
	assert.Equal(t, "10.1.81.16", adapted.Config["node_host"])
	assert.Equal(t, 3306, adapted.Config["mysql_port"])
	assert.Equal(t, "/data/mysql/3306", adapted.Config["data_dir"])
	nodesConfig, ok := adapted.Config["nodes"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"10.1.81.16", "10.1.81.17"}, nodesConfig)
}

func TestBuildClusterSwitchDeployRequestForMHA(t *testing.T) {
	req := DeployTaskRequest{
		TaskID:     "cluster-switch-test",
		InstanceID: "one",
		Config: map[string]interface{}{
			"switch_type":      "single-to-mha",
			"cluster_id":       "mha-reconfig",
			"cluster_type":     "mha",
			"replication_user": "repl",
			"replication_pass": "repl@1234556",
			"mysql_password":   "VFR$3edcXSW@1qaz",
		},
	}
	nodes := []map[string]any{
		{"instance_id": "one", "host": "10.1.81.16", "port": 3306, "role": "master"},
		{"instance_id": "two", "host": "10.1.81.17", "port": 3306, "role": "slave"},
		{"instance_id": "three", "host": "10.1.81.18", "port": 3306, "role": "slave"},
	}

	adapted, err := buildClusterSwitchDeployRequest(req, "mha", nodes)

	require.NoError(t, err)
	assert.Equal(t, "mha", adapted.Config["deploy_mode"])
	assert.Equal(t, "10.1.81.16", adapted.Config["manager_host"])
	assert.Equal(t, "10.1.81.16", adapted.Config["master_host"])
	assert.Equal(t, 3306, adapted.Config["master_port"])
	assert.Equal(t, "repl", adapted.Config["repl_user"])
	assert.Equal(t, "repl@1234556", adapted.Config["repl_pass"])
	assert.Equal(t, []any{"10.1.81.17", "10.1.81.18"}, adapted.Config["slave_hosts"])
	assert.Equal(t, []any{3306, 3306}, adapted.Config["slave_ports"])
}
