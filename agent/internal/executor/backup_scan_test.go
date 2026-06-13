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
	assert.True(t, byName["full-001"].IsDir)
	assert.True(t, byName["full-001"].Complete)
	assert.Equal(t, "full", byName["full-002.xbstream"].BackupType)
	assert.False(t, byName["full-002.xbstream"].IsDir)
	assert.True(t, byName["full-002.xbstream"].Complete)
	assert.Equal(t, "logical", byName["logical-001.sql.gz"].BackupType)
	assert.True(t, byName["logical-001.sql.gz"].Complete)
	_, partialIncluded := byName["full-003.xbstream.partial"]
	assert.False(t, partialIncluded)
}

func TestExecuteColdDatadirBackupCreatesArchive(t *testing.T) {
	t.Setenv("DBOPS_ALLOW_TMP_DECOMMISSION_TEST", "1")
	root := t.TempDir()
	datadir := filepath.Join(root, "mysql-3307")
	targetDir := filepath.Join(root, "backup")
	require.NoError(t, os.MkdirAll(datadir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(datadir, "ibdata1"), []byte("data"), 0o644))

	executor := NewTaskExecutor()
	result, err := executor.ExecuteBackup(context.Background(), DeployTaskRequest{
		TaskID: "cold-backup-test",
		Config: map[string]interface{}{
			"backup_type": "full",
			"target_dir":  targetDir,
			"mysql_host":  "127.0.0.1",
			"mysql_port":  1,
			"mysql_user":  "root",
			"mysql_pass":  "wrong",
			"datadir":     datadir,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "completed", result.Status)
	data, ok := result.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "cold_datadir", data["backup_method"])
	backupPath, _ := data["backup_path"].(string)
	require.NotEmpty(t, backupPath)
	require.FileExists(t, backupPath)
	require.Greater(t, data["file_size"].(int64), int64(0))
	require.NotEmpty(t, data["checksum"])
}
