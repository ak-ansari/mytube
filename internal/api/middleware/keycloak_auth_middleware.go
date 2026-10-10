package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/ak-ansari/mytube/internal/auth"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/util"
	"github.com/ak-ansari/mytube/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Authenticate validates an optional "Authorization: Bearer <keycloak access token>" header.
// With a valid token the user is synced to Postgres and stored under "user"; without a header the
// request continues anonymously; an invalid token is rejected so the client knows to refresh/login.
func Authenticate(verifier *auth.KeycloakVerifier, users *services.UserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.Next()
			return
		}
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, util.NewResponse(http.StatusUnauthorized, "invalid Authorization header", nil, nil))
			return
		}
		claims, err := verifier.Verify(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, util.NewResponse(http.StatusUnauthorized, "invalid or expired token", nil, nil))
			return
		}
		user, err := users.SyncFromToken(c, claims)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, util.NewResponse(http.StatusInternalServerError, "failed to load user", nil, nil))
			return
		}
		c.Set("user", user)
		c.Next()
	}
}

// RequireUser rejects anonymous requests. Use after Authenticate.
func RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if CurrentUser(c) == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, util.NewResponse(http.StatusUnauthorized, "login required", nil, nil))
			return
		}
		c.Next()
	}
}

// RequireVideoOwner allows the request only for the uploader of the video in the ":id" param. Use after RequireUser.
func RequireVideoOwner(catalog *services.VideoCatalogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, util.NewResponse(http.StatusBadRequest, "invalid video id", nil, nil))
			return
		}
		if err := catalog.AssertOwner(c, id, CurrentUser(c).ID); err != nil {
			status, msg := http.StatusForbidden, "you do not own this video"
			if errors.Is(err, services.ErrNotFound) {
				status, msg = http.StatusNotFound, "video not found"
			}
			c.AbortWithStatusJSON(status, util.NewResponse(status, msg, nil, nil))
			return
		}
		c.Next()
	}
}

// CurrentUser returns the authenticated user, or nil for anonymous requests.
func CurrentUser(c *gin.Context) *models.User {
	if u, ok := c.Get("user"); ok {
		if user, ok := u.(*models.User); ok {
			return user
		}
	}
	return nil
}

// CurrentUserID returns the authenticated user's id, or nil for anonymous requests.
func CurrentUserID(c *gin.Context) *uuid.UUID {
	if u := CurrentUser(c); u != nil {
		return &u.ID
	}
	return nil
}
