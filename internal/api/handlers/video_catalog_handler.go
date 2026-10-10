package handlers

import (
	"github.com/ak-ansari/mytube/internal/api/middleware"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

type VideoCatalogHandler struct {
	catalog *services.VideoCatalogService
}

func NewVideoCatalogHandler(catalog *services.VideoCatalogService) *VideoCatalogHandler {
	return &VideoCatalogHandler{catalog: catalog}
}

// Browse: GET /videos/browse?search=&page=&size= — home feed, or search results when search is set.
func (h *VideoCatalogHandler) Browse(c *gin.Context) {
	page, err := h.catalog.Browse(c, c.Query("search"), paging(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "videos fetched successfully", page)
}

func (h *VideoCatalogHandler) Mine(c *gin.Context) {
	page, err := h.catalog.MyVideos(c, middleware.CurrentUser(c).ID, paging(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "videos fetched successfully", page)
}

func (h *VideoCatalogHandler) Liked(c *gin.Context) {
	page, err := h.catalog.LikedVideos(c, middleware.CurrentUser(c).ID, paging(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "videos fetched successfully", page)
}

func (h *VideoCatalogHandler) SubscriptionFeed(c *gin.Context) {
	page, err := h.catalog.SubscriptionFeed(c, middleware.CurrentUser(c).ID, paging(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "videos fetched successfully", page)
}

func (h *VideoCatalogHandler) Details(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	d, err := h.catalog.GetDetails(c, id, middleware.CurrentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "video fetched successfully", d)
}

func (h *VideoCatalogHandler) Related(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	page, err := h.catalog.Related(c, id, paging(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "videos fetched successfully", page)
}

func (h *VideoCatalogHandler) Update(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	var in services.UpdateVideoInput
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid request body")
		return
	}
	d, err := h.catalog.Update(c, id, middleware.CurrentUser(c).ID, in)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "video updated successfully", d)
}

func (h *VideoCatalogHandler) Delete(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	if err := h.catalog.Delete(c, id, middleware.CurrentUser(c).ID); err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "video deleted successfully", nil)
}

func (h *VideoCatalogHandler) RecordView(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	viewer := c.ClientIP()
	if uid := middleware.CurrentUserID(c); uid != nil {
		viewer = uid.String()
	}
	if err := h.catalog.RecordView(c, id, viewer); err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "view recorded", nil)
}

type reactionBody struct {
	Value *int `json:"value"`
}

func (h *VideoCatalogHandler) React(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	var body reactionBody
	if err := c.ShouldBindJSON(&body); err != nil || body.Value == nil {
		badRequest(c, "value is required (1, -1 or 0)")
		return
	}
	d, err := h.catalog.React(c, id, middleware.CurrentUser(c).ID, *body.Value)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "reaction saved", d)
}

func (h *VideoCatalogHandler) ListComments(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	page, err := h.catalog.ListComments(c, id, middleware.CurrentUserID(c), paging(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "comments fetched successfully", page)
}

type commentBody struct {
	Content string `json:"content"`
}

func (h *VideoCatalogHandler) AddComment(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	var body commentBody
	if err := c.ShouldBindJSON(&body); err != nil {
		badRequest(c, "invalid request body")
		return
	}
	comment, err := h.catalog.AddComment(c, id, middleware.CurrentUser(c).ID, body.Content)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "comment added", comment)
}

func (h *VideoCatalogHandler) DeleteComment(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	commentID, ok := uuidParam(c, "commentId")
	if !ok {
		return
	}
	if err := h.catalog.DeleteComment(c, id, commentID, middleware.CurrentUser(c).ID); err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "comment deleted", nil)
}
