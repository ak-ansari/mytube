package routes

import (
	"github.com/ak-ansari/mytube/internal/api/handlers"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

func SetupVideoRoutes(r *gin.RouterGroup, s *services.VideoService) {
	vh := handlers.NewVideoHandler(s)
	vr := r.Group("/videos")
	{
		// vr.POST("/videos/upload", vh.UploadVideo)
		vr.GET("/:id", vh.GetVideo)
		vr.POST("/upload/presign", vh.UploadPreSign)
		vr.GET("/url", vh.GetDownloadUrl)
	}
}
