package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/monkeycode/mysql-ops-platform/pkg/license"
)

type FeatureChecker func(feature license.Feature) bool

func RequireFeature(checker FeatureChecker, feature license.Feature) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if checker(feature) {
			ctx.Next()
			return
		}
		ctx.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "This feature requires " + string(feature) + " which is not available in your current license tier. Please upgrade to Enterprise or Ultimate Edition.",
		})
		ctx.Abort()
	}
}
