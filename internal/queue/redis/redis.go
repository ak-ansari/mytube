package redis

import (
	"context"
	"time"

	"github.com/ak-ansari/mytube/internal/config"
	"github.com/redis/go-redis/v9"
)

type redisQueue struct {
	client *redis.Client
}

func NewRedisQ(cnf *config.Config) *redisQueue {
	addr := cnf.Queue.RedisHost + ":" + cnf.Queue.RedisPort
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	return &redisQueue{client: client}
}

func (rq *redisQueue) Enqueue(ctx context.Context, queueName string, payload []byte) error {
	return rq.client.RPush(ctx, queueName, payload).Err()
}
func (rq *redisQueue) Dequeue(ctx context.Context, queueName string) ([]byte, error) {
	res, err := rq.client.BLPop(ctx, 5*time.Second, queueName).Result()
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
