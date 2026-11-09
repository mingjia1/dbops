package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTOFURejectsInvalidKnownHosts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, os.WriteFile(path, []byte("not a known hosts file"), 0o600))
	err := tofuHostKeyCallback(path, "db.example.com")("db.example.com:22", nil, nil)
	require.ErrorContains(t, err, "cannot parse known_hosts")
}
