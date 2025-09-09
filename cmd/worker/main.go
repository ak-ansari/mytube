package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	redisCache "github.com/ak-ansari/mytube/internal/cache/redis"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/db"
	"github.com/ak-ansari/mytube/internal/media"
	client "github.com/ak-ansari/mytube/internal/pkg/redis"
	redisQueue "github.com/ak-ansari/mytube/internal/queue/redis"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/workers"
)

// TODO implement telemetry (logs + metrics + traces)
// TODO unit testing

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, err := config.GetConfig()
	if err != nil {
		log.Fatal(err)
	}
	pool, err := db.NewPool(conf)
	if err != nil {
		log.Fatal(err)
	}
	repo := postgres.NewVideoRepo(pool)
	store, err := storage.NewS3Store()
	if err != nil {
		log.Fatal(err)
	}
	client := client.NewRedisClient(&conf.Redis)
	queue := redisQueue.NewRedisQ(client)
	cache := redisCache.NewRedisCache(client)
	ffm := media.NewFFM()
	service := services.NewVideoService(store, repo, queue, cache, conf.Redis.RedisQueueName)
	validate := workers.NewValidate(service, store, ffm)
	transcode := workers.NewTranscoder(service, store, ffm)
	segment := workers.NewSegment(service, store, ffm)
	checksum := workers.NewChecksum()
	publish := workers.NewPublish(service)
	thumbnail := workers.NewThumbnail(service, ffm, store)
	runner := workers.NewRunner(
		queue,
		conf.Redis.RedisQueueName,
		validate,
		transcode,
		segment,
		checksum,
		publish,
		thumbnail,
	)
	go func() {
		runner.Start(ctx)
	}()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	cancel()
}
