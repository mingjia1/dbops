package controllers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/stretchr/testify/require"
)

func TestVersionControllerListSupportedMarksLocalAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	versionDir := filepath.Join(root, "mysql", "8.0.36")
	require.NoError(t, os.MkdirAll(versionDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(versionDir, "mysql-8.0.36-linux-glibc2.17-x86_64.tar.xz"), []byte("ok"), 0o644))

	controller := NewVersionController(services.NewVersionCatalog(), root)
	router := gin.New()
	router.GET("/versions/supported", controller.ListSupported)

	req := httptest.NewRequest(http.MethodGet, "/versions/supported?flavor=mysql", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"version":"8.0.36"`)
	require.Contains(t, rec.Body.String(), `"local_available":true`)
}
