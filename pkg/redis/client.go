package client

import (
	"fmt"

	"github.com/ak-ansari/mytube/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewRedisClient(cnf *config.Redis) *redis.Client {
	addr := fmt.Sprintf("%s:%s", cnf.RedisHost, cnf.RedisPort)
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}
