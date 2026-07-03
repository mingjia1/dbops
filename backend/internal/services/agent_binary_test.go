package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindAgentBinaryRequiresLinuxELF(t *testing.T) {
	root := t.TempDir()
	agentBin := filepath.Join(root, "agent", "bin")
	require.NoError(t, os.MkdirAll(agentBin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentBin, "agent"), []byte("MZwindows"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentBin, "agent-linux-amd64"), []byte{0x7f, 'E', 'L', 'F', 1}, 0o755))

	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	path, err := findAgentBinary()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("agent", "bin", "agent-linux-amd64"), path)
}

func TestFindAgentBinaryRejectsWindowsOnlyBinary(t *testing.T) {
	root := t.TempDir()
	agentBin := filepath.Join(root, "agent", "bin")
	require.NoError(t, os.MkdirAll(agentBin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentBin, "agent"), []byte("MZwindows"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(agentBin, "agent.exe"), []byte("MZwindows"), 0o755))

	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	_, err = findAgentBinary()
	require.Error(t, err)
}
