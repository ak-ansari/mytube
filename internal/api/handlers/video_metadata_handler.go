package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/util"
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
		c.JSON(http.StatusInternalServerError, util.NewResponse(http.StatusInternalServerError, "Error while getting the video metadata", nil, err))
		return
	}

	c.JSON(http.StatusOK, util.NewResponse(201, "get video metadata successfully", result, nil))
}
