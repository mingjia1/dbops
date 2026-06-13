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

func NewRedis(addr, password string, db int) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
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
