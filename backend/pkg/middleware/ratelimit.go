package middleware

import (
	"net/http"
	"os"
	"strings"
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
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

func NewRateLimiter(limit int, interval time.Duration) *RateLimiter {
	r := &RateLimiter{
		buckets:  make(map[string][]time.Time),
		limit:    limit,
		interval: interval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
	go func() {
		defer close(r.doneCh)
		r.cleanup()
	}()
	return r
}

func (r *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			cutoff := time.Now().Add(-10 * time.Minute)
			for key, bucket := range r.buckets {
				i := 0
				for ; i < len(bucket); i++ {
					if bucket[i].After(cutoff) {
						break
					}
				}
				if i >= len(bucket) {
					delete(r.buckets, key)
				} else {
					r.buckets[key] = bucket[i:]
				}
			}
			r.mu.Unlock()
		case <-r.stopCh:
			return
		}
	}
}

// Stop P1-D: 停止清理协程并等待其退出. 幂等 (重复调用不 panic).
func (r *RateLimiter) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() { close(r.stopCh) })
	<-r.doneCh
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

// KeyLimiter P1-C: 限流器统一接口. 内存版 (*RateLimiter) 与 Redis 版
// (*RedisRateLimiter) 都实现它, 中间件只依赖这个抽象.
type KeyLimiter interface {
	Allow(key string) bool
}

// RateLimitByIP P0-3: 全局限流中间件. 默认 100 req/s per IP, 超过返 429.
func RateLimitByIP(limiter KeyLimiter) gin.HandlerFunc {
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

// loginRateLimitDisabled P2-B: 开关语义与 .env.example 对齐.
// 接受 1/true/yes/on (启用禁用) 与 0/false/no/off (不禁用),
// 避免以前只认 "=1" 导致 .env.example 写 false 时被误读.
func loginRateLimitDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DBOPS_LOGIN_RATELIMIT_DISABLE"))) {
	case "1", "true", "yes", "on":
		return true
	default: // "", "0", "false", "no", "off" 及其他取值都视为不禁用
		return false
	}
}

// LoginRateLimit 登录端点专用, 更严格 (5 req/min per IP).
// P1-C: limiter 可注入 Redis 版 (多副本共享); P2-B: 开关支持 true/false 写法.
// 通过环境变量 DBOPS_LOGIN_RATELIMIT_DISABLE=1/true 可禁用 (用于 e2e).
func LoginRateLimit(limiter KeyLimiter) gin.HandlerFunc {
	if loginRateLimitDisabled() {
		return func(c *gin.Context) { c.Next() }
	}
	return func(ctx *gin.Context) {
		ip := ctx.ClientIP()
		if !limiter.Allow("login:" + ip) {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "too many login attempts, please try again later",
			})
			return
		}
		ctx.Next()
	}
}
