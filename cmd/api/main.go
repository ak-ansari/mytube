package main

import (
	"log"

	"github.com/ak-ansari/mytube/internal/api"
	redisCache "github.com/ak-ansari/mytube/internal/cache/redis"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/db"
	client "github.com/ak-ansari/mytube/internal/pkg/redis"
	redisQueue "github.com/ak-ansari/mytube/internal/queue/redis"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
)

func main() {
	conf, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}
	dbPool, err := db.NewPool(conf)
	if err != nil {
		log.Fatal(err)
	}
	client := client.NewRedisClient(&conf.Redis)
	queue := redisQueue.NewRedisQ(client)
	cache := redisCache.NewRedisCache(client)

	objStore, err := storage.NewS3Store()
	if err != nil {
		log.Fatal(err)
	}
	repo := postgres.NewVideoRepo(dbPool)
	service := services.NewVideoService(objStore, repo, queue, cache, conf.Redis.RedisQueueName)
	r := api.SetupRouter(service)
	r.Run(":" + conf.Server.HttpPort)
}
