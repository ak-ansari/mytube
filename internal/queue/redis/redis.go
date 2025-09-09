package redisQueue

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisQueue struct {
	client *redis.Client
}

func NewRedisQ(c *redis.Client) *redisQueue {
	return &redisQueue{client: c}
}

func (r *redisQueue) Enqueue(ctx context.Context, queueName string, payload []byte) error {
	return r.client.RPush(ctx, queueName, payload).Err()
}
func (r *redisQueue) Dequeue(ctx context.Context, queueName string) ([]byte, error) {
	res, err := r.client.BLPop(ctx, 5*time.Second, queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	if len(res) != 2 {
		return nil, nil
	}
	return []byte(res[1]), nil
}
