package caching

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisCache struct {
	redis *redis.Client
}

func (c *redisCache) Store(ctx context.Context, key string, value any, ttl time.Duration) error {
	_, err := c.redis.SetEx(ctx, key, value, ttl).Result()
	if err != nil {
		return err
	}

	return nil
}

func (c *redisCache) Fetch(ctx context.Context, key string) ([]byte, error) {
	value, err := c.redis.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	if err == redis.Nil {
		return nil, nil
	}

	return value, nil
}
