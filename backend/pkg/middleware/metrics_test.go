package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMetricsBuildInfoIsNotDuplicated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/metrics", MetricsHandler(MetricsStats{StartTime: time.Now(), Version: "1.2.3"}))
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, 1, strings.Count(response.Body.String(), "dbops_build_info{version=\"1.2.3\"} 1"))
	require.NotContains(t, response.Body.String(), "\ndbops_build_info 1\n")
}
