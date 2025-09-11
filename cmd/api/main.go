package main

import (
	"log"
	"os"

	"github.com/ak-ansari/mytube/internal/api"
	redisCache "github.com/ak-ansari/mytube/internal/cache/redis"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/db"
	"github.com/ak-ansari/mytube/internal/pkg/logger"
	client "github.com/ak-ansari/mytube/internal/pkg/redis"
	redisQueue "github.com/ak-ansari/mytube/internal/queue/redis"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
)

func main() {
	// Load config
	conf, err := config.GetConfig()
	if err != nil {
		// at this point we don’t have a logger, so print and exit
		panic(err)
	}

	// Initialize logger
	logr, err := logger.NewZapLogger(conf.Env)
	if err != nil {
		log.Fatal(err)

	}
	defer logr.Flush()

	// DB pool
	dbPool, err := db.NewPool(conf, logr)
	if err != nil {
		logr.Fatal("failed to init db pool", logger.Error(err))
	}

	// Redis client
	client := client.NewRedisClient(&conf.Redis)
	queue := redisQueue.NewRedisQ(client)
	cache := redisCache.NewRedisCache(client)

	// Object store
	objStore, err := storage.NewS3Store(logr)
	if err != nil {
		logr.Error("failed to init s3 store", logger.Any("error", err))
		os.Exit(1)
	}

	// Repository + Service
	repo := postgres.NewVideoRepo(dbPool)
	service := services.NewVideoService(objStore, repo, queue, cache, conf.Redis.RedisQueueName)

	// Setup router
	r := api.SetupRouter(service)
	logr.Info("starting server", logger.String("port", conf.Server.HttpPort))
	logr.Info("Application is Running in ", logger.String("env", conf.Env))
	if err := r.Run(":" + conf.Server.HttpPort); err != nil {
		logr.Error("server exited with error", logger.Any("error", err))
		os.Exit(1)
	}
}
