package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/db"
	"github.com/ak-ansari/mytube/internal/media"
	"github.com/ak-ansari/mytube/internal/queue/redis"
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
	queue := redis.NewRedisQ(conf)
	ffm := media.NewFFM()
	service := services.NewVideoService(store, repo, queue, conf)
	validate := workers.NewValidate(service, store, ffm)
	transcode := workers.NewTranscoder(service, store, queue, ffm)
	segment := workers.NewSegment(service, store, ffm)
	checksum := workers.NewChecksum()
	publish := workers.NewPublish()
	thumbnail := workers.NewThumbnail()
	runner := workers.NewRunner(
		queue,
		conf.Queue.RedisQueueName,
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
