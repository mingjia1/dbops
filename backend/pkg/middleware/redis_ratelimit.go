package middleware

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisCounter P1-C: 限流所需的最小 Redis 操作集. 用窄接口是为了能在单测里
// 注入 fake, 不必起真实 Redis.
type redisCounter interface {
	// Incr 自增并返回新值.
	Incr(ctx context.Context, key string) (int64, error)
	// Expire 设置 key 过期时间 (仅窗口首击时调用).
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

// redisCounterAdapter 把 *redis.Client 适配成 redisCounter.
type redisCounterAdapter struct {
	client *redis.Client
}

func (a redisCounterAdapter) Incr(ctx context.Context, key string) (int64, error) {
	return a.client.Incr(ctx, key).Result()
}

func (a redisCounterAdapter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return a.client.Expire(ctx, key, ttl).Err()
}

// RedisRateLimiter P1-C: 基于 Redis 的固定窗口限流.
//
// 之前的登录限流是进程内 map —— 重启丢计数, 扩第二个副本会出现"任务在 A 副本、
// 限流在 B 副本"的割裂 (P1-7). Redis 已在技术栈内, 这里用 INCR + EXPIRE 让
// 计数跨副本共享.
//
// 降级策略: Redis 不可达/超时 → 自动回退到进程内 RateLimiter 并只告警一次,
// 绝不因为限流存储故障把登录接口打挂.
type RedisRateLimiter struct {
	client   redisCounter
	limit    int
	interval time.Duration
	fallback *RateLimiter
	// timeout 单次 Redis 调用上限, 避免 Redis 挂起把登录请求拖死.
	timeout time.Duration

	degraded atomic.Bool
}

// NewRedisRateLimiter fallback 为 nil 时超限判定直接放行 (fail-open, 仅用于测试).
func NewRedisRateLimiter(client *redis.Client, limit int, interval time.Duration, fallback *RateLimiter) *RedisRateLimiter {
	if client == nil {
		return nil
	}
	return &RedisRateLimiter{
		client:   redisCounterAdapter{client: client},
		limit:    limit,
		interval: interval,
		fallback: fallback,
		timeout:  750 * time.Millisecond,
	}
}

// newRedisRateLimiterWithClient 内部/测试用: 直接注入计数实现.
func newRedisRateLimiterWithClient(client redisCounter, limit int, interval time.Duration, fallback *RateLimiter) *RedisRateLimiter {
	return &RedisRateLimiter{
		client:   client,
		limit:    limit,
		interval: interval,
		fallback: fallback,
		timeout:  750 * time.Millisecond,
	}
}

// Allow 记录一次命中, 返回 true=放行 / false=超限.
func (r *RedisRateLimiter) Allow(key string) bool {
	if r == nil || r.client == nil {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()

	fullKey := "ratelimit:" + key
	n, err := r.client.Incr(ctx, fullKey)
	if err != nil {
		r.degrade(err)
		if r.fallback != nil {
			return r.fallback.Allow(key)
		}
		return true
	}
	if n == 1 {
		// 只在窗口首击设置过期, 让 TTL 跟随窗口起点.
		if expErr := r.client.Expire(ctx, fullKey, r.interval); expErr != nil {
			r.degrade(expErr)
		}
	}
	r.recover()
	return int(n) <= r.limit
}

// Degraded 返回当前是否处于"回退到内存限流"状态 (可观测/测试用).
func (r *RedisRateLimiter) Degraded() bool {
	return r != nil && r.degraded.Load()
}

func (r *RedisRateLimiter) degrade(err error) {
	if r.degraded.CompareAndSwap(false, true) {
		log.Printf("WARN: rate limit Redis unavailable, falling back to in-memory limiter: %v", err)
	}
}

func (r *RedisRateLimiter) recover() {
	if r.degraded.CompareAndSwap(true, false) {
		log.Printf("INFO: rate limit Redis recovered, back to shared counting")
	}
}
