package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/internal/models"
	"github.com/jackcode/mysql-ops-platform/internal/services"
)

type bodyWriter struct {
	gin.ResponseWriter
	body        *bytes.Buffer
	overflow    bool
	passthrough bool // 检测到非 JSON 响应 (如 SSE text/event-stream) 后全程直通
	maxBuffer   int
}

// Write P1-1: 修复双写 bug —— 旧实现先把原始 body 写进底层 ResponseWriter,
// 掩码后又把 masked 追加写一遍, 客户端收到 "原始+掩码" 两段拼接的损坏响应.
// 现在: JSON 响应只入缓冲, 待掩码后一次性写出; 非 JSON / 超限响应直接透传.
func (w *bodyWriter) Write(b []byte) (int, error) {
	if !w.passthrough && !w.overflow {
		if ct := w.Header().Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "application/json") {
			w.passthrough = true
			w.body.Reset()
		}
	}
	if w.passthrough || w.overflow {
		return w.ResponseWriter.Write(b)
	}
	if w.body.Len()+len(b) > w.maxBuffer {
		w.overflow = true
		w.body.Reset()
		return w.ResponseWriter.Write(b)
	}
	w.body.Write(b)
	return len(b), nil
}

// Flush 透传 flush (SSE 需要); 直通模式下语义不变.
func (w *bodyWriter) Flush() {
	w.ResponseWriter.Flush()
}

func DataMask(maskingService *services.MaskingService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		role := ctx.GetString("role")
		if role == "" || role == "admin" {
			ctx.Next()
			return
		}

		rules, err := maskingService.GetEnabledRules(ctx.Request.Context())
		if err != nil || len(rules) == 0 {
			ctx.Next()
			return
		}

		applicable := make([]models.MaskingRule, 0)
		for _, rule := range rules {
			for _, r := range rule.Roles {
				if r == role || r == "*" {
					applicable = append(applicable, rule)
					break
				}
			}
		}
		if len(applicable) == 0 {
			ctx.Next()
			return
		}

		w := &bodyWriter{body: &bytes.Buffer{}, ResponseWriter: ctx.Writer, maxBuffer: 2 << 20} // 2MB cap
		ctx.Writer = w
		ctx.Next()

		// 直通 (非 JSON) 或超限截断的响应已经写到底层, 不再二次处理.
		if w.passthrough || w.overflow || ctx.Writer.Status() != http.StatusOK {
			return
		}
		contentType := w.Header().Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			// 未知 Content-Type 且没写过字节 → 保守起见原样写出.
			if w.body.Len() > 0 {
				_, _ = w.ResponseWriter.Write(w.body.Bytes())
			}
			return
		}

		bodyBytes := w.body.Bytes()
		if len(bodyBytes) == 0 {
			return
		}

		var data interface{}
		if err := json.Unmarshal(bodyBytes, &data); err != nil {
			// 不是合法 JSON → 原样透传, 绝不能丢弃响应.
			_, _ = w.ResponseWriter.Write(bodyBytes)
			return
		}

		masked := maskJSON(data, applicable)
		maskedBytes, err := json.Marshal(masked)
		if err != nil {
			_, _ = w.ResponseWriter.Write(bodyBytes)
			return
		}

		_, _ = w.ResponseWriter.Write(maskedBytes)
	}
}

func maskJSON(data interface{}, rules []models.MaskingRule) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			result[key] = maskJSON(val, rules)
			if str, ok := val.(string); ok {
				for _, rule := range rules {
					if matchField(key, rule.FieldPath) {
						if rule.Pattern == "" || matchesPattern(str, rule.Pattern) {
							result[key] = applyMask(str, rule.Algorithm, rule.Replacement)
						}
					}
				}
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = maskJSON(val, rules)
		}
		return result
	default:
		return data
	}
}

func matchField(key, fieldPath string) bool {
	if fieldPath == "" {
		return false
	}
	parts := strings.Split(fieldPath, ".")
	last := parts[len(parts)-1]
	return strings.EqualFold(key, last)
}

func matchesPattern(value, pattern string) bool {
	if pattern == "" {
		return true
	}
	return strings.Contains(value, pattern)
}

func applyMask(value string, algorithm models.MaskingAlgorithm, replacement string) string {
	switch algorithm {
	case models.MaskingMD5:
		// P2: 与 services.MaskMD5Mask 统一实现, 避免两套逻辑漂移.
		return services.MaskMD5Mask(value)
	case models.MaskingMask:
		if replacement != "" {
			return replacement
		}
		if len(value) <= 4 {
			return strings.Repeat("*", len(value))
		}
		return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
	case models.MaskingReplace:
		if replacement != "" {
			return replacement
		}
		return "***"
	default:
		return "***"
	}
}
