package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/testutil/fakes"
	"github.com/google/uuid"
)

var errBoom = errors.New("boom")

func TestPagingOffset(t *testing.T) {
	tests := []struct {
		p    services.Paging
		want int
	}{
		{services.Paging{Page: 1, Size: 20}, 0},
		{services.Paging{Page: 2, Size: 20}, 20},
		{services.Paging{Page: 5, Size: 3}, 12},
	}
	for _, tt := range tests {
		if got := tt.p.Offset(); got != tt.want {
			t.Errorf("%+v.Offset() = %d, want %d", tt.p, got, tt.want)
		}
	}
}

func TestBrowseLatest(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	owner := uuid.New()
	var ids []uuid.UUID
	for i := 0; i < 5; i++ {
		ids = append(ids, e.video(owner, withCreated(i), withAvatar("avatars/o/a.png")))
	}
	e.video(owner, withStatus(models.StatusTranscoded), withCreated(10))
	e.video(owner, withVis(models.VisibilityPrivate), withCreated(11))
	e.video(owner, withVis(models.VisibilityUnlisted), withCreated(12))

	tests := []struct {
		name    string
		query   string
		p       services.Paging
		wantIDs []uuid.UUID
		hasMore bool
	}{
		{"first page", "", services.Paging{Page: 1, Size: 2}, []uuid.UUID{ids[4], ids[3]}, true},
		{"middle page", "", services.Paging{Page: 2, Size: 2}, []uuid.UUID{ids[2], ids[1]}, true},
		{"last page", "", services.Paging{Page: 3, Size: 2}, []uuid.UUID{ids[0]}, false},
		{"past the end", "", services.Paging{Page: 9, Size: 2}, nil, false},
		{"exact fit", "", services.Paging{Page: 1, Size: 5}, []uuid.UUID{ids[4], ids[3], ids[2], ids[1], ids[0]}, false},
		{"whitespace query is the feed", "   ", services.Paging{Page: 1, Size: 1}, []uuid.UUID{ids[4]}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := e.svc.Browse(ctx, tt.query, tt.p)
			if err != nil {
				t.Fatal(err)
			}
			if page.HasMore != tt.hasMore || page.Page != tt.p.Page || page.Size != tt.p.Size {
				t.Errorf("page meta = %+v", page)
			}
			if page.Items == nil {
				t.Error("items must be an empty slice, not nil (JSON [])")
			}
			if len(page.Items) != len(tt.wantIDs) {
				t.Fatalf("got %d items, want %d", len(page.Items), len(tt.wantIDs))
			}
			for i, c := range page.Items {
				if c.ID != tt.wantIDs[i] {
					t.Errorf("item %d = %s, want %s", i, c.ID, tt.wantIDs[i])
				}
				if c.ThumbnailURL != "https://get.test/"+*c.Thumbnail {
					t.Errorf("thumbnail url = %q", c.ThumbnailURL)
				}
				if c.Channel.AvatarURL != "https://get.test/avatars/o/a.png" {
					t.Errorf("avatar url = %q", c.Channel.AvatarURL)
				}
			}
			if e.repo.LastLimit != tt.p.Size+1 || e.repo.LastOffset != tt.p.Offset() {
				t.Errorf("repo called with offset %d limit %d", e.repo.LastOffset, e.repo.LastLimit)
			}
			if e.index.Calls.Count("SearchVideo") != 0 {
				t.Error("feed must not hit the search index")
			}
		})
	}
}

func TestBrowseSearch(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	owner := uuid.New()
	a := e.video(owner, withTitle(fakes.Ptr("Go testing basics")))
	b := e.video(owner, withTitle(fakes.Ptr("Advanced go testing")))
	priv := e.video(owner, withTitle(fakes.Ptr("Private go testing")), withVis(models.VisibilityPrivate))
	c := e.video(owner, withTitle(fakes.Ptr("Go testing again")))
	for _, id := range []uuid.UUID{a, b, priv, c} {
		d, _ := e.repo.Video(id)
		_ = e.index.InsertVideo(ctx, &models.Video{ID: id, Title: d.Title})
	}
	e.index.ExtraIDs = []string{"not-a-uuid"}

	page, err := e.svc.Browse(ctx, "  testing ", services.Paging{Page: 1, Size: 2})
	if err != nil {
		t.Fatal(err)
	}
	if e.index.LastOffset != 0 || e.index.LastSize != 3 {
		t.Errorf("ES paging offset=%d size=%d, want 0/3", e.index.LastOffset, e.index.LastSize)
	}
	// ES returned a, b, priv (size+1) -> has_more; Postgres drops priv
	if !page.HasMore {
		t.Error("expected has_more")
	}
	if len(page.Items) != 2 || page.Items[0].ID != a || page.Items[1].ID != b {
		t.Fatalf("items = %v", page.Items)
	}
	if page.Items[0].ThumbnailURL == "" {
		t.Error("search cards must be signed")
	}

	// second page: ES gives priv, c, junk (3 hits -> has_more by ES count); priv and the junk id are dropped
	page, err = e.svc.Browse(ctx, "testing", services.Paging{Page: 2, Size: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasMore || len(page.Items) != 1 || page.Items[0].ID != c {
		t.Fatalf("page 2 = %+v", page)
	}
	// third page: only the junk id -> nothing, no more pages
	page, err = e.svc.Browse(ctx, "testing", services.Paging{Page: 3, Size: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.HasMore || len(page.Items) != 0 {
		t.Fatalf("page 3 = %+v", page)
	}
	for _, id := range e.repo.LastFilter.IDs {
		if id == uuid.Nil {
			t.Error("invalid ids must not reach the repo")
		}
	}

	// no hits -> empty, with a non-nil IDs filter (so the repo does not fall back to "all videos")
	page, err = e.svc.Browse(ctx, "nothing-matches", services.Paging{Page: 1, Size: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 || page.HasMore {
		t.Fatalf("no-hit page = %+v", page)
	}
	if e.repo.LastFilter.IDs == nil {
		t.Error("expected a non-nil (empty) IDs filter for a search with no hits")
	}
}

func TestBrowseSearchTrimsWhenPostgresReturnsMoreThanSize(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	owner := uuid.New()
	for i := 0; i < 3; i++ {
		id := e.video(owner, withTitle(fakes.Ptr("match")))
		_ = e.index.InsertVideo(ctx, &models.Video{ID: id, Title: fakes.Ptr("match")})
	}
	page, err := e.svc.Browse(ctx, "match", services.Paging{Page: 1, Size: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || !page.HasMore {
		t.Fatalf("page = %d items, has_more=%v", len(page.Items), page.HasMore)
	}
}

func TestBrowseErrors(t *testing.T) {
	ctx := context.Background()
	t.Run("index error", func(t *testing.T) {
		e := newCatalogEnv(t)
		e.index.Errs.Set("SearchVideo", errBoom)
		if _, err := e.svc.Browse(ctx, "q", services.Paging{Page: 1, Size: 2}); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("repo error on search", func(t *testing.T) {
		e := newCatalogEnv(t)
		e.repo.Errs.Set("ListPublished", errBoom)
		if _, err := e.svc.Browse(ctx, "q", services.Paging{Page: 1, Size: 2}); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("repo error on feed", func(t *testing.T) {
		e := newCatalogEnv(t)
		e.repo.Errs.Set("ListPublished", errBoom)
		if _, err := e.svc.Browse(ctx, "", services.Paging{Page: 1, Size: 2}); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("ListByOwner error", func(t *testing.T) {
		e := newCatalogEnv(t)
		e.repo.Errs.Set("ListByOwner", errBoom)
		if _, err := e.svc.MyVideos(ctx, uuid.New(), services.Paging{Page: 1, Size: 2}); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestFilteredLists(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	alice, bob, viewer := uuid.New(), uuid.New(), uuid.New()
	a1 := e.video(alice, withCreated(1))
	a2 := e.video(alice, withCreated(2))
	b1 := e.video(bob, withCreated(3))
	aPriv := e.video(alice, withVis(models.VisibilityPrivate), withCreated(4))
	aProc := e.video(alice, withStatus(models.StatusUploaded), withCreated(5))
	_ = e.users.Subscribe(ctx, viewer, bob)
	_ = e.repo.SetReaction(ctx, a1, viewer, 1)
	_ = e.repo.SetReaction(ctx, b1, viewer, -1)
	p := services.Paging{Page: 1, Size: 10}

	ids := func(page models.Page[*models.VideoCard], err error) []uuid.UUID {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		out := []uuid.UUID{}
		for _, c := range page.Items {
			out = append(out, c.ID)
		}
		return out
	}
	tests := []struct {
		name string
		got  []uuid.UUID
		want []uuid.UUID
	}{
		{"channel videos", ids(e.svc.ChannelVideos(ctx, alice, p)), []uuid.UUID{a2, a1}},
		{"subscription feed", ids(e.svc.SubscriptionFeed(ctx, viewer, p)), []uuid.UUID{b1}},
		{"liked videos", ids(e.svc.LikedVideos(ctx, viewer, p)), []uuid.UUID{a1}},
		{"related excludes self", ids(e.svc.Related(ctx, a2, p)), []uuid.UUID{b1, a1}},
		{"my videos include private and processing", ids(e.svc.MyVideos(ctx, alice, p)), []uuid.UUID{aProc, aPriv, a2, a1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.got) != len(tt.want) {
				t.Fatalf("got %v, want %v", tt.got, tt.want)
			}
			for i := range tt.got {
				if tt.got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", tt.got, tt.want)
				}
			}
		})
	}
}

func TestMyVideosPaging(t *testing.T) {
	e := newCatalogEnv(t)
	owner := uuid.New()
	for i := 0; i < 3; i++ {
		e.video(owner, withCreated(i), withStatus(models.StatusUploadPending), withNoThumb())
	}
	page, err := e.svc.MyVideos(context.Background(), owner, services.Paging{Page: 1, Size: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 || !page.HasMore {
		t.Fatalf("page = %+v", page)
	}
	if page.Items[0].ThumbnailURL != "" {
		t.Error("no thumbnail key -> empty url")
	}
	if e.repo.LastLimit != 3 {
		t.Errorf("limit = %d, want size+1", e.repo.LastLimit)
	}
}

func TestGetDetailsVisibility(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	owner, other := uuid.New(), uuid.New()
	pub := e.video(owner)
	unlisted := e.video(owner, withVis(models.VisibilityUnlisted))
	private := e.video(owner, withVis(models.VisibilityPrivate))
	processing := e.video(owner, withStatus(models.StatusTranscoded))
	failed := e.video(owner, withStatus(models.StatusPhase2Failed))
	noVis := e.video(owner, withNoVis(), withStatus(models.StatusValid))

	tests := []struct {
		name    string
		id      uuid.UUID
		viewer  *uuid.UUID
		wantErr error
	}{
		{"public anon", pub, nil, nil},
		{"public other", pub, &other, nil},
		{"unlisted anon", unlisted, nil, nil},
		{"private anon", private, nil, services.ErrNotFound},
		{"private other", private, &other, services.ErrNotFound},
		{"private owner", private, &owner, nil},
		{"processing anon", processing, nil, services.ErrNotFound},
		{"processing owner", processing, &owner, nil},
		{"failed other", failed, &other, services.ErrNotFound},
		{"no visibility anon", noVis, nil, services.ErrNotFound},
		{"no visibility owner", noVis, &owner, nil},
		{"missing", uuid.New(), nil, services.ErrNotFound},
		{"missing owner", uuid.New(), &owner, services.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := e.svc.GetDetails(ctx, tt.id, tt.viewer)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && d.ID != tt.id {
				t.Fatalf("got video %s", d.ID)
			}
			if tt.wantErr != nil && d != nil {
				t.Fatal("expected nil details on error")
			}
		})
	}
}

func TestGetDetailsSigning(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	owner := uuid.New()
	withHLS := e.video(owner, withManifest("segments/x/master.m3u8"), withAvatar("avatars/a.png"))
	noHLS := e.video(owner)
	emptyHLS := e.video(owner, withManifest(""))

	d, err := e.svc.GetDetails(ctx, withHLS, nil)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "http://gw.test/videos/"
	suffix := "/" + withHLS.String() + "/master.m3u8"
	if !strings.HasPrefix(d.ManifestURL, prefix) || !strings.HasSuffix(d.ManifestURL, suffix) {
		t.Errorf("manifest url = %q", d.ManifestURL)
	}
	if d.ThumbnailURL == "" || d.Channel.AvatarURL != "https://get.test/avatars/a.png" {
		t.Errorf("thumb=%q avatar=%q", d.ThumbnailURL, d.Channel.AvatarURL)
	}
	for _, id := range []uuid.UUID{noHLS, emptyHLS} {
		d, err := e.svc.GetDetails(ctx, id, nil)
		if err != nil {
			t.Fatal(err)
		}
		if d.ManifestURL != "" {
			t.Errorf("manifest url for video without HLS = %q", d.ManifestURL)
		}
		if d.Channel.AvatarURL != "" {
			t.Errorf("no avatar key -> empty avatar url, got %q", d.Channel.AvatarURL)
		}
	}

	// presign failure degrades to empty urls
	e.store.Errs.Set("GetUrl", errBoom)
	d, err = e.svc.GetDetails(ctx, withHLS, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.ThumbnailURL != "" || d.Channel.AvatarURL != "" {
		t.Errorf("expected empty urls when presign fails, got %q %q", d.ThumbnailURL, d.Channel.AvatarURL)
	}
}

func TestGetDetailsRepoErrorPassesThrough(t *testing.T) {
	e := newCatalogEnv(t)
	e.repo.Errs.Set("GetDetails", errBoom)
	if _, err := e.svc.GetDetails(context.Background(), uuid.New(), nil); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
}

func TestAssertOwner(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	owner := uuid.New()
	id := e.video(owner)
	tests := []struct {
		name    string
		video   uuid.UUID
		user    uuid.UUID
		repoErr error
		want    error
	}{
		{"owner", id, owner, nil, nil},
		{"not owner", id, uuid.New(), nil, services.ErrForbidden},
		{"missing", uuid.New(), owner, nil, services.ErrNotFound},
		{"repo error", id, owner, errBoom, errBoom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e.repo.Errs.Set("GetOwnerID", tt.repoErr)
			if err := e.svc.AssertOwner(ctx, tt.video, tt.user); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestUpdateValidation(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	owner := uuid.New()
	id := e.video(owner)
	untitled := e.video(owner, withTitle(nil), withStatus(models.StatusUploaded))
	bad := models.VideoVisibility("secret")

	tests := []struct {
		name  string
		video uuid.UUID
		user  uuid.UUID
		in    services.UpdateVideoInput
		want  error
		msg   string
	}{
		{"not owner", id, uuid.New(), services.UpdateVideoInput{}, services.ErrForbidden, ""},
		{"missing", uuid.New(), owner, services.UpdateVideoInput{}, services.ErrNotFound, ""},
		{"not confirmed", untitled, owner, services.UpdateVideoInput{Title: fakes.Ptr("Hello")}, services.ErrBadRequest, "confirm"},
		{"title too short", id, owner, services.UpdateVideoInput{Title: fakes.Ptr("  ab  ")}, services.ErrBadRequest, "title"},
		{"title too long", id, owner, services.UpdateVideoInput{Title: fakes.Ptr(strings.Repeat("x", 101))}, services.ErrBadRequest, "title"},
		{"hindi title too long (101 runes)", id, owner, services.UpdateVideoInput{Title: fakes.Ptr(strings.Repeat("न", 101))}, services.ErrBadRequest, "title"},
		{"hindi title too short (2 runes)", id, owner, services.UpdateVideoInput{Title: fakes.Ptr("नम")}, services.ErrBadRequest, "title"},
		{"hindi description too long", id, owner, services.UpdateVideoInput{Description: fakes.Ptr(strings.Repeat("न", 5001))}, services.ErrBadRequest, "description"},
		{"thumbnail nested", id, owner, services.UpdateVideoInput{Thumbnail: fakes.Ptr("thumbnails/" + id.String() + "/x/a.png")}, services.ErrBadRequest, "thumbnail"},
		{"thumbnail not an image", id, owner, services.UpdateVideoInput{Thumbnail: fakes.Ptr("thumbnails/" + id.String() + "/a.mp4")}, services.ErrBadRequest, "thumbnail"},
		{"thumbnail no extension", id, owner, services.UpdateVideoInput{Thumbnail: fakes.Ptr("thumbnails/" + id.String() + "/a")}, services.ErrBadRequest, "thumbnail"},
		{"description too long", id, owner, services.UpdateVideoInput{Description: fakes.Ptr(strings.Repeat("x", 5001))}, services.ErrBadRequest, "description"},
		{"thumbnail of another video", id, owner, services.UpdateVideoInput{Thumbnail: fakes.Ptr("thumbnails/" + uuid.NewString() + "/a.png")}, services.ErrBadRequest, "thumbnail"},
		{"thumbnail folder only", id, owner, services.UpdateVideoInput{Thumbnail: fakes.Ptr("thumbnails/" + id.String() + "/")}, services.ErrBadRequest, "thumbnail"},
		{"thumbnail traversal", id, owner, services.UpdateVideoInput{Thumbnail: fakes.Ptr("thumbnails/" + id.String() + "/../x/a.png")}, services.ErrBadRequest, "thumbnail"},
		{"thumbnail empty", id, owner, services.UpdateVideoInput{Thumbnail: fakes.Ptr("")}, services.ErrBadRequest, "thumbnail"},
		{"avatar key as thumbnail", id, owner, services.UpdateVideoInput{Thumbnail: fakes.Ptr("avatars/" + id.String() + "/a.png")}, services.ErrBadRequest, "thumbnail"},
		{"bad visibility", id, owner, services.UpdateVideoInput{Visibility: &bad}, services.ErrBadRequest, "visibility"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := e.repo.Calls.Count("UpdateDetails")
			_, err := e.svc.Update(ctx, tt.video, tt.user, tt.in)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if tt.msg != "" && !strings.Contains(err.Error(), tt.msg) {
				t.Errorf("err %q should mention %q", err, tt.msg)
			}
			if e.repo.Calls.Count("UpdateDetails") != before {
				t.Error("invalid input must not be written")
			}
		})
	}
}

func TestUpdateSuccess(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	owner := uuid.New()
	id := e.video(owner, withVis(models.VisibilityPrivate), withStatus(models.StatusTranscoded))
	vis := models.VisibilityUnlisted
	thumb := "thumbnails/" + id.String() + "/" + uuid.NewString() + ".png"

	d, err := e.svc.Update(ctx, id, owner, services.UpdateVideoInput{
		Title: fakes.Ptr("  New title  "), Description: fakes.Ptr("desc"), Visibility: &vis, Thumbnail: &thumb,
	})
	if err != nil {
		t.Fatal(err)
	}
	if *d.Title != "New title" || *d.Description != "desc" || *d.Visibility != vis || *d.Thumbnail != thumb {
		t.Errorf("details = %+v", d.VideoCard)
	}
	if d.ThumbnailURL != "https://get.test/"+thumb {
		t.Errorf("thumbnail url = %q", d.ThumbnailURL)
	}
	doc := e.index.Docs[id.String()]
	if doc == nil || *doc.Title != "New title" || *doc.Description != "desc" {
		t.Fatalf("index doc = %+v", doc)
	}

	// partial update keeps the other fields; nil description is indexed as ""
	e2 := newCatalogEnv(t)
	id2 := e2.video(owner)
	if _, err := e2.svc.Update(ctx, id2, owner, services.UpdateVideoInput{}); err != nil {
		t.Fatal(err)
	}
	if doc := e2.index.Docs[id2.String()]; doc == nil || doc.Description == nil || *doc.Description != "" {
		t.Fatalf("index doc = %+v", doc)
	}

	// boundary lengths are accepted
	if _, err := e.svc.Update(ctx, id, owner, services.UpdateVideoInput{
		Title: fakes.Ptr("abc"), Description: fakes.Ptr(strings.Repeat("d", 5000)),
	}); err != nil {
		t.Fatalf("boundary update: %v", err)
	}
	if _, err := e.svc.Update(ctx, id, owner, services.UpdateVideoInput{Title: fakes.Ptr(strings.Repeat("t", 100))}); err != nil {
		t.Fatalf("100-char title: %v", err)
	}
	// lengths count runes: a 40-character Hindi title is 120 bytes
	hindi := strings.Repeat("न", 40)
	if d, err := e.svc.Update(ctx, id, owner, services.UpdateVideoInput{Title: &hindi, Description: fakes.Ptr(strings.Repeat("न", 5000))}); err != nil || *d.Title != hindi {
		t.Fatalf("hindi title: %v", err)
	}
	if _, err := e.svc.Update(ctx, id, owner, services.UpdateVideoInput{Title: fakes.Ptr(strings.Repeat("न", 100))}); err != nil {
		t.Fatalf("100-rune title: %v", err)
	}
	for _, ext := range []string{"jpg", "jpeg", "png", "webp", "gif", "PNG"} {
		th := "thumbnails/" + id.String() + "/c." + ext
		if _, err := e.svc.Update(ctx, id, owner, services.UpdateVideoInput{Thumbnail: &th}); err != nil {
			t.Errorf("thumbnail .%s: %v", ext, err)
		}
	}
}

func TestUpdateErrors(t *testing.T) {
	ctx := context.Background()
	for _, tt := range []struct {
		name   string
		inject func(e *catalogEnv)
	}{
		{"UpdateDetails", func(e *catalogEnv) { e.repo.Errs.Set("UpdateDetails", errBoom) }},
		{"index", func(e *catalogEnv) { e.index.Errs.Set("UpdateVideo", errBoom) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newCatalogEnv(t)
			owner := uuid.New()
			id := e.video(owner)
			tt.inject(e)
			if _, err := e.svc.Update(ctx, id, owner, services.UpdateVideoInput{Title: fakes.Ptr("Fine title")}); !errors.Is(err, errBoom) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	t.Run("owner removes row, index doc and all prefixes", func(t *testing.T) {
		e := newCatalogEnv(t)
		owner := uuid.New()
		id := e.video(owner)
		_ = e.index.InsertVideo(ctx, &models.Video{ID: id})
		if err := e.svc.Delete(ctx, id, owner); err != nil {
			t.Fatal(err)
		}
		if _, ok := e.repo.Video(id); ok {
			t.Error("row still present")
		}
		if len(e.index.IDs()) != 0 {
			t.Error("index doc still present")
		}
		want := []string{"originals/" + id.String() + "/", "thumbnails/" + id.String() + "/", "transcoded/" + id.String() + "/", "segments/" + id.String() + "/"}
		if strings.Join(e.store.DeletedPrefixes, ",") != strings.Join(want, ",") {
			t.Errorf("deleted prefixes = %v, want %v", e.store.DeletedPrefixes, want)
		}
	})
	t.Run("cleanup failures are best effort", func(t *testing.T) {
		e := newCatalogEnv(t)
		owner := uuid.New()
		id := e.video(owner)
		e.index.Errs.Set("DeleteVideo", errBoom)
		e.store.Errs.Set("DeletePrefix", errBoom)
		if err := e.svc.Delete(ctx, id, owner); err != nil {
			t.Fatalf("err = %v", err)
		}
		if len(e.store.DeletedPrefixes) != 4 {
			t.Errorf("all four prefixes should be attempted, got %v", e.store.DeletedPrefixes)
		}
	})
	t.Run("not owner", func(t *testing.T) {
		e := newCatalogEnv(t)
		id := e.video(uuid.New())
		if err := e.svc.Delete(ctx, id, uuid.New()); !errors.Is(err, services.ErrForbidden) {
			t.Fatalf("err = %v", err)
		}
		if _, ok := e.repo.Video(id); !ok {
			t.Error("video deleted by non-owner")
		}
	})
	t.Run("missing", func(t *testing.T) {
		e := newCatalogEnv(t)
		if err := e.svc.Delete(ctx, uuid.New(), uuid.New()); !errors.Is(err, services.ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("repo delete error stops cleanup", func(t *testing.T) {
		e := newCatalogEnv(t)
		owner := uuid.New()
		id := e.video(owner)
		e.repo.Errs.Set("Delete", errBoom)
		if err := e.svc.Delete(ctx, id, owner); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
		if len(e.store.DeletedPrefixes) != 0 || e.index.Calls.Count("DeleteVideo") != 0 {
			t.Error("cleanup ran although the row was not deleted")
		}
	})
}

func TestRecordView(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	owner, viewer := uuid.New(), uuid.New()
	id := e.video(owner)
	private := e.video(owner, withVis(models.VisibilityPrivate))

	views := func(id uuid.UUID) int64 { d, _ := e.repo.Video(id); return d.Views }

	if err := e.svc.RecordView(ctx, id, nil, "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RecordView(ctx, id, nil, "1.2.3.4"); err != nil {
		t.Fatal(err)
	}
	if got := views(id); got != 1 {
		t.Fatalf("views after duplicate = %d, want 1", got)
	}
	key := "view:" + id.String() + ":1.2.3.4"
	if !e.cache.Has(key) || e.cache.TTL(key) != 30*time.Minute {
		t.Errorf("dedup key %q missing or wrong ttl %v", key, e.cache.TTL(key))
	}
	if err := e.svc.RecordView(ctx, id, &viewer, viewer.String()); err != nil {
		t.Fatal(err)
	}
	if got := views(id); got != 2 {
		t.Fatalf("views = %d, want 2", got)
	}

	if err := e.svc.RecordView(ctx, private, nil, "1.2.3.4"); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("private anon view err = %v", err)
	}
	if err := e.svc.RecordView(ctx, private, &owner, owner.String()); err != nil {
		t.Fatalf("owner view of private video: %v", err)
	}
	if views(private) != 1 {
		t.Error("owner view should count")
	}

	e.cache.Errs.Set("SetNX", errBoom)
	if err := e.svc.RecordView(ctx, id, nil, "9.9.9.9"); !errors.Is(err, errBoom) {
		t.Fatalf("cache error = %v", err)
	}
	e.cache.Errs.Set("SetNX", nil)
	e.repo.Errs.Set("IncrementViews", errBoom)
	if err := e.svc.RecordView(ctx, id, nil, "8.8.8.8"); !errors.Is(err, errBoom) {
		t.Fatalf("increment error = %v", err)
	}
}

func TestReact(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	owner, user := uuid.New(), uuid.New()
	id := e.video(owner)
	private := e.video(owner, withVis(models.VisibilityPrivate))

	for _, v := range []int{2, -2, 100} {
		if _, err := e.svc.React(ctx, id, user, v); !errors.Is(err, services.ErrBadRequest) {
			t.Errorf("value %d: err = %v", v, err)
		}
	}
	steps := []struct {
		value                    int
		likes, dislikes, myReact int64
	}{
		{1, 1, 0, 1},
		{1, 1, 0, 1},
		{-1, 0, 1, -1},
		{0, 0, 0, 0},
	}
	for _, s := range steps {
		d, err := e.svc.React(ctx, id, user, s.value)
		if err != nil {
			t.Fatal(err)
		}
		if d.Likes != s.likes || d.Dislikes != s.dislikes || int64(d.MyReaction) != s.myReact {
			t.Errorf("after %d: likes=%d dislikes=%d mine=%d", s.value, d.Likes, d.Dislikes, d.MyReaction)
		}
	}
	if _, err := e.svc.React(ctx, private, user, 1); !errors.Is(err, services.ErrNotFound) {
		t.Errorf("react to private: %v", err)
	}
	if e.repo.Reaction(private, user) != 0 {
		t.Error("reaction stored for invisible video")
	}
	e.repo.Errs.Set("SetReaction", errBoom)
	if _, err := e.svc.React(ctx, id, user, 1); !errors.Is(err, errBoom) {
		t.Errorf("repo error: %v", err)
	}
}

func TestComments(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	owner, alice, bob := uuid.New(), uuid.New(), uuid.New()
	e.users.Put(models.User{ID: alice, UserName: "alice", ChannelName: "Alice", Avatar: "avatars/alice.png"})
	id := e.video(owner)
	private := e.video(owner, withVis(models.VisibilityPrivate))

	t.Run("add validation", func(t *testing.T) {
		for _, content := range []string{"", "   \n\t", strings.Repeat("x", 2001)} {
			if _, err := e.svc.AddComment(ctx, id, alice, content); !errors.Is(err, services.ErrBadRequest) {
				t.Errorf("content len %d: err = %v", len(content), err)
			}
		}
		if _, err := e.svc.AddComment(ctx, private, alice, "hi"); !errors.Is(err, services.ErrNotFound) {
			t.Errorf("comment on private: %v", err)
		}
		if _, err := e.svc.AddComment(ctx, id, alice, strings.Repeat("x", 2000)); err != nil {
			t.Errorf("2000 chars should be fine: %v", err)
		}
	})

	var c1 *models.Comment
	t.Run("add", func(t *testing.T) {
		var err error
		c1, err = e.svc.AddComment(ctx, id, alice, "  first!  ")
		if err != nil {
			t.Fatal(err)
		}
		if c1.Content != "first!" || c1.VideoID != id || c1.Author.ID != alice {
			t.Errorf("comment = %+v", c1)
		}
		if c1.Author.AvatarURL != "https://get.test/avatars/alice.png" {
			t.Errorf("author avatar = %q", c1.Author.AvatarURL)
		}
	})
	t.Run("add repo errors", func(t *testing.T) {
		e.comments.Errs.Set("Insert", errBoom)
		if _, err := e.svc.AddComment(ctx, id, alice, "x"); !errors.Is(err, errBoom) {
			t.Errorf("insert err = %v", err)
		}
		e.comments.Errs.Set("Insert", nil)
		e.comments.Errs.Set("Get", errBoom)
		if _, err := e.svc.AddComment(ctx, id, alice, "x"); !errors.Is(err, errBoom) {
			t.Errorf("get err = %v", err)
		}
		e.comments.Errs.Set("Get", nil)
	})

	t.Run("list paging", func(t *testing.T) {
		page, err := e.svc.ListComments(ctx, id, nil, services.Paging{Page: 1, Size: 1})
		if err != nil {
			t.Fatal(err)
		}
		// 2000-char comment, "first!", plus any added by the error subtests -> at least 2
		if len(page.Items) != 1 || !page.HasMore {
			t.Fatalf("page = %+v", page)
		}
		if _, err := e.svc.ListComments(ctx, private, nil, services.Paging{Page: 1, Size: 1}); !errors.Is(err, services.ErrNotFound) {
			t.Errorf("list on private: %v", err)
		}
		if _, err := e.svc.ListComments(ctx, private, &owner, services.Paging{Page: 1, Size: 1}); err != nil {
			t.Errorf("owner list on private: %v", err)
		}
		e.comments.Errs.Set("List", errBoom)
		if _, err := e.svc.ListComments(ctx, id, nil, services.Paging{Page: 1, Size: 1}); !errors.Is(err, errBoom) {
			t.Errorf("list err = %v", err)
		}
		e.comments.Errs.Set("List", nil)
	})

	t.Run("delete permissions", func(t *testing.T) {
		mk := func(author uuid.UUID, video uuid.UUID) uuid.UUID {
			cid := uuid.New()
			e.comments.Put(&models.Comment{ID: cid, VideoID: video, Author: models.ChannelSummary{ID: author}})
			return cid
		}
		tests := []struct {
			name    string
			comment uuid.UUID
			video   uuid.UUID
			user    uuid.UUID
			want    error
		}{
			{"author", mk(alice, id), id, alice, nil},
			{"video owner", mk(alice, id), id, owner, nil},
			{"stranger", mk(alice, id), id, bob, services.ErrForbidden},
			{"wrong video", mk(alice, id), private, alice, services.ErrNotFound},
			{"missing", uuid.New(), id, alice, services.ErrNotFound},
			{"video gone, not author", mk(alice, uuid.Nil), uuid.Nil, bob, services.ErrForbidden},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := e.svc.DeleteComment(ctx, tt.video, tt.comment, tt.user)
				if !errors.Is(err, tt.want) {
					t.Fatalf("err = %v, want %v", err, tt.want)
				}
				if exists := e.comments.Has(tt.comment); (tt.want == nil) == exists && tt.name != "missing" {
					t.Errorf("comment exists=%v after delete err=%v", exists, err)
				}
			})
		}
	})
}

// compile-time reminder that the filter type the fakes record is the repository one
var _ = repository.VideoFilter{}
