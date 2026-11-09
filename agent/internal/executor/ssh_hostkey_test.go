package executor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentRejectsInvalidKnownHosts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte("not a known hosts file"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DBOPS_KNOWN_HOSTS", path)
	if err := agentHostKeyCallback("db.example.com")("db.example.com:22", nil, nil); err == nil {
		t.Fatal("expected invalid known_hosts to reject the connection")
	}
}
