package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		traceID := uuid.New().String()
		ctx.Set("trace_id", traceID)
		start := time.Now()

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
			zap.Duration("latency", time.Since(start)),
		)
	}
}

func ErrorHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()

		if len(ctx.Errors) > 0 {
			if ctx.Writer.Status() > 0 {
				return
			}
			err := ctx.Errors.Last()
			status := http.StatusInternalServerError
			message := "Internal server error"
			errMsg := err.Error()
			switch {
			case strings.Contains(errMsg, "validation"):
				status = http.StatusBadRequest
				message = "Invalid request parameters"
			case strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "token"):
				status = http.StatusUnauthorized
				message = "Authentication required"
			case strings.Contains(errMsg, "forbidden") || strings.Contains(errMsg, "permission"):
				status = http.StatusForbidden
				message = "Permission denied"
			case strings.Contains(errMsg, "not found"):
				status = http.StatusNotFound
				message = "Resource not found"
			}
			ctx.JSON(status, gin.H{
				"code":     status,
				"message":  message,
				"trace_id": ctx.GetString("trace_id"),
			})
		}
	}
}

// CORS P0-3: 用白名单代替 Allow-Origin: *, 防止任意站点跨源调用.
// allowedOrigins: 从 config.yaml 注入, 默认 ["http://localhost:3000"].
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = struct{}{}
		}
	}
	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				ctx.Header("Access-Control-Allow-Origin", origin)
				ctx.Header("Vary", "Origin")
				ctx.Header("Access-Control-Allow-Credentials", "true")
			} else if len(allowed) > 0 {
				// 显式拒绝: 不写 Access-Control-Allow-Origin, 浏览器会拦截响应.
				ctx.Header("Vary", "Origin")
			}
		}
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		ctx.Header("Access-Control-Max-Age", "600")

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
		if values, ok := ctx.Get("permissions"); ok {
			if permissions, ok := values.([]string); ok && hasPermission(permissions, permission) {
				ctx.Next()
				return
			}
		}

		rolePermissions := map[string][]string{
			"dba":       {"instance:*", "deploy:*", "upgrade:*", "backup:*", "restore:*", "monitor:view"},
			"operator":  {"instance:view", "deploy:execute", "backup:execute", "restore:execute", "monitor:view"},
			"developer": {"instance:view_own", "backup:apply", "monitor:view_own"},
			"auditor":   {"instance:view", "monitor:view", "audit:view"},
		}

		permissions := rolePermissions[role]
		hasPermission := false
		prefix := ""
		if idx := strings.Index(permission, ":"); idx > 0 {
			prefix = permission[:idx]
		}
		for _, p := range permissions {
			if p == "*" || p == permission {
				hasPermission = true
				break
			}
			if prefix != "" && strings.HasSuffix(p, ":*") && strings.HasPrefix(p, prefix+":") {
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

func hasPermission(permissions []string, required string) bool {
	for _, granted := range permissions {
		if granted == "*" || granted == required {
			return true
		}
		if required == "admin" {
			continue
		}
		if strings.HasSuffix(granted, ":*") {
			prefix := strings.TrimSuffix(granted, "*")
			if strings.HasPrefix(required, prefix) {
				return true
			}
		}
	}
	return false
}
