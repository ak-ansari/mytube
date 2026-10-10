package api

import (
	"time"

	"github.com/ak-ansari/mytube/internal/api/middleware"
	"github.com/ak-ansari/mytube/internal/api/routes"
	"github.com/ak-ansari/mytube/internal/auth"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRouter(videoService *services.VideoService, objectStore storage.ObjectStore, verifier *auth.KeycloakVerifier, userService *services.UserService, catalogService *services.VideoCatalogService) *gin.Engine {
	r := gin.Default()
	// no proxy is trusted, so ClientIP() is the socket peer and X-Forwarded-For can't be spoofed
	if err := r.SetTrustedProxies(nil); err != nil {
		panic(err)
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"}, // frontend URLs
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	// reads are public; write routes add middleware.RequireUser()
	api := r.Group("/api", middleware.Authenticate(verifier, userService))
	routes.SetupVideoRoutes(api, videoService, catalogService)
	routes.SetupFileRoutes(api, objectStore)
	routes.SetupVideoMetadataRoutes(api, videoService)
	routes.SetupVideoCatalogRoutes(api, catalogService)
	routes.SetupUserRoutes(api, userService, catalogService)
	return r
}
