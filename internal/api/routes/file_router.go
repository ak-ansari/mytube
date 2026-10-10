package routes

import (
	"github.com/ak-ansari/mytube/internal/api/handlers"
	"github.com/ak-ansari/mytube/internal/api/middleware"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

func SetupFileRoutes(r *gin.RouterGroup, s *services.FileService) {
	fh := handlers.NewFileHandler(s)
	vr := r.Group("/files")
	{
		vr.GET("/presign", middleware.RequireUser(), fh.GerPreSignedPutUrl)
		vr.GET("/url", fh.GetUrl)
	}
}
