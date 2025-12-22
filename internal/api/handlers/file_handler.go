package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/util"
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
		c.JSON(http.StatusInternalServerError, util.NewResponse(http.StatusInternalServerError, "Error while getting the file url", nil, err))
		return
	}

	c.JSON(http.StatusOK, util.NewResponse(201, "get video download url successfully", result, nil))
}
func (fh *FileHandler) GerPreSignedPutUrl(c *gin.Context) {
	key := c.Query("key")
	size := c.Query("size")
	s, err := strconv.ParseInt(size, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.New("Size is not a valid number"))
		return
	}
	ctx, cancel := context.WithTimeout(c, 120*time.Second)
	defer cancel()
	url, err := fh.objectStore.GerPreSignedPutUrl(ctx, key, s)
	if err != nil {
		c.JSON(http.StatusInternalServerError, util.NewResponse(http.StatusBadRequest, "Size is not a valid number", nil, err))
		return
	}

	c.JSON(http.StatusOK, util.NewResponse(201, "get video download url successfully", map[string]string{"key": key, "url": url}, nil))
}
