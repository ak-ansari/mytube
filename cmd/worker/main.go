package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	redisCache "github.com/ak-ansari/mytube/internal/cache/redis"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/db"
	elasticsearch_index "github.com/ak-ansari/mytube/internal/index/elasticsearch"
	"github.com/ak-ansari/mytube/internal/media"
	redisQueue "github.com/ak-ansari/mytube/internal/queue/redis"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/workers"
	"github.com/ak-ansari/mytube/pkg/elastic"
	"github.com/ak-ansari/mytube/pkg/logger"
	client "github.com/ak-ansari/mytube/pkg/redis"
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
	esClient, err := elastic.NewEsClient(conf)
	if err != nil {
		log.Fatal("failed to connect with elastic search client", logger.Error(err))
	}
	videoIndex, err := elasticsearch_index.NewVideoEsIndex(esClient)
	if err != nil {
		log.Fatal("failed to init video index", logger.Error(err))
	}
	// --- Media + Services ---
	ffm := media.NewFFM()
	videoMetadataRepo := postgres.NewVideoMetadataRepo(pool)
	videoRepo := postgres.NewVideoRepo(pool)
	stateMachine := services.NewVideoStateMachine()
	pipelineCoordinator := services.NewPipelineCoordinator(videoRepo, stateMachine, queue, conf.Redis.RedisQueueName)
	service := services.NewVideoService(videoIndex, store, videoMetadataRepo, videoRepo, queue, cache, conf.Redis.RedisQueueName, stateMachine, pipelineCoordinator)

	// --- Workers ---
	validate := workers.NewValidate(service, store, ffm, log)
	transcode := workers.NewTranscoder(service, store, ffm, log)
	segment := workers.NewSegment(service, store, ffm, log)
	checksum := workers.NewChecksum(log)
	publish := workers.NewPublish(service, log)
	thumbnail := workers.NewThumbnail(service, ffm, store, log)
	handlerRegistry := workers.NewHandlerRegistry(validate, thumbnail, transcode, segment, publish, checksum)
	runner := workers.NewRunner(queue, cache, conf.Redis.RedisQueueName, conf.S3.MinioRedisQueueName, log, handlerRegistry, pipelineCoordinator, service)

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
