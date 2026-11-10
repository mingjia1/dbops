package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackcode/mysql-ops-platform/pkg/license"
)

// FeatureChecker P1-A: 判定当前请求是否具备某商业特性.
// 带 *gin.Context 是为了能用请求 context 查询 license (可取消/带超时),
// 而不是在闭包里用 context.Background() 绕开请求生命周期.
type FeatureChecker func(ctx *gin.Context, feature license.Feature) bool

// RequireFeature 商业版功能门禁 (P1-1/P1-1 尾款): 不满足时返回结构化 403,
// 便于前端按 error=feature_not_licensed 本地化提示.
func RequireFeature(checker FeatureChecker, feature license.Feature) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if checker(ctx, feature) {
			ctx.Next()
			return
		}
		ctx.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"error":   "feature_not_licensed",
			"feature": string(feature),
			"message": "该功能需要 " + string(feature) + "，当前许可证版本不支持。请升级到企业版或旗舰版。",
		})
		ctx.Abort()
	}
}
