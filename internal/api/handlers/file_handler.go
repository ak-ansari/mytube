package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/ak-ansari/mytube/internal/api/middleware"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	files *services.FileService
}

func NewFileHandler(files *services.FileService) *FileHandler {
	return &FileHandler{files: files}
}

// GetUrl signs a GET for a thumbnail or avatar image.
func (fh *FileHandler) GetUrl(c *gin.Context) {
	key := c.Query("key")
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := fh.files.DownloadURL(ctx, key)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, "get video download url successfully", result)
}

// GerPreSignedPutUrl signs a PUT for the caller's avatar or a thumbnail of the caller's video.
func (fh *FileHandler) GerPreSignedPutUrl(c *gin.Context) {
	key := c.Query("key")
	size := c.Query("size")
	s, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		badRequest(c, "Size is not a valid number")
		return
	}
	user := middleware.CurrentUser(c)
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	url, err := fh.files.PresignImageUpload(ctx, user.ID, key, s)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, "get video download url successfully", map[string]string{"key": key, "url": url})
}
