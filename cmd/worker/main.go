package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	redisCache "github.com/ak-ansari/mytube/internal/cache/redis"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/db"
	"github.com/ak-ansari/mytube/internal/media"
	"github.com/ak-ansari/mytube/internal/pkg/logger"
	client "github.com/ak-ansari/mytube/internal/pkg/redis"
	redisQueue "github.com/ak-ansari/mytube/internal/queue/redis"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/workers"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conf, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	// --- Initialize logger ---
	log, err := logger.NewZapLogger(conf.Env)
	if err != nil {
		panic(err)
	}
	defer log.Flush()

	// --- Database ---
	pool, err := db.NewPool(conf, log)
	if err != nil {
		log.Fatal("Failed to init db pool", logger.Error(err))
	}

	// --- Storage ---
	store, err := storage.NewS3Store(log)
	if err != nil {
		log.Fatal("Failed to init object store", logger.Error(err))
	}

	// --- Redis + Queue + Cache ---
	redisClient := client.NewRedisClient(&conf.Redis)
	queue := redisQueue.NewRedisQ(redisClient)
	cache := redisCache.NewRedisCache(redisClient)

	// --- Media + Services ---
	ffm := media.NewFFM()
	repo := postgres.NewVideoRepo(pool)
	service := services.NewVideoService(store, repo, queue, cache, conf.Redis.RedisQueueName)

	// --- Workers ---
	validate := workers.NewValidate(service, store, ffm, log)
	transcode := workers.NewTranscoder(service, store, ffm, log)
	segment := workers.NewSegment(service, store, ffm, log)
	checksum := workers.NewChecksum(log)
	publish := workers.NewPublish(service, log)
	thumbnail := workers.NewThumbnail(service, ffm, store, log)

	runner := workers.NewRunner(
		queue,
		conf.Redis.RedisQueueName,
		validate,
		transcode,
		segment,
		checksum,
		publish,
		thumbnail,
		log,
	)

	// --- Start worker runner ---
	go func() {
		runner.Start(ctx)
	}()
	log.Info("Application is Running in ", logger.String("env", conf.Env))

	// --- Graceful shutdown ---
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	cancel()
	log.Info("Shutting down gracefully...")
}
