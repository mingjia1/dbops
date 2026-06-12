package executor

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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

func TestProcessMatchesDatadirAllowsProcessCWD(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("process cwd matching uses /proc")
	}
	datadir := t.TempDir()
	oldwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(datadir))
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	require.True(t, processMatchesDatadir(os.Getpid(), datadir))
}

func TestExecuteInstanceAdminDecommissionRemovesSafeDatadir(t *testing.T) {
	executor := NewTaskExecutor()
	t.Setenv("DBOPS_ALLOW_TMP_DECOMMISSION_TEST", "1")
	root := t.TempDir()
	datadir := filepath.Join(root, "mysql-3307")
	require.NoError(t, os.MkdirAll(datadir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(datadir, "mysql.pid"), []byte("999999"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(datadir, "ibdata1"), []byte("data"), 0o644))

	result, err := executor.ExecuteInstanceAdmin(context.Background(), DeployTaskRequest{
		TaskID: "decommission-test",
		Config: map[string]interface{}{
			"action":      "decommission",
			"datadir":     datadir,
			"target_port": 3307,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "completed", result.Status)
	assert.Contains(t, result.Message, "removed datadir=")
	_, statErr := os.Stat(datadir)
	assert.True(t, os.IsNotExist(statErr))
}

func TestExecuteInstanceAdminDecommissionRejectsUnsafeDatadir(t *testing.T) {
	executor := NewTaskExecutor()

	result, err := executor.ExecuteInstanceAdmin(context.Background(), DeployTaskRequest{
		TaskID: "decommission-test",
		Config: map[string]interface{}{
			"action":      "decommission",
			"datadir":     "/data/mysql",
			"target_port": 3307,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "refuse to remove datadir outside managed paths")
}

func TestValidateDecommissionDatadirAllowsManagedTempDir(t *testing.T) {
	for _, datadir := range []string{
		filepath.Join(os.TempDir(), "dbops-mysql-24310"),
		filepath.Join(os.TempDir(), "dbops-mha57-24320"),
		filepath.Join(os.TempDir(), "dbops-pxc-24410"),
	} {
		require.NoError(t, validateDecommissionDatadir(datadir))
	}
}
