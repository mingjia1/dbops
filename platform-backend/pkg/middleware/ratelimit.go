package middleware

import (
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 简单的滑动窗口 (1 秒粒度) 内存限流. 适用于单机/小集群部署.
// 大规模部署应替换为 Redis 限流.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string][]time.Time
	limit    int
	interval time.Duration
}

func NewRateLimiter(limit int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[string][]time.Time),
		limit:    limit,
		interval: interval,
	}
}

// Allow 记录一次命中, 返回 true=放行 / false=超限.
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-r.interval)
	bucket := r.buckets[key]
	// 丢弃窗口外的旧记录.
	i := 0
	for ; i < len(bucket); i++ {
		if bucket[i].After(cutoff) {
			break
		}
	}
	bucket = bucket[i:]
	if len(bucket) >= r.limit {
		r.buckets[key] = bucket
		return false
	}
	bucket = append(bucket, now)
	r.buckets[key] = bucket
	return true
}

// RateLimitByIP P0-3: 全局限流中间件. 默认 100 req/s per IP, 超过返 429.
func RateLimitByIP(limiter *RateLimiter) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ip := ctx.ClientIP()
		if !limiter.Allow(ip) {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "rate limit exceeded",
			})
			return
		}
		ctx.Next()
	}
}

// LoginRateLimit 登录端点专用, 更严格 (5 req/min per IP).
// 通过环境变量 DBOPS_LOGIN_RATELIMIT_DISABLE=1 可禁用 (用于 e2e).
func LoginRateLimit(limiter *RateLimiter) gin.HandlerFunc {
	if os.Getenv("DBOPS_LOGIN_RATELIMIT_DISABLE") == "1" {
		return func(c *gin.Context) { c.Next() }
	}
	return func(ctx *gin.Context) {
		ip := ctx.ClientIP()
		if !limiter.Allow("login:"+ip) {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "too many login attempts, please try again later",
			})
			return
		}
		ctx.Next()
	}
}
