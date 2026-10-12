package routes

import (
	"github.com/ak-ansari/mytube/internal/api/handlers"
	"github.com/ak-ansari/mytube/internal/api/middleware"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

func SetupVideoCatalogRoutes(r *gin.RouterGroup, catalog *services.VideoCatalogService) {
	h := handlers.NewVideoCatalogHandler(catalog)
	requireUser := middleware.RequireUser()
	vr := r.Group("/videos")
	{
		vr.GET("/browse", h.Browse)
		vr.GET("/mine", requireUser, h.Mine)
		vr.GET("/liked", requireUser, h.Liked)
		vr.GET("/subscriptions", requireUser, h.SubscriptionFeed)
		vr.GET("/:id/details", h.Details)
		vr.GET("/:id/related", h.Related)
		vr.PUT("/:id", requireUser, h.Update)
		vr.DELETE("/:id", requireUser, h.Delete)
		vr.POST("/:id/view", h.RecordView)
		vr.PUT("/:id/reaction", requireUser, h.React)
		vr.GET("/:id/comments", h.ListComments)
		vr.POST("/:id/comments", requireUser, h.AddComment)
		vr.DELETE("/:id/comments/:commentId", requireUser, h.DeleteComment)
	}
}
