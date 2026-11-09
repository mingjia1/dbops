package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/services"
	"github.com/stretchr/testify/require"
)

func TestTaskStreamRejectsNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	controller := NewWSController(services.NewWSHub(), services.NewMessageBus())
	controller.RegisterRoutes(r.Group("/api/v1"), func(ctx *gin.Context) {
		ctx.Set("role", "developer")
	})

	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/stream/task-1", nil))

	require.Equal(t, http.StatusForbidden, response.Code)
}
