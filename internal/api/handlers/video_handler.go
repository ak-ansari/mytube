package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/ak-ansari/mytube/internal/api/dto"
	"github.com/ak-ansari/mytube/internal/models"
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

// func (vh *VideoHandler) UploadVideo(c *gin.Context) {
// 	file, err := c.FormFile("file")
// 	if err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
// 		return
// 	}
// 	ctx, cancel := context.WithTimeout(c, 120*time.Second)
// 	defer cancel()
// 	result, err := vh.service.UploadVideo(ctx, file)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, err.Error())
// 		return
// 	}

// 	c.JSON(http.StatusCreated, util.NewResponse(201, "file uploaded successfully", result, nil))

// }
func (vh *VideoHandler) UploadPreSign(c *gin.Context) {
	var vd dto.UploadVideoDto
	if err := c.ShouldBindBodyWithJSON(&vd); err != nil {
		badRequest(c, "file_name (with extension) and a size between 1 byte and 10 GiB are required")
		return
	}
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, util.NewResponse(http.StatusUnauthorized, "Unauthorized", nil, nil))
		return
	}
	u, ok := user.(*models.User)
	if !ok {
		c.JSON(http.StatusUnauthorized, util.NewResponse(http.StatusUnauthorized, "Unauthorized", nil, nil))
		return
	}
	result, err := vh.service.UploadPreSign(ctx, &vd, u)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, util.NewResponse(201, "file uploaded successfully", result, nil))

}
func (vh *VideoHandler) ConfirmVideo(c *gin.Context) {
	var videoConfirmDto dto.VideoConfirmDto
	id := c.Param("id")
	if err := c.ShouldBindBodyWithJSON(&videoConfirmDto); err != nil {
		badRequest(c, "title (3-100 characters), visibility (public, private or unlisted) and a description of at most 5000 characters are required")
		return
	}
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	vm, err := vh.service.ConfirmVideo(ctx, id, videoConfirmDto)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "Video is successfully saved", vm)
}
func (vh *VideoHandler) GetVideo(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := vh.service.GetVideoKey(ctx, id)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, "get video successfully", result)

}

// GetDownloadUrl signs a GET for a thumbnail or avatar image; video files are not handed out here.
func (vh *VideoHandler) GetDownloadUrl(c *gin.Context) {
	key := c.Query("key")
	if !services.IsPublicImageKey(key) {
		badRequest(c, "only thumbnail and avatar images can be fetched")
		return
	}
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := vh.service.GetDownloadUrl(ctx, key)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, "get video download url successfully", result)

}
func (vh *VideoHandler) SearchVideo(c *gin.Context) {
	query := c.Query("search")
	size := c.Query("size")
	page := c.Query("page")
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := vh.service.SearchVideo(ctx, query, page, size)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, "videos retrieved successfully", result)

}
func (vh *VideoHandler) SuggestVideo(c *gin.Context) {
	query := c.Query("search")
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := vh.service.SuggestVideo(ctx, query)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, "search suggestion fetched successfully", result)

}
