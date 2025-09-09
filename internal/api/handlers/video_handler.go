package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/util"
	"github.com/gin-gonic/gin"
)

type VideoHandler struct {
	service *services.VideoService
}

func NewVideoHandler(service *services.VideoService) *VideoHandler {
	vh := &VideoHandler{
		service: service,
	}
	return vh
}
func (vh *VideoHandler) UploadVideo(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := vh.service.UploadVideo(ctx, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, util.NewResponse(201, "file uploaded successfully", result, nil))

}
func (vh *VideoHandler) GetVideo(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := vh.service.GetVideo(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, util.NewResponse(201, "get video successfully", result, nil))

}
func (vh *VideoHandler) GetDownloadUrl(c *gin.Context) {
	key := c.Query("key")
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := vh.service.GetDownloadUrl(ctx, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, util.NewResponse(201, "get video successfully", result, nil))

}
