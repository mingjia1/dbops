package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/pkg/license"
)

// P1-A: 特性门禁 —— 放行与拦截路径、结构化错误体.
func TestRequireFeature(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newRouter := func(hasFeature bool) *gin.Engine {
		r := gin.New()
		r.GET("/gated", RequireFeature(func(ctx *gin.Context, f license.Feature) bool {
			return hasFeature
		}, license.FeatureDataMasking), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
		return r
	}

	t.Run("licensed passes through", func(t *testing.T) {
		w := httptest.NewRecorder()
		newRouter(true).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/gated", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
	})

	t.Run("unlicensed blocked with structured 403", func(t *testing.T) {
		w := httptest.NewRecorder()
		newRouter(false).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/gated", nil))
		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", w.Code)
		}
		var body struct {
			Code    int    `json:"code"`
			Error   string `json:"error"`
			Feature string `json:"feature"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Code != 403 || body.Error != "feature_not_licensed" || body.Feature != "data_masking" {
			t.Errorf("body = %+v, want code=403 error=feature_not_licensed feature=data_masking", body)
		}
	})
}
