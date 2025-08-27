package main

import (
	"log"

	"github.com/ak-ansari/mytube/internal/api"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/db"
	"github.com/ak-ansari/mytube/internal/queue/redis"
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
	queue := redis.NewRedisQ(conf)

	objStore, err := storage.NewS3Store()
	if err != nil {
		log.Fatal(err)
	}
	repo := postgres.NewVideoRepo(dbPool)
	service := services.NewVideoService(objStore, repo, queue, conf)
	r := api.SetupRouter(service)
	r.Run(":" + conf.Server.HttpPort)
}
