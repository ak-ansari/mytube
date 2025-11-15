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
func (c *redisCache) GetFromHash(ctx context.Context, hash string, key string) (string, error) {
	result, err := c.client.HGet(ctx, hash, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	return result, nil
}
func (c *redisCache) GetAllFromHash(ctx context.Context, hash string) (map[string]string, error) {
	result, err := c.client.HGetAll(ctx, hash).Result()
	if err != nil {
		if err == redis.Nil {
			return make(map[string]string), nil
		}
		return nil, err
	}
	return result, nil
}
func (c *redisCache) DeleteFromHash(ctx context.Context, hash string, key string) error {
	_, err := c.client.HDel(ctx, hash, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil
		}
		return err
	}
	return nil
}

// SetNX sets a key only if it does not already exist
func (c *redisCache) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}

	return c.client.SetNX(ctx, key, data, expiration).Result()
}

func (c *redisCache) DeleteLock(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}
