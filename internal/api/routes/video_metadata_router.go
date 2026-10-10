package routes

import (
	"github.com/ak-ansari/mytube/internal/api/handlers"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

func SetupVideoMetadataRoutes(r *gin.RouterGroup, s *services.VideoService) {
	vh := handlers.NewVideoMetadataHandler(s)
	vr := r.Group("/video-meta")
	{
		vr.GET("/:id", vh.GetVideoMeta)
	}
}
