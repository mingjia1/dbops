package kernel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXinchuangCoreBaseExecuteUsesCompletedAgentTask(t *testing.T) {
	fake := &fakeAgentCaller{resp: map[string]interface{}{"status": "completed", "message": "deployed"}}
	plugin := NewXinchuangCoreBase("TiDB", "/agent/tasks/flavor", fake.call)

	result, err := plugin.Execute(context.Background(), testEnv(), map[string]interface{}{"operation": "deploy", "version": "v8.5.7"})
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "deployed", result.Message)
	require.Len(t, fake.calls, 1)
	assert.Equal(t, "/agent/tasks/flavor", fake.calls[0].Path)
	assert.Equal(t, "tidb", fake.calls[0].Payload["flavor"])
	assert.Equal(t, "deploy", fake.calls[0].Payload["operation"])
	assert.Equal(t, "v8.5.7", fake.calls[0].Payload["version"])
}

func TestXinchuangCoreBaseRejectsUnsuccessfulAgentTasks(t *testing.T) {
	for _, status := range []string{"failed", "running", ""} {
		t.Run(status, func(t *testing.T) {
			fake := &fakeAgentCaller{resp: map[string]interface{}{"status": status, "message": "agent state"}}
			plugin := NewXinchuangCoreBase("tidb", "/agent/tasks/flavor", fake.call)

			_, err := plugin.Execute(context.Background(), testEnv(), map[string]interface{}{"operation": "monitor"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "agent")
		})
	}
}

func TestXinchuangCoreBaseRollbackAfterFailedAgentTask(t *testing.T) {
	calls := make([]agentCall, 0, 2)
	caller := func(_ context.Context, host string, agentPort int, path string, payload map[string]interface{}) (map[string]interface{}, error) {
		calls = append(calls, agentCall{Host: host, AgentPort: agentPort, Path: path, Payload: payload})
		if payload["operation"] == "rollback" {
			return map[string]interface{}{"status": "completed"}, nil
		}
		return map[string]interface{}{"status": "failed", "message": "installation failed"}, nil
	}
	plugin := NewXinchuangCoreBase("tidb", "/agent/tasks/flavor", caller)
	env := testEnv()

	_, err := plugin.Execute(context.Background(), env, map[string]interface{}{"operation": "deploy"})
	require.Error(t, err)
	require.NoError(t, plugin.Rollback(context.Background(), env))
	require.Len(t, calls, 2)
	assert.Equal(t, "deploy", calls[0].Payload["operation"])
	assert.Equal(t, "rollback", calls[1].Payload["operation"])
}

func TestXinchuangCoreBaseSupportsEveryLifecyclePath(t *testing.T) {
	fake := &fakeAgentCaller{resp: map[string]interface{}{"status": "success"}}
	plugin := NewXinchuangCoreBase("opengauss", "/agent/tasks/flavor", fake.call)
	env := testEnv()

	for _, operation := range []LifecycleOperation{
		OperationDeploy, OperationConfigure, OperationBackup, OperationRestore, OperationUpgrade,
		OperationMigrate, OperationHA, OperationReplication, OperationFailover, OperationMonitor,
	} {
		_, err := plugin.Execute(context.Background(), env, map[string]interface{}{"operation": string(operation)})
		require.NoError(t, err, operation)
	}
	require.NoError(t, plugin.Rollback(context.Background(), env))
	require.NoError(t, plugin.Teardown(context.Background(), env))
	require.NoError(t, plugin.Join(context.Background(), env, env.Node))
	require.NoError(t, plugin.Leave(context.Background(), env, env.Node))
	assert.Len(t, fake.calls, 14)
}

func TestXinchuangCoreBaseRejectsInvalidContractInputs(t *testing.T) {
	plugin := NewXinchuangCoreBase("", "", nil)
	err := plugin.Prepare(context.Background(), testEnv())
	require.Error(t, err)

	valid := NewXinchuangCoreBase("dm", "/agent/tasks/flavor", (&fakeAgentCaller{}).call)
	_, err = valid.Execute(context.Background(), testEnv(), map[string]interface{}{"operation": "unknown"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}
