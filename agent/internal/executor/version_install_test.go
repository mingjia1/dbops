package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadFileRejectsChecksumMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("mysql tarball"))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "mysql.tar.gz")
	err := downloadFile(context.Background(), server.URL+"/mysql.tar.gz", dest, "0000000000000000000000000000000000000000000000000000000000000000")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sha256 mismatch")
	_, statErr := os.Stat(dest)
	assert.NoError(t, statErr)
}
