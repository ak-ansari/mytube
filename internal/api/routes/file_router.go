package routes

import (
	"github.com/ak-ansari/mytube/internal/api/handlers"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/gin-gonic/gin"
)

func SetupFileRoutes(r *gin.RouterGroup, s storage.ObjectStore) {
	fh := handlers.NewFileHandler(s)
	vr := r.Group("/files")
	{
		vr.GET("/presign", fh.GerPreSignedPutUrl)
		vr.GET("/url", fh.GetUrl)
	}
}
