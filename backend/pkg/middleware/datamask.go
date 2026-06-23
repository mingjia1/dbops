package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/internal/models"
	"github.com/monkeycode/mysql-ops-platform/internal/services"
)

type bodyWriter struct {
	gin.ResponseWriter
	body      *bytes.Buffer
	overflow  bool
	maxBuffer int
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	if !w.overflow && w.body.Len()+len(b) > w.maxBuffer {
		w.overflow = true
		w.body.Reset()
	} else if !w.overflow {
		w.body.Write(b)
	}
	return w.ResponseWriter.Write(b)
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

		if ctx.Writer.Status() != http.StatusOK || w.overflow {
			return
		}
		contentType := ctx.Writer.Header().Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			return
		}

		bodyBytes := w.body.Bytes()
		if len(bodyBytes) == 0 {
			return
		}

		var data interface{}
		if err := json.Unmarshal(bodyBytes, &data); err != nil {
			return
		}

		masked := maskJSON(data, applicable)
		maskedBytes, err := json.Marshal(masked)
		if err != nil {
			return
		}

		w.body.Reset()
		w.ResponseWriter.Write(maskedBytes)
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
		if len(value) <= 4 {
			return strings.Repeat("*", len(value))
		}
		return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
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
