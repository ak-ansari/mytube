package handlers

import (
	"context"
	"time"

	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

type VideoMetadataHandler struct {
	vs *services.VideoService
}

func NewVideoMetadataHandler(vs *services.VideoService) *VideoMetadataHandler {
	return &VideoMetadataHandler{vs: vs}
}
func (vm *VideoMetadataHandler) GetVideoMeta(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := vm.vs.GetMeta(ctx, id)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, "get video metadata successfully", result)
}
