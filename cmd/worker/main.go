package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	redisCache "github.com/ak-ansari/mytube/internal/cache/redis"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/db"
	elasticsearch_index "github.com/ak-ansari/mytube/internal/index/elasticsearch"
	"github.com/ak-ansari/mytube/internal/media"
	"github.com/ak-ansari/mytube/internal/observability"
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
	log, err := logger.NewZapLoggerWithFile(conf.Env, "worker", logFile(conf.Observability.LogDir, "worker"))
	if err != nil {
		panic(err)
	}
	defer log.Flush()

	// --- Database ---
	pool, err := db.NewPool(conf, log)
	if err != nil {
		log.Fatal("Failed to init db pool", logger.Error(err))
	}
	observability.RegisterDBCollector(pool, "worker", true)
	if conf.Observability.MetricsAddr != "" {
		observability.ServeMetrics(ctx, conf.Observability.MetricsAddr, log)
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
	handlerRegistry := workers.NewHandlerRegistry(validate, thumbnail, transcode, segment, publish, checksum).Instrument()
	deadLetterRepo := postgres.NewDeadLetterRepo(pool)
	retryPolicy := workers.RetryPolicyFromConfig(conf.Worker)
	runner := workers.NewRunner(queue, cache, conf.Redis.RedisQueueName, conf.S3.MinioRedisQueueName, log, handlerRegistry, pipelineCoordinator, service).
		WithRetry(queue, deadLetterRepo, retryPolicy)
	log.Info("Retry policy",
		logger.Int("maxAttempts", retryPolicy.MaxAttempts),
		logger.String("baseDelay", retryPolicy.BaseDelay.String()),
		logger.String("maxDelay", retryPolicy.MaxDelay.String()))

	// --- Start worker runner ---
	runner.Start(ctx)
	log.Info("Application is Running in ", logger.String("env", conf.Env))

	// --- Graceful shutdown ---
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Info("Shutting down gracefully...")
	cancel()
	// let workers finish their current iteration before exiting
	runner.Wait()
	log.Info("Workers stopped")
}

// logFile is <dir>/<name>.log, or "" (no file sink) when dir is unset.
func logFile(dir, name string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, name+".log")
}
