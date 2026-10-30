package middleware

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeRedis P1-C: 注入型计数实现, 可切换为故障以验证回退路径.
type fakeRedis struct {
	calls   int64
	fail    bool
	expired int
}

func (f *fakeRedis) Incr(ctx context.Context, key string) (int64, error) {
	if f.fail {
		return 0, errors.New("redis down")
	}
	f.calls++
	return f.calls, nil
}

func (f *fakeRedis) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if f.fail {
		return errors.New("redis down")
	}
	f.expired++
	return nil
}

// Redis 正常时: 超过 limit 即拒绝, 窗口首击设置过期.
func TestRedisRateLimiter_AllowAndLimit(t *testing.T) {
	fake := &fakeRedis{}
	rl := newRedisRateLimiterWithClient(fake, 2, time.Minute, NewRateLimiter(100, time.Minute))

	if !rl.Allow("login:1.2.3.4") || !rl.Allow("login:1.2.3.4") {
		t.Fatal("first two attempts should pass")
	}
	if rl.Allow("login:1.2.3.4") {
		t.Fatal("third attempt should be rate limited")
	}
	if fake.expired != 1 {
		t.Errorf("Expire called %d times, want 1 (only on first hit)", fake.expired)
	}
	if rl.Degraded() {
		t.Error("Degraded = true, want false while redis is healthy")
	}
}

// Redis 故障时: 自动回退内存限流, 不 panic 不放任 (回退器仍然限流).
func TestRedisRateLimiter_FallsBackWhenRedisDown(t *testing.T) {
	fake := &fakeRedis{fail: true}
	memory := NewRateLimiter(1, time.Minute)
	rl := newRedisRateLimiterWithClient(fake, 5, time.Minute, memory)

	if !rl.Allow("login:5.6.7.8") {
		t.Fatal("first attempt should pass via fallback")
	}
	if rl.Allow("login:5.6.7.8") {
		t.Fatal("second attempt should be rejected by in-memory fallback (limit=1)")
	}
	if !rl.Degraded() {
		t.Error("Degraded = false, want true after redis error")
	}
}

// nil 安全: 未配置 Redis 时放行 (由调用方决定是否启用).
func TestRedisRateLimiter_NilSafe(t *testing.T) {
	var rl *RedisRateLimiter
	if !rl.Allow("any") {
		t.Error("nil limiter should allow")
	}
	if NewRedisRateLimiter(nil, 5, time.Minute, nil) != nil {
		t.Error("NewRedisRateLimiter(nil) should return nil")
	}
}

// P2-B: 禁用开关语义 (1/true/yes/on 关闭限流, 其他不禁用).
func TestLoginRateLimitDisabledSemantics(t *testing.T) {
	for _, tc := range []struct {
		val  string
		want bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"yes", true},
		{"on", true},
		{"0", false},
		{"false", false},
		{"", false},
		{"garbage", false},
	} {
		t.Setenv("DBOPS_LOGIN_RATELIMIT_DISABLE", tc.val)
		if got := loginRateLimitDisabled(); got != tc.want {
			t.Errorf("loginRateLimitDisabled() with %q = %v, want %v", tc.val, got, tc.want)
		}
	}
}
