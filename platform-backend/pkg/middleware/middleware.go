package middleware

import (
	"net/http"
	"strings"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		traceID := uuid.New().String()
		ctx.Set("trace_id", traceID)

		logger.Info("Request started",
			zap.String("trace_id", traceID),
			zap.String("method", ctx.Request.Method),
			zap.String("path", ctx.Request.URL.Path),
		)

		ctx.Next()

		logger.Info("Request completed",
			zap.String("trace_id", traceID),
			zap.Int("status", ctx.Writer.Status()),
			zap.Int("response_size", ctx.Writer.Size()),
		)
	}
}

func ErrorHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()

		if len(ctx.Errors) > 0 {
			err := ctx.Errors.Last()
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "Internal server error",
				"error":   err.Error(),
				"trace_id": ctx.GetString("trace_id"),
			})
		}
	}
}

func CORS() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")

		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}

func RequirePermission(permission string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		role := ctx.GetString("role")
		if role == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Unauthorized",
			})
			ctx.Abort()
			return
		}

		if role == "admin" {
			ctx.Next()
			return
		}

		rolePermissions := map[string][]string{
			"dba":      {"instance:*", "deploy:*", "upgrade:*", "backup:*", "restore:*", "monitor:view"},
			"operator": {"instance:view", "deploy:execute", "backup:execute", "restore:execute", "monitor:view"},
			"developer": {"instance:view_own", "backup:apply", "monitor:view_own"},
			"auditor":  {"instance:view", "monitor:view", "audit:view"},
		}

		permissions := rolePermissions[role]
		hasPermission := false
		
		for _, p := range permissions {
			if p == "*" || p == permission || strings.HasPrefix(p, strings.Split(permission, ":")[0]+":*") {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			ctx.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "Permission denied",
			})
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}