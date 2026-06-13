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
		ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
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