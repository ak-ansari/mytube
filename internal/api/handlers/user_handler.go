package handlers

import (
	"github.com/ak-ansari/mytube/internal/api/middleware"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	users   *services.UserService
	catalog *services.VideoCatalogService
}

func NewUserHandler(users *services.UserService, catalog *services.VideoCatalogService) *UserHandler {
	return &UserHandler{users: users, catalog: catalog}
}

func (h *UserHandler) GetMe(c *gin.Context) {
	p, err := h.users.GetProfile(c, middleware.CurrentUser(c).ID)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "profile fetched successfully", p)
}

func (h *UserHandler) UpdateMe(c *gin.Context) {
	var in services.UpdateProfileInput
	if err := c.ShouldBindJSON(&in); err != nil {
		badRequest(c, "invalid request body")
		return
	}
	p, err := h.users.UpdateProfile(c, middleware.CurrentUser(c).ID, in)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "profile updated successfully", p)
}

func (h *UserHandler) MySubscriptions(c *gin.Context) {
	channels, err := h.users.ListSubscriptions(c, middleware.CurrentUser(c).ID)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "subscriptions fetched successfully", channels)
}

func (h *UserHandler) GetChannel(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	ch, err := h.users.GetChannel(c, id, middleware.CurrentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "channel fetched successfully", ch)
}

func (h *UserHandler) ChannelVideos(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	page, err := h.catalog.ChannelVideos(c, id, paging(c))
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "channel videos fetched successfully", page)
}

func (h *UserHandler) Subscribe(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	ch, err := h.users.Subscribe(c, middleware.CurrentUser(c).ID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "subscribed", ch)
}

func (h *UserHandler) Unsubscribe(c *gin.Context) {
	id, ok := uuidParam(c, "id")
	if !ok {
		return
	}
	ch, err := h.users.Unsubscribe(c, middleware.CurrentUser(c).ID, id)
	if err != nil {
		respondError(c, err)
		return
	}
	respondOK(c, "unsubscribed", ch)
}
