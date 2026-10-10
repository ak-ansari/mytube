package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxPageSize = 50

// maxResultWindow is Elasticsearch's default index.max_result_window (from + size).
const maxResultWindow = 10000

func respondOK(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, util.NewResponse(http.StatusOK, message, data, nil))
}

// respondError maps service errors to HTTP statuses; the message is what the UI shows.
func respondError(c *gin.Context, err error) {
	status, msg := http.StatusInternalServerError, "something went wrong"
	switch {
	case errors.Is(err, services.ErrNotFound):
		status, msg = http.StatusNotFound, "not found"
	case errors.Is(err, services.ErrForbidden):
		status, msg = http.StatusForbidden, "you are not allowed to do that"
	case errors.Is(err, services.ErrBadRequest):
		status, msg = http.StatusBadRequest, strings.TrimPrefix(err.Error(), services.ErrBadRequest.Error()+": ")
	default:
		log.Printf("%s %s: %v", c.Request.Method, c.FullPath(), err)
	}
	c.JSON(status, util.NewResponse(status, msg, nil, nil))
}

func badRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, util.NewResponse(http.StatusBadRequest, msg, nil, nil))
}

func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		badRequest(c, "invalid "+name)
		return uuid.Nil, false
	}
	return id, true
}

func paging(c *gin.Context) services.Paging {
	size := util.ParseIntWithDefault(c.Query("size"), 20)
	if size > maxPageSize {
		size = maxPageSize
	}
	page := util.ParseIntWithDefault(c.Query("page"), 1)
	// services fetch size+1 rows from (page-1)*size, so keep page*size+1 inside the window
	if maxPage := (maxResultWindow - 1) / size; page > maxPage {
		page = maxPage
	}
	return services.Paging{Page: page, Size: size}
}
