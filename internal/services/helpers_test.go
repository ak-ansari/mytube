package services_test

import (
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/stream"
	"github.com/ak-ansari/mytube/internal/testutil/fakes"
	"github.com/google/uuid"
)

type catalogEnv struct {
	users    *fakes.UserRepo
	repo     *fakes.CatalogRepo
	comments *fakes.CommentRepo
	index    *fakes.VideoIndex
	store    *fakes.ObjectStore
	cache    *fakes.Cache
	svc      *services.VideoCatalogService
}

func newCatalogEnv(t *testing.T) *catalogEnv {
	t.Helper()
	users := fakes.NewUserRepo()
	e := &catalogEnv{
		users: users, repo: fakes.NewCatalogRepo(users), comments: fakes.NewCommentRepo(users),
		index: fakes.NewVideoIndex(), store: fakes.NewObjectStore(), cache: fakes.NewCache(),
	}
	signer, err := stream.NewSigner(config.Stream{BaseUrl: "http://gw.test", TokenSecret: "k", TokenTTL: "1h"})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = services.NewVideoCatalogService(e.repo, e.comments, e.index, e.store, e.cache, signer)
	return e
}

type vopt func(d *models.VideoDetails)

func withStatus(s models.VideoStatus) vopt { return func(d *models.VideoDetails) { d.Status = s } }
func withVis(v models.VideoVisibility) vopt {
	return func(d *models.VideoDetails) { d.Visibility = fakes.Ptr(v) }
}
func withNoVis() vopt            { return func(d *models.VideoDetails) { d.Visibility = nil } }
func withTitle(t *string) vopt   { return func(d *models.VideoDetails) { d.Title = t } }
func withManifest(p string) vopt { return func(d *models.VideoDetails) { d.ManifestPath = &p } }
func withAvatar(k string) vopt   { return func(d *models.VideoDetails) { d.Channel.Avatar = &k } }
func withNoThumb() vopt          { return func(d *models.VideoDetails) { d.Thumbnail = nil } }
func withCreated(min int) vopt {
	return func(d *models.VideoDetails) { d.CreatedAt = time.Date(2026, 1, 1, 0, min, 0, 0, time.UTC) }
}

func (e *catalogEnv) video(owner uuid.UUID, opts ...vopt) uuid.UUID {
	id := uuid.New()
	d := &models.VideoDetails{VideoCard: models.VideoCard{
		ID: id, Title: fakes.Ptr("Title " + id.String()[:4]), Status: models.StatusPublished,
		Visibility: fakes.Ptr(models.VisibilityPublic), Thumbnail: fakes.Ptr("thumbnails/" + id.String() + "/auto.jpeg"),
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Channel:   models.ChannelSummary{ID: owner, UserName: "owner"},
	}}
	for _, o := range opts {
		o(d)
	}
	e.repo.Put(d)
	return id
}
