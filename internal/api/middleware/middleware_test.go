package middleware_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/api/middleware"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/stream"
	"github.com/ak-ansari/mytube/internal/testutil/apptest"
	"github.com/ak-ansari/mytube/internal/testutil/authtest"
	"github.com/ak-ansari/mytube/internal/testutil/fakes"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func init() { gin.SetMode(gin.TestMode) }

// whoami echoes the current user id ("" when anonymous).
func whoami(c *gin.Context) {
	id := ""
	if uid := middleware.CurrentUserID(c); uid != nil {
		id = uid.String()
	}
	c.JSON(http.StatusOK, gin.H{"data": id, "success": true})
}

func doAuth(t *testing.T, r *gin.Engine, header string) (int, apptest.Envelope) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	rec := newRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code, decode(t, rec.Body.Bytes())
}

func newRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

func decode(t *testing.T, b []byte) apptest.Envelope {
	t.Helper()
	var env apptest.Envelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	return env
}

func TestAuthenticate(t *testing.T) {
	kc := authtest.New(t)
	users := fakes.NewUserRepo()
	r := gin.New()
	r.Use(middleware.Authenticate(kc.Verifier(t), services.NewUserService(users, fakes.NewObjectStore())))
	r.GET("/", whoami)

	sub := uuid.New()
	mod := func(f func(c jwt.MapClaims)) string {
		c := kc.Claims(sub.String())
		f(c)
		return kc.Sign(t, c)
	}
	tests := []struct {
		name     string
		header   string
		status   int
		wantUser string
		msg      string
	}{
		{"no header is anonymous", "", 200, "", ""},
		{"valid token", "Bearer " + kc.Token(t, sub.String()), 200, sub.String(), ""},
		{"scheme is case-insensitive", "bearer " + kc.Token(t, sub.String()), 200, sub.String(), ""},
		{"basic auth", "Basic dXNlcjpwYXNz", 401, "", "invalid Authorization header"},
		{"bearer without token", "Bearer", 401, "", "invalid Authorization header"},
		{"bearer empty token", "Bearer ", 401, "", "invalid or expired token"},
		{"expired", "Bearer " + mod(func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }), 401, "", "invalid or expired token"},
		{"wrong azp", "Bearer " + mod(func(c jwt.MapClaims) { c["azp"] = "other" }), 401, "", "invalid or expired token"},
		{"wrong iss", "Bearer " + mod(func(c jwt.MapClaims) { c["iss"] = "http://evil" }), 401, "", "invalid or expired token"},
		{"bad signature", "Bearer " + kc.SignRogue(t, kc.Claims(sub.String())), 401, "", "invalid or expired token"},
		{"sub not a uuid", "Bearer " + mod(func(c jwt.MapClaims) { c["sub"] = "service-account" }), 500, "", "failed to load user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, env := doAuth(t, r, tt.header)
			if status != tt.status {
				t.Fatalf("status = %d, want %d (%+v)", status, tt.status, env)
			}
			if tt.status == 200 {
				var got string
				apptest.DecodeData(t, env, &got)
				if got != tt.wantUser {
					t.Errorf("user = %q, want %q", got, tt.wantUser)
				}
				return
			}
			if env.Message != tt.msg || env.Success || env.Status != tt.status {
				t.Errorf("envelope = %+v", env)
			}
		})
	}
	if u, ok := users.User(sub); !ok || u.UserName != "demo" || u.ChannelName != "Demo Channel" || u.Email != "demo@example.com" {
		t.Errorf("synced user = %+v (found=%v)", u, ok)
	}

	t.Run("user sync failure", func(t *testing.T) {
		users.Errs.Set("Upsert", errors.New("db down"))
		defer users.Errs.Set("Upsert", nil)
		if status, _ := doAuth(t, r, "Bearer "+kc.Token(t, sub.String())); status != 500 {
			t.Fatalf("status = %d", status)
		}
	})
}

func TestRequireUser(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if c.GetHeader("X-User") != "" {
			c.Set("user", &models.User{ID: uuid.MustParse(c.GetHeader("X-User"))})
		}
		if c.GetHeader("X-Bogus") != "" {
			c.Set("user", "not a user")
		}
	})
	r.GET("/", middleware.RequireUser(), whoami)

	id := uuid.New()
	for _, tt := range []struct {
		name   string
		hdr    map[string]string
		status int
	}{
		{"anonymous", nil, 401},
		{"wrong type in context", map[string]string{"X-Bogus": "1"}, 401},
		{"user", map[string]string{"X-User": id.String()}, 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tt.hdr {
				req.Header.Set(k, v)
			}
			rec := newRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status = %d", rec.Code)
			}
			env := decode(t, rec.Body.Bytes())
			if tt.status == 401 && (env.Message != "login required" || env.Success || env.Status != 401) {
				t.Errorf("envelope = %+v", env)
			}
		})
	}
}

func TestRequireVideoOwner(t *testing.T) {
	users := fakes.NewUserRepo()
	catalog := fakes.NewCatalogRepo(users)
	signer, _ := stream.NewSigner(config.Stream{BaseUrl: "http://x", TokenSecret: "s"})
	svc := services.NewVideoCatalogService(catalog, fakes.NewCommentRepo(users), fakes.NewVideoIndex(), fakes.NewObjectStore(), fakes.NewCache(), signer)
	owner := uuid.New()
	vid := uuid.New()
	catalog.Put(&models.VideoDetails{VideoCard: models.VideoCard{ID: vid, Channel: models.ChannelSummary{ID: owner}}})

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{ID: uuid.MustParse(c.GetHeader("X-User"))})
	})
	r.POST("/videos/:id", middleware.RequireVideoOwner(svc), whoami)

	tests := []struct {
		name   string
		path   string
		user   uuid.UUID
		status int
		msg    string
	}{
		{"owner", "/videos/" + vid.String(), owner, 200, ""},
		{"not owner", "/videos/" + vid.String(), uuid.New(), 403, "you do not own this video"},
		{"missing video", "/videos/" + uuid.NewString(), owner, 404, "video not found"},
		{"bad id", "/videos/123", owner, 400, "invalid video id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, tt.path, nil)
			req.Header.Set("X-User", tt.user.String())
			rec := newRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if tt.msg != "" {
				if env := decode(t, rec.Body.Bytes()); env.Message != tt.msg || env.Status != tt.status || env.Success {
					t.Errorf("envelope = %+v", env)
				}
			}
		})
	}

	t.Run("repo error is forbidden", func(t *testing.T) {
		catalog.Errs.Set("GetOwnerID", errors.New("db down"))
		defer catalog.Errs.Set("GetOwnerID", nil)
		req, _ := http.NewRequest(http.MethodPost, "/videos/"+vid.String(), nil)
		req.Header.Set("X-User", owner.String())
		rec := newRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 403 {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestCurrentUserHelpers(t *testing.T) {
	c, _ := gin.CreateTestContext(newRecorder())
	if middleware.CurrentUser(c) != nil || middleware.CurrentUserID(c) != nil {
		t.Fatal("anonymous context should have no user")
	}
	c.Set("user", models.User{ID: uuid.New()}) // value, not pointer
	if middleware.CurrentUser(c) != nil {
		t.Fatal("non-pointer user must be ignored")
	}
	u := &models.User{ID: uuid.New()}
	c.Set("user", u)
	if middleware.CurrentUser(c) != u || *middleware.CurrentUserID(c) != u.ID {
		t.Fatal("CurrentUser/CurrentUserID mismatch")
	}
}

func TestLegacyAuthMiddlewareSetsStubUser(t *testing.T) {
	r := gin.New()
	r.GET("/", middleware.AuthMiddleware(), whoami)
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	rec := newRecorder()
	r.ServeHTTP(rec, req)
	var got string
	apptest.DecodeData(t, decode(t, rec.Body.Bytes()), &got)
	if got != "49672439-457b-44af-9e9a-7760d4dee9e3" {
		t.Fatalf("stub user = %q", got)
	}
}
