package handlers

import (
	"context"
	"fmt"
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
		c.JSON(http.StatusBadRequest, util.NewResponse(http.StatusBadRequest, "Bad request", nil, err))
		return
	}
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, util.NewResponse(http.StatusUnauthorized, "Unauthorized", nil, nil))
	}
	u, ok := user.(*models.User)
	if !ok {
		c.JSON(http.StatusUnauthorized, util.NewResponse(http.StatusUnauthorized, "Unauthorized", nil, nil))
		return
	}
	result, err := vh.service.UploadPreSign(ctx, &vd, u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusCreated, util.NewResponse(201, "file uploaded successfully", result, nil))

}
func (vh *VideoHandler) ConfirmVideo(c *gin.Context) {
	var videoConfirmDto dto.VideoConfirmDto
	id := c.Param("id")
	if err := c.ShouldBindBodyWithJSON(&videoConfirmDto); err != nil {

		fmt.Println(err)
		c.JSON(http.StatusBadRequest, util.NewResponse(http.StatusBadRequest, "Bad request", nil, err))
		return
	}
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	vm, err := vh.service.ConfirmVideo(ctx, id, videoConfirmDto)
	if err != nil {

		fmt.Println(err)
		c.JSON(http.StatusInternalServerError, util.NewResponse(http.StatusBadRequest, "Bad request", nil, err))
		return
	}
	c.JSON(http.StatusOK, util.NewResponse(http.StatusOK, "Video is successfully saved", vm, nil))
}
func (vh *VideoHandler) GetVideo(c *gin.Context) {
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := vh.service.GetVideoKey(ctx, id)
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

	c.JSON(http.StatusOK, util.NewResponse(201, "get video download url successfully", result, nil))

}
func (vh *VideoHandler) SearchVideo(c *gin.Context) {
	query := c.Query("search")
	size := c.Query("size")
	page := c.Query("page")
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := vh.service.SearchVideo(ctx, query, page, size)
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, util.NewResponse(201, "videos retrieved successfully", result, nil))

}
func (vh *VideoHandler) SuggestVideo(c *gin.Context) {
	query := c.Query("search")
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := vh.service.SuggestVideo(ctx, query)
	if err != nil {
		fmt.Println(err)
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, util.NewResponse(201, "search suggestion fetched successfully", result, nil))

}
