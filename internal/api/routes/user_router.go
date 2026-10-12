package routes

import (
	"github.com/ak-ansari/mytube/internal/api/handlers"
	"github.com/ak-ansari/mytube/internal/api/middleware"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

func SetupUserRoutes(r *gin.RouterGroup, users *services.UserService, catalog *services.VideoCatalogService) {
	h := handlers.NewUserHandler(users, catalog)
	requireUser := middleware.RequireUser()
	me := r.Group("/users/me", requireUser)
	{
		me.GET("", h.GetMe)
		me.PUT("", h.UpdateMe)
		me.GET("/subscriptions", h.MySubscriptions)
	}
	ch := r.Group("/channels")
	{
		ch.GET("/:id", h.GetChannel)
		ch.GET("/:id/videos", h.ChannelVideos)
		ch.POST("/:id/subscribe", requireUser, h.Subscribe)
		ch.DELETE("/:id/subscribe", requireUser, h.Unsubscribe)
	}
}
