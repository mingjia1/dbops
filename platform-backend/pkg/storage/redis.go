package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	Client *redis.Client
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func NewRedis(addr, password string, db int, configOverride ...*RedisConfig) (*Redis, error) {
	cfg := &RedisConfig{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	if len(configOverride) > 0 && configOverride[0] != nil {
		oc := configOverride[0]
		if oc.Addr != "" {
			cfg.Addr = oc.Addr
		}
		if oc.Password != "" {
			cfg.Password = oc.Password
		}
		if oc.DB != 0 {
			cfg.DB = oc.DB
		}
		if oc.PoolSize > 0 {
			cfg.PoolSize = oc.PoolSize
		}
		if oc.MinIdleConns > 0 {
			cfg.MinIdleConns = oc.MinIdleConns
		}
		if oc.DialTimeout > 0 {
			cfg.DialTimeout = oc.DialTimeout
		}
		if oc.ReadTimeout > 0 {
			cfg.ReadTimeout = oc.ReadTimeout
		}
		if oc.WriteTimeout > 0 {
			cfg.WriteTimeout = oc.WriteTimeout
		}
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &Redis{Client: client}, nil
}

func (r *Redis) Close() error {
	if r == nil || r.Client == nil {
		return nil
	}
	return r.Client.Close()
}

func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	if r == nil || r.Client == nil {
		return "", fmt.Errorf("redis not available")
	}
	return r.Client.Get(ctx, key).Result()
}

func (r *Redis) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if r == nil || r.Client == nil {
		return fmt.Errorf("redis not available")
	}
	return r.Client.Set(ctx, key, value, ttl).Err()
}

func (r *Redis) Del(ctx context.Context, keys ...string) error {
	if r == nil || r.Client == nil {
		return fmt.Errorf("redis not available")
	}
	return r.Client.Del(ctx, keys...).Err()
}
