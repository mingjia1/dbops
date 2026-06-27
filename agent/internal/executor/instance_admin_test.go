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

func TestExecuteInstanceAdminReadConfigResolvesDefaultPathFromDatadir(t *testing.T) {
	executor := NewTaskExecutor()
	datadir := t.TempDir()
	configFile := filepath.Join(datadir, "my.cnf")
	require.NoError(t, os.WriteFile(configFile, []byte("[mysqld]\nport=3307\n"), 0o600))

	result, err := executor.ExecuteInstanceAdmin(context.Background(), DeployTaskRequest{
		TaskID: "admin-test",
		Config: map[string]interface{}{
			"action":      "read_config",
			"path":        "/etc/my.cnf",
			"datadir":     datadir,
			"target_port": 3307,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "completed", result.Status)
	assert.Contains(t, result.Message, "port=3307")
	data, ok := result.Data.(map[string]string)
	require.True(t, ok)
	assert.Equal(t, configFile, data["path"])
	assert.Contains(t, data["content"], "port=3307")
}

func TestExecuteInstanceAdminReadConfigReportsDefaultPathCandidates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip: candidate paths may exist on Windows (e.g. MSYS /etc)")
	}
	executor := NewTaskExecutor()
	datadir := t.TempDir()

	result, err := executor.ExecuteInstanceAdmin(context.Background(), DeployTaskRequest{
		TaskID: "admin-test",
		Config: map[string]interface{}{
			"action":      "read_config",
			"path":        "/etc/my.cnf",
			"datadir":     datadir,
			"target_port": 3307,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "failed", result.Status)
	assert.Contains(t, result.Message, "config file not found; tried:")
	normalizedMessage := filepath.ToSlash(result.Message)
	assert.Contains(t, normalizedMessage, "/etc/dbops-pxc/dbops-pxc-3307.cnf")
	assert.Contains(t, normalizedMessage, filepath.ToSlash(filepath.Join(datadir, "my.cnf")))
}

func TestConfigPathCandidatesIncludesMHAPath(t *testing.T) {
	candidates := configPathCandidates(map[string]interface{}{
		"target_port": 3307,
		"datadir":     "/data/mysql/mha-3307",
	})

	foundMHA := false
	for _, c := range candidates {
		if filepath.ToSlash(c) == "/etc/mha/app1.cnf" {
			foundMHA = true
			break
		}
	}
	require.True(t, foundMHA, "configPathCandidates should include /etc/mha/app1.cnf, got: %v", candidates)

	foundDefault := false
	for _, c := range candidates {
		if filepath.ToSlash(c) == "/etc/my.cnf" {
			foundDefault = true
			break
		}
	}
	require.True(t, foundDefault, "configPathCandidates should include /etc/my.cnf, got: %v", candidates)

	foundPXC := false
	for _, c := range candidates {
		if filepath.ToSlash(c) == "/etc/dbops-pxc/dbops-pxc-3307.cnf" {
			foundPXC = true
			break
		}
	}
	require.True(t, foundPXC, "configPathCandidates should include PXC path when port is given, got: %v", candidates)
}

func TestConfigPathCandidatesDeduplicatesPaths(t *testing.T) {
	candidates := configPathCandidates(map[string]interface{}{})

	seen := make(map[string]bool)
	for _, c := range candidates {
		if seen[c] {
			t.Errorf("duplicate path found: %s", c)
		}
		seen[c] = true
	}
}

func TestConfigPathCandidatesWithMGRPortHasMHAFallback(t *testing.T) {
	candidates := configPathCandidates(map[string]interface{}{
		"target_port": 3306,
	})

	foundMHA := false
	for _, c := range candidates {
		if filepath.ToSlash(c) == "/etc/mha/app1.cnf" {
			foundMHA = true
			break
		}
	}
	require.True(t, foundMHA, "configPathCandidates should always include /etc/mha/app1.cnf as a candidate, got: %v", candidates)
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

func TestInstancePIDReadsMysqldPIDFile(t *testing.T) {
	datadir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(datadir, "mysqld.pid"), []byte("12345"), 0o644))

	pid, ok := instancePID(datadir)
	require.True(t, ok)
	assert.Equal(t, 12345, pid)
}

func TestFindProcessByDatadirUsesProcCmdlineOrCWD(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("process matching uses /proc")
	}
	datadir := t.TempDir()
	oldwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(datadir))
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	pid, ok := findProcessByDatadir(datadir)
	require.True(t, ok)
	assert.Equal(t, os.Getpid(), pid)
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
