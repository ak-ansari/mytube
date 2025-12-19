package api

import (
	"time"

	"github.com/ak-ansari/mytube/internal/api/middleware"
	"github.com/ak-ansari/mytube/internal/api/routes"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter(videoService *services.VideoService) *gin.Engine {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "https://myfrontend.com"}, // frontend URLs
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	protected := r.Group("/api", middleware.AuthMiddleware())
	routes.SetupVideoRoutes(protected, videoService)
	return r
}
