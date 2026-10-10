package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	objectStore storage.ObjectStore
}

func NewFileHandler(objectStore storage.ObjectStore) *FileHandler {
	return &FileHandler{objectStore: objectStore}
}
func (fh *FileHandler) GetUrl(c *gin.Context) {
	key := c.Query("key")
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	result, err := fh.objectStore.GetUrl(ctx, key)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, "get video download url successfully", result)
}
func (fh *FileHandler) GerPreSignedPutUrl(c *gin.Context) {
	key := c.Query("key")
	size := c.Query("size")
	s, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		badRequest(c, "Size is not a valid number")
		return
	}
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	url, err := fh.objectStore.GerPreSignedPutUrl(ctx, key, s)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, "get video download url successfully", map[string]string{"key": key, "url": url})
}
