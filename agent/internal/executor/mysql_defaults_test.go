package executor

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMySQLDefaultsFile_CreateAndCleanup(t *testing.T) {
	f, err := NewMySQLDefaultsFile("127.0.0.1", 3306, "root", "testpass")
	require.NoError(t, err)
	defer f.Cleanup()

	assert.NotEmpty(t, f.Path())
	assert.Contains(t, f.Path(), "mysql_defaults")

	data, err := os.ReadFile(f.Path())
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "host=127.0.0.1")
	assert.Contains(t, content, "port=3306")
	assert.Contains(t, content, "user=root")
	assert.Contains(t, content, "password=testpass")
}

func TestMySQLDefaultsFile_Args(t *testing.T) {
	f, err := NewMySQLDefaultsFile("10.0.0.1", 3307, "repl", "replpass")
	require.NoError(t, err)
	defer f.Cleanup()

	args := f.Args()
	assert.Len(t, args, 1)
	assert.Contains(t, args[0], "--defaults-extra-file=")
}

func TestMySQLDefaultsFile_DefaultValues(t *testing.T) {
	f, err := NewMySQLDefaultsFile("", 0, "", "pass")
	require.NoError(t, err)
	defer f.Cleanup()

	data, err := os.ReadFile(f.Path())
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "port=3306")
	assert.Contains(t, content, "user=root")
}
