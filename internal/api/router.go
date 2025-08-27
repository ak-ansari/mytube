package api

import (
	"github.com/ak-ansari/mytube/internal/api/handlers"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

func SetupRouter(service *services.VideoService) *gin.Engine {
	r := gin.Default()
	vh := handlers.NewVideoHandler(service)

	r.POST("/videos/upload", vh.UploadVideo)

	return r
}
