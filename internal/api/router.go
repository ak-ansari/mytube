package api

import (
	"github.com/ak-ansari/mytube/internal/api/middleware"
	"github.com/ak-ansari/mytube/internal/api/routes"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

func SetupRouter(videoService *services.VideoService) *gin.Engine {
	r := gin.Default()
	protected := r.Group("", middleware.AuthMiddleware())
	routes.SetupVideoRoutes(protected, videoService)
	return r
}
