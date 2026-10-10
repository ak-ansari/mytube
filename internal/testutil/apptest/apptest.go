// Package apptest wires the real services and gin router over in-memory fakes for HTTP-level unit tests.
package apptest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/api"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/stream"
	"github.com/ak-ansari/mytube/internal/testutil/authtest"
	"github.com/ak-ansari/mytube/internal/testutil/fakes"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const QueueName = "jobs"

// Env holds every fake plus the services and router built on top of them.
type Env struct {
	Users      *fakes.UserRepo
	Catalog    *fakes.CatalogRepo
	Comments   *fakes.CommentRepo
	Videos     *fakes.VideoRepo
	Metas      *fakes.VideoMetadataRepo
	Index      *fakes.VideoIndex
	Store      *fakes.ObjectStore
	Cache      *fakes.Cache
	Queue      *fakes.Queue
	Signer     *stream.Signer
	Keycloak   *authtest.Keycloak
	UserSvc    *services.UserService
	CatalogSvc *services.VideoCatalogService
	VideoSvc   *services.VideoService
	Router     *gin.Engine
}

// New builds an Env with a real router (api.SetupRouter) and a fake Keycloak.
func New(t testing.TB) *Env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	e := &Env{
		Users: fakes.NewUserRepo(), Videos: fakes.NewVideoRepo(), Metas: fakes.NewVideoMetadataRepo(),
		Index: fakes.NewVideoIndex(), Store: fakes.NewObjectStore(), Cache: fakes.NewCache(), Queue: fakes.NewQueue(),
	}
	e.Catalog = fakes.NewCatalogRepo(e.Users)
	e.Comments = fakes.NewCommentRepo(e.Users)
	signer, err := stream.NewSigner(config.Stream{BaseUrl: "http://gateway.test", TokenSecret: "s3cret", TokenTTL: "1h"})
	if err != nil {
		t.Fatal(err)
	}
	e.Signer = signer
	e.UserSvc = services.NewUserService(e.Users, e.Store)
	e.CatalogSvc = services.NewVideoCatalogService(e.Catalog, e.Comments, e.Index, e.Store, e.Cache, signer)
	sm := services.NewVideoStateMachine()
	pc := services.NewPipelineCoordinator(e.Videos, sm, e.Queue, QueueName)
	e.VideoSvc = services.NewVideoService(e.Index, e.Store, e.Metas, e.Videos, e.Queue, e.Cache, QueueName, sm, pc)
	e.Keycloak = authtest.New(t)
	e.Router = api.SetupRouter(e.VideoSvc, e.Store, e.Keycloak.Verifier(t), e.UserSvc, e.CatalogSvc)
	return e
}

// User seeds a user and returns a bearer token for it.
func (e *Env) User(t testing.TB, name string) (uuid.UUID, string) {
	t.Helper()
	id := uuid.New()
	e.Users.Put(models.User{ID: id, UserName: name, ChannelName: name + " channel"})
	return id, e.Keycloak.Token(t, id.String())
}

// VideoOpts customises SeedVideo.
type VideoOpts struct {
	Status     models.VideoStatus
	Visibility models.VideoVisibility
	Title      *string
	CreatedAt  time.Time
	Manifest   string
}

// SeedVideo stores a catalog video owned by owner (published+public by default) and returns its id.
func (e *Env) SeedVideo(owner uuid.UUID, o VideoOpts) uuid.UUID {
	if o.Status == "" {
		o.Status = models.StatusPublished
	}
	if o.Visibility == "" {
		o.Visibility = models.VisibilityPublic
	}
	if o.Title == nil {
		o.Title = fakes.Ptr("A video")
	}
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	id := uuid.New()
	d := &models.VideoDetails{VideoCard: models.VideoCard{
		ID: id, Title: o.Title, Status: o.Status, Visibility: fakes.Ptr(o.Visibility),
		Thumbnail: fakes.Ptr("thumbnails/" + id.String() + "/" + id.String() + ".jpeg"), CreatedAt: o.CreatedAt,
		Channel: models.ChannelSummary{ID: owner, UserName: "owner"},
	}}
	if o.Manifest != "" {
		d.ManifestPath = fakes.Ptr(o.Manifest)
	}
	e.Catalog.Put(d)
	return id
}

// Envelope is the util.Response JSON shape.
type Envelope struct {
	Message string          `json:"message"`
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Status  int             `json:"status"`
	Error   json.RawMessage `json:"error"`
}

// Do performs a request against the router. body may be nil, a string, or any JSON-marshalable value.
func (e *Env) Do(t testing.TB, method, path, token string, body any) (*httptest.ResponseRecorder, Envelope) {
	t.Helper()
	return Do(t, e.Router, method, path, token, body)
}

// Do performs a request against any handler and decodes the envelope (when the body is JSON).
func Do(t testing.TB, h http.Handler, method, path, token string, body any) (*httptest.ResponseRecorder, Envelope) {
	t.Helper()
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		r = bytes.NewBufferString(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewBuffer(raw)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	var env Envelope
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &env)
	}
	return w, env
}

// DecodeData unmarshals env.Data into v.
func DecodeData(t testing.TB, env Envelope, v any) {
	t.Helper()
	if err := json.Unmarshal(env.Data, v); err != nil {
		t.Fatalf("decode data %s: %v", env.Data, err)
	}
}
