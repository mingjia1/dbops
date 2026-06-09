package executor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteBackupScanRecognizesCompleteBackupFilesAndDirs(t *testing.T) {
	root := t.TempDir()
	fullDir := filepath.Join(root, "full-001")
	require.NoError(t, os.MkdirAll(fullDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fullDir, "xtrabackup_checkpoints"), []byte("backup_type = full-backuped\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(fullDir, "xtrabackup_info"), []byte("uuid = test\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(fullDir, "backup-my.cnf"), []byte("[mysqld]\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "full-002.xbstream"), []byte("xbstream-data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "full-003.xbstream.partial"), []byte("partial"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "logical-001.sql.gz"), []byte("dump"), 0o644))

	executor := NewTaskExecutor()
	result, err := executor.ExecuteBackupScan(context.Background(), DeployTaskRequest{
		TaskID: "scan-test",
		Config: map[string]interface{}{
			"paths": []interface{}{root},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "completed", result.Status)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	backups, ok := data["backups"].([]BackupScanFile)
	require.True(t, ok)
	require.Len(t, backups, 3)

	byName := map[string]BackupScanFile{}
	for _, item := range backups {
		byName[item.FileName] = item
	}
	assert.Equal(t, "full", byName["full-001"].BackupType)
	assert.Equal(t, "full", byName["full-002.xbstream"].BackupType)
	assert.Equal(t, "logical", byName["logical-001.sql.gz"].BackupType)
	_, partialIncluded := byName["full-003.xbstream.partial"]
	assert.False(t, partialIncluded)
}
