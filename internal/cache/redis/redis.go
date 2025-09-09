package redisCache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisCache struct {
	client *redis.Client
}

func NewRedisCache(c *redis.Client) *redisCache {
	return &redisCache{client: c}
}

func (r *redisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, key, data, expiration).Err()
}

// Get retrieves value into target (must pass pointer)
func (c *redisCache) Get(ctx context.Context, key string, target any) error {
	data, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil // key does not exist
	} else if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Delete removes a key
func (c *redisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

// Exists checks if a key exists
func (c *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	return n > 0, err
}
