//go:build integration

package postgres_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// publishedVideoCond: only status=published AND visibility=public is publicly listed.
func TestCatalog_PublishedVisibilityRules(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoCatalogRepo(pool)
	alice := mkUser(t, pool, "alice")
	base := time.Now().Add(-time.Hour)

	visible := mkVideo(t, pool, alice, videoOpts{CreatedAt: base})
	hidden := []uuid.UUID{
		mkVideo(t, pool, alice, videoOpts{Visibility: ptr(models.VisibilityUnlisted), CreatedAt: base.Add(1 * time.Minute)}),
		mkVideo(t, pool, alice, videoOpts{Visibility: ptr(models.VisibilityPrivate), CreatedAt: base.Add(2 * time.Minute)}),
		mkVideo(t, pool, alice, videoOpts{Visibility: noVisibility, CreatedAt: base.Add(3 * time.Minute)}),
		mkVideo(t, pool, alice, videoOpts{Status: models.StatusUploadPending, CreatedAt: base.Add(4 * time.Minute)}),
		mkVideo(t, pool, alice, videoOpts{Status: models.StatusValid, CreatedAt: base.Add(5 * time.Minute)}),
		mkVideo(t, pool, alice, videoOpts{Status: models.StatusSegmentGenerated, CreatedAt: base.Add(6 * time.Minute)}),
		mkVideo(t, pool, alice, videoOpts{Status: models.StatusPhase2Failed, CreatedAt: base.Add(7 * time.Minute)}),
	}

	all, err := repo.ListPublished(ctx, repository.VideoFilter{}, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	equalIDs(t, cardIDs(all), []uuid.UUID{visible})

	// ids filter cannot leak hidden videos either (search results path)
	byIDs, err := repo.ListPublished(ctx, repository.VideoFilter{IDs: append([]uuid.UUID{visible}, hidden...)}, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	equalIDs(t, cardIDs(byIDs), []uuid.UUID{visible})

	byChannel, _ := repo.ListPublished(ctx, repository.VideoFilter{ChannelID: &alice}, 0, 50)
	equalIDs(t, cardIDs(byChannel), []uuid.UUID{visible})

	ch, err := postgres.NewUserRepo(pool).GetChannel(ctx, alice, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ch.VideoCount != 1 {
		t.Fatalf("channel video_count = %d, want 1 (published+public only)", ch.VideoCount)
	}

	// the studio sees everything, newest first
	mine, err := repo.ListByOwner(ctx, alice, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	want := []uuid.UUID{}
	for i := len(hidden) - 1; i >= 0; i-- {
		want = append(want, hidden[i])
	}
	equalIDs(t, cardIDs(mine), append(want, visible))

	// GetDetails is not visibility-filtered (the service decides)
	if _, err := repo.GetDetails(ctx, hidden[1], nil); err != nil {
		t.Fatalf("GetDetails of private video: %v", err)
	}
}

func TestCatalog_ListPublished_CardFields(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoCatalogRepo(pool)
	alice := mkUser(t, pool, "alice")
	exec(t, pool, `UPDATE users SET avatar='avatars/a.png' WHERE id=$1`, alice)
	id := mkVideo(t, pool, alice, videoOpts{Title: "Hello", MetaThumb: "thumbnails/auto.jpeg", Duration: 93})
	exec(t, pool, `UPDATE videos SET views=7 WHERE id=$1`, id)

	cards, err := repo.ListPublished(ctx, repository.VideoFilter{}, 0, 10)
	if err != nil || len(cards) != 1 {
		t.Fatalf("cards = %v, err = %v", cards, err)
	}
	c := cards[0]
	if c.Title == nil || *c.Title != "Hello" || c.Description == nil || *c.Description != "desc" {
		t.Fatalf("title/desc: %+v", c)
	}
	if c.Status != models.StatusPublished || c.Visibility == nil || *c.Visibility != models.VisibilityPublic || c.Views != 7 {
		t.Fatalf("status/visibility/views: %+v", c)
	}
	if c.DurationSeconds == nil || *c.DurationSeconds != 93 {
		t.Fatalf("duration = %v", c.DurationSeconds)
	}
	if c.Channel.ID != alice || c.Channel.UserName != "alice" || c.Channel.ChannelName == nil ||
		*c.Channel.ChannelName != "alice channel" || c.Channel.Avatar == nil || *c.Channel.Avatar != "avatars/a.png" {
		t.Fatalf("channel summary: %+v", c.Channel)
	}
	if c.ThumbnailURL != "" || c.Channel.AvatarURL != "" {
		t.Fatal("urls are filled by the service, not the repo")
	}
}

func TestCatalog_ThumbnailFallback(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoCatalogRepo(pool)
	alice := mkUser(t, pool, "alice")

	nullThumb := mkVideo(t, pool, alice, videoOpts{MetaThumb: "thumbnails/a/auto.jpeg"})
	emptyThumb := mkVideo(t, pool, alice, videoOpts{MetaThumb: "thumbnails/b/auto.jpeg"})
	exec(t, pool, `UPDATE videos SET thumbnail='' WHERE id=$1`, emptyThumb)
	custom := mkVideo(t, pool, alice, videoOpts{MetaThumb: "thumbnails/c/auto.jpeg"})
	exec(t, pool, `UPDATE videos SET thumbnail='thumbnails/c/custom.png' WHERE id=$1`, custom)
	noMeta := mkVideo(t, pool, alice, videoOpts{NoMeta: true})

	for id, want := range map[uuid.UUID]*string{
		nullThumb:  ptr("thumbnails/a/auto.jpeg"),
		emptyThumb: ptr("thumbnails/b/auto.jpeg"),
		custom:     ptr("thumbnails/c/custom.png"),
		noMeta:     nil,
	} {
		d, err := repo.GetDetails(ctx, id, nil)
		if err != nil {
			t.Fatal(err)
		}
		if (want == nil) != (d.Thumbnail == nil) || (want != nil && *want != *d.Thumbnail) {
			t.Fatalf("thumbnail for %s = %v, want %v", id, d.Thumbnail, want)
		}
	}
	// a video without metadata still lists (LEFT JOIN)
	cards, _ := repo.ListPublished(ctx, repository.VideoFilter{IDs: []uuid.UUID{noMeta}}, 0, 10)
	if len(cards) != 1 || cards[0].DurationSeconds != nil {
		t.Fatalf("video without metadata: %v", cards)
	}
}

// Services fetch size+1 rows and models.NewPage trims and sets has_more.
func TestCatalog_PagingSizePlusOne(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoCatalogRepo(pool)
	alice := mkUser(t, pool, "alice")
	now := time.Now()
	var newestFirst []uuid.UUID
	for i := 0; i < 5; i++ {
		newestFirst = append(newestFirst, mkVideo(t, pool, alice, videoOpts{CreatedAt: now.Add(-time.Duration(i) * time.Minute)}))
	}
	mkVideo(t, pool, alice, videoOpts{Visibility: ptr(models.VisibilityPrivate)}) // never counted

	const size = 2
	cases := []struct {
		page    int
		want    []uuid.UUID
		hasMore bool
	}{
		{1, newestFirst[0:2], true},
		{2, newestFirst[2:4], true},
		{3, newestFirst[4:5], false},
		{4, nil, false},
	}
	for _, tc := range cases {
		rows, err := repo.ListPublished(ctx, repository.VideoFilter{}, (tc.page-1)*size, size+1)
		if err != nil {
			t.Fatal(err)
		}
		p := models.NewPage(rows, tc.page, size)
		if p.HasMore != tc.hasMore {
			t.Fatalf("page %d has_more = %v, want %v", tc.page, p.HasMore, tc.hasMore)
		}
		if tc.want == nil {
			tc.want = []uuid.UUID{}
		}
		equalIDs(t, cardIDs(p.Items), tc.want)
	}

	// exactly size rows -> no more
	rows, _ := repo.ListPublished(ctx, repository.VideoFilter{}, 0, 5+1)
	if p := models.NewPage(rows, 1, 5); p.HasMore || len(p.Items) != 5 {
		t.Fatalf("size=5 page: items=%d has_more=%v", len(p.Items), p.HasMore)
	}

	// studio list pages the same way
	mine, _ := repo.ListByOwner(ctx, alice, 0, 3+1)
	if p := models.NewPage(mine, 1, 3); !p.HasMore || len(p.Items) != 3 {
		t.Fatalf("owner page: items=%d has_more=%v", len(p.Items), p.HasMore)
	}
}

func TestCatalog_ListPublished_Filters(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoCatalogRepo(pool)
	users := postgres.NewUserRepo(pool)
	alice := mkUser(t, pool, "alice")
	bob := mkUser(t, pool, "bob")
	carol := mkUser(t, pool, "carol")
	now := time.Now()
	a1 := mkVideo(t, pool, alice, videoOpts{CreatedAt: now.Add(-1 * time.Minute)})
	a2 := mkVideo(t, pool, alice, videoOpts{CreatedAt: now.Add(-2 * time.Minute)})
	b1 := mkVideo(t, pool, bob, videoOpts{CreatedAt: now.Add(-3 * time.Minute)})
	c1 := mkVideo(t, pool, carol, videoOpts{CreatedAt: now.Add(-4 * time.Minute)})

	t.Run("ids keep search order", func(t *testing.T) {
		got, err := repo.ListPublished(ctx, repository.VideoFilter{IDs: []uuid.UUID{c1, a1, uuid.New(), b1}}, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		equalIDs(t, cardIDs(got), []uuid.UUID{c1, a1, b1})
		got, _ = repo.ListPublished(ctx, repository.VideoFilter{IDs: []uuid.UUID{c1, a1, b1}}, 1, 1)
		equalIDs(t, cardIDs(got), []uuid.UUID{a1})
	})
	t.Run("empty non-nil ids match nothing", func(t *testing.T) {
		got, err := repo.ListPublished(ctx, repository.VideoFilter{IDs: []uuid.UUID{}}, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		equalIDs(t, cardIDs(got), []uuid.UUID{})
	})
	t.Run("channel", func(t *testing.T) {
		got, _ := repo.ListPublished(ctx, repository.VideoFilter{ChannelID: &alice}, 0, 10)
		equalIDs(t, cardIDs(got), []uuid.UUID{a1, a2})
	})
	t.Run("subscriptions feed", func(t *testing.T) {
		_ = users.Subscribe(ctx, carol, alice)
		_ = users.Subscribe(ctx, carol, bob)
		got, _ := repo.ListPublished(ctx, repository.VideoFilter{SubscriberID: &carol}, 0, 10)
		equalIDs(t, cardIDs(got), []uuid.UUID{a1, a2, b1})
		got, _ = repo.ListPublished(ctx, repository.VideoFilter{SubscriberID: &alice}, 0, 10)
		equalIDs(t, cardIDs(got), []uuid.UUID{})
	})
	t.Run("liked", func(t *testing.T) {
		_ = repo.SetReaction(ctx, a2, bob, 1)
		_ = repo.SetReaction(ctx, c1, bob, 1)
		_ = repo.SetReaction(ctx, a1, bob, -1) // dislike is not a like
		got, _ := repo.ListPublished(ctx, repository.VideoFilter{LikedBy: &bob}, 0, 10)
		equalIDs(t, cardIDs(got), []uuid.UUID{a2, c1})
	})
	t.Run("related excludes current", func(t *testing.T) {
		got, _ := repo.ListPublished(ctx, repository.VideoFilter{ExcludeID: &a1}, 0, 10)
		equalIDs(t, cardIDs(got), []uuid.UUID{a2, b1, c1})
	})
	t.Run("combined", func(t *testing.T) {
		got, _ := repo.ListPublished(ctx, repository.VideoFilter{ChannelID: &alice, ExcludeID: &a1}, 0, 10)
		equalIDs(t, cardIDs(got), []uuid.UUID{a2})
	})
}

func TestCatalog_GetDetails_Counts(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoCatalogRepo(pool)
	users := postgres.NewUserRepo(pool)
	comments := postgres.NewCommentRepo(pool)
	meta := postgres.NewVideoMetadataRepo(pool)
	alice := mkUser(t, pool, "alice")
	bob := mkUser(t, pool, "bob")
	carol := mkUser(t, pool, "carol")
	dave := mkUser(t, pool, "dave")
	v := mkVideo(t, pool, alice, videoOpts{Title: "Watch me"})
	other := mkVideo(t, pool, alice, videoOpts{})
	_ = meta.UpdateManifest(ctx, v.String(), "segments/v/master.m3u8")
	_ = meta.UpdateQualities(ctx, v.String(), []string{"360p", "720p"})

	for _, r := range []struct {
		u   uuid.UUID
		val int
	}{{bob, 1}, {carol, 1}, {dave, -1}} {
		if err := repo.SetReaction(ctx, v, r.u, r.val); err != nil {
			t.Fatal(err)
		}
	}
	_ = repo.SetReaction(ctx, other, dave, 1) // other videos don't count
	_ = comments.Insert(ctx, uuid.New(), v, bob, "nice")
	_ = comments.Insert(ctx, uuid.New(), v, carol, "cool")
	_ = comments.Insert(ctx, uuid.New(), other, carol, "elsewhere")
	_ = users.Subscribe(ctx, bob, alice)
	_ = users.Subscribe(ctx, carol, alice)

	cases := []struct {
		name       string
		viewer     *uuid.UUID
		reaction   int
		subscribed bool
	}{
		{"anonymous", nil, 0, false},
		{"liker+subscriber", &bob, 1, true},
		{"disliker", &dave, -1, false},
		{"owner", &alice, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := repo.GetDetails(ctx, v, tc.viewer)
			if err != nil {
				t.Fatal(err)
			}
			if d.ID != v || d.Title == nil || *d.Title != "Watch me" {
				t.Fatalf("card: %+v", d.VideoCard)
			}
			if d.Likes != 2 || d.Dislikes != 1 || d.CommentCount != 2 || d.Subscribers != 2 {
				t.Fatalf("likes/dislikes/comments/subs = %d/%d/%d/%d, want 2/1/2/2", d.Likes, d.Dislikes, d.CommentCount, d.Subscribers)
			}
			if d.MyReaction != tc.reaction || d.IsSubscribed != tc.subscribed {
				t.Fatalf("my_reaction=%d is_subscribed=%v, want %d/%v", d.MyReaction, d.IsSubscribed, tc.reaction, tc.subscribed)
			}
			if d.ManifestPath == nil || *d.ManifestPath != "segments/v/master.m3u8" ||
				len(d.AvailableQualities) != 2 || d.AvailableQualities[1] != "720p" {
				t.Fatalf("manifest/qualities: %v %v", d.ManifestPath, d.AvailableQualities)
			}
		})
	}

	noMeta := mkVideo(t, pool, alice, videoOpts{NoMeta: true})
	d, err := repo.GetDetails(ctx, noMeta, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.ManifestPath != nil || d.AvailableQualities == nil || len(d.AvailableQualities) != 0 {
		t.Fatalf("no-metadata details: manifest=%v qualities=%#v", d.ManifestPath, d.AvailableQualities)
	}

	if _, err := repo.GetDetails(ctx, uuid.New(), nil); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing video: err = %v, want pgx.ErrNoRows", err)
	}
}

func TestCatalog_GetOwnerID(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoCatalogRepo(pool)
	alice := mkUser(t, pool, "alice")
	v := mkVideo(t, pool, alice, videoOpts{})
	owner, err := repo.GetOwnerID(ctx, v)
	if err != nil || owner != alice {
		t.Fatalf("owner = %s, %v; want %s", owner, err, alice)
	}
	if _, err := repo.GetOwnerID(ctx, uuid.New()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("err = %v, want pgx.ErrNoRows", err)
	}
}

func TestCatalog_UpdateDetails_NilFieldsUnchanged(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoCatalogRepo(pool)
	videos := postgres.NewVideoRepo(pool)
	v := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{Title: "Original"})
	exec(t, pool, `UPDATE videos SET thumbnail='thumbnails/x/a.jpeg', updated_at='2020-01-01' WHERE id=$1`, v)

	if err := repo.UpdateDetails(ctx, &models.Video{ID: v, Title: ptr("Renamed")}); err != nil {
		t.Fatal(err)
	}
	got, _ := videos.Get(ctx, v.String())
	if *got.Title != "Renamed" || *got.Description != "desc" || *got.Visibility != models.VisibilityPublic ||
		*got.Thumbnail != "thumbnails/x/a.jpeg" {
		t.Fatalf("partial update changed other fields: %+v", got)
	}
	if got.UpdatedAt.Year() == 2020 {
		t.Fatal("updated_at not bumped")
	}

	vis := models.VisibilityPrivate
	if err := repo.UpdateDetails(ctx, &models.Video{ID: v, Description: ptr(""), Visibility: &vis, Thumbnail: ptr("thumbnails/x/b.png")}); err != nil {
		t.Fatal(err)
	}
	got, _ = videos.Get(ctx, v.String())
	if *got.Title != "Renamed" || *got.Description != "" || *got.Visibility != vis || *got.Thumbnail != "thumbnails/x/b.png" {
		t.Fatalf("second update: %+v", got)
	}
	// now private -> gone from the public feed
	cards, _ := repo.ListPublished(ctx, repository.VideoFilter{}, 0, 10)
	if len(cards) != 0 {
		t.Fatalf("private video still listed: %v", cardIDs(cards))
	}
}

func TestCatalog_IncrementViews_Concurrent(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoCatalogRepo(pool)
	v := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{})

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- repo.IncrementViews(ctx, v)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	d, _ := repo.GetDetails(ctx, v, nil)
	if d.Views != n {
		t.Fatalf("views = %d, want %d (lost updates)", d.Views, n)
	}
	// unknown video: no error, nothing happens
	if err := repo.IncrementViews(ctx, uuid.New()); err != nil {
		t.Fatal(err)
	}
}

func TestCatalog_SetReaction(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoCatalogRepo(pool)
	alice := mkUser(t, pool, "alice")
	bob := mkUser(t, pool, "bob")
	v := mkVideo(t, pool, alice, videoOpts{})
	reactions := func() (likes, dislikes int64, mine int) {
		d, err := repo.GetDetails(ctx, v, &bob)
		if err != nil {
			t.Fatal(err)
		}
		return d.Likes, d.Dislikes, d.MyReaction
	}

	if err := repo.SetReaction(ctx, v, bob, 1); err != nil {
		t.Fatal(err)
	}
	if l, d, m := reactions(); l != 1 || d != 0 || m != 1 {
		t.Fatalf("after like: %d/%d/%d", l, d, m)
	}
	if err := repo.SetReaction(ctx, v, bob, 1); err != nil { // idempotent
		t.Fatal(err)
	}
	if err := repo.SetReaction(ctx, v, bob, -1); err != nil { // switch
		t.Fatal(err)
	}
	if l, d, m := reactions(); l != 0 || d != 1 || m != -1 {
		t.Fatalf("after dislike: %d/%d/%d", l, d, m)
	}
	if n := count(t, pool, `SELECT COUNT(*) FROM video_reactions`); n != 1 {
		t.Fatalf("one row per (user, video), got %d", n)
	}
	if err := repo.SetReaction(ctx, v, bob, 0); err != nil { // clear
		t.Fatal(err)
	}
	if l, d, m := reactions(); l != 0 || d != 0 || m != 0 {
		t.Fatalf("after clear: %d/%d/%d", l, d, m)
	}
	if err := repo.SetReaction(ctx, v, bob, 0); err != nil { // clearing nothing is fine
		t.Fatal(err)
	}
	if err := repo.SetReaction(ctx, v, bob, 2); err == nil {
		t.Fatal("value 2 must violate the check constraint")
	}
	if err := repo.SetReaction(ctx, v, uuid.New(), 1); err == nil {
		t.Fatal("unknown user must violate the foreign key")
	}
	if err := repo.SetReaction(ctx, uuid.New(), bob, 1); err == nil {
		t.Fatal("unknown video must violate the foreign key")
	}
}

func TestCatalog_Delete_Cascades(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoCatalogRepo(pool)
	alice := mkUser(t, pool, "alice")
	bob := mkUser(t, pool, "bob")
	v := mkVideo(t, pool, alice, videoOpts{})
	keep := mkVideo(t, pool, alice, videoOpts{})
	for _, id := range []uuid.UUID{v, keep} {
		_ = repo.SetReaction(ctx, id, bob, 1)
		_ = postgres.NewCommentRepo(pool).Insert(ctx, uuid.New(), id, bob, "hi")
	}
	_ = postgres.NewUserRepo(pool).Subscribe(ctx, bob, alice)

	if err := repo.Delete(ctx, v); err != nil {
		t.Fatal(err)
	}
	for table, col := range map[string]string{"videos": "id", "video_metadata": "video_id", "video_reactions": "video_id", "comments": "video_id"} {
		if n := count(t, pool, `SELECT COUNT(*) FROM `+table+` WHERE `+col+` = $1`, v); n != 0 {
			t.Fatalf("%s still has %d rows for the deleted video", table, n)
		}
		if n := count(t, pool, `SELECT COUNT(*) FROM `+table+` WHERE `+col+` = $1`, keep); n != 1 {
			t.Fatalf("%s lost rows of an unrelated video (%d)", table, n)
		}
	}
	if n := count(t, pool, `SELECT COUNT(*) FROM users`); n != 2 {
		t.Fatalf("users deleted: %d", n)
	}
	if n := count(t, pool, `SELECT COUNT(*) FROM subscriptions`); n != 1 {
		t.Fatalf("subscriptions deleted: %d", n)
	}
	// deleting a missing video is not an error
	if err := repo.Delete(ctx, uuid.New()); err != nil {
		t.Fatal(err)
	}
}

func TestCommentRepo_CRUDAndPaging(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewCommentRepo(pool)
	alice := mkUser(t, pool, "alice")
	bob := mkUser(t, pool, "bob")
	exec(t, pool, `UPDATE users SET avatar='avatars/b.png' WHERE id=$1`, bob)
	v := mkVideo(t, pool, alice, videoOpts{})
	other := mkVideo(t, pool, alice, videoOpts{})

	now := time.Now()
	var newestFirst []uuid.UUID
	for i := 0; i < 5; i++ {
		id := uuid.New()
		if err := repo.Insert(ctx, id, v, bob, "comment"); err != nil {
			t.Fatal(err)
		}
		exec(t, pool, `UPDATE comments SET created_at=$2 WHERE id=$1`, id, now.Add(-time.Duration(i)*time.Minute))
		newestFirst = append(newestFirst, id)
	}
	_ = repo.Insert(ctx, uuid.New(), other, bob, "elsewhere")

	ids := func(cs []*models.Comment) []uuid.UUID {
		out := make([]uuid.UUID, 0, len(cs))
		for _, c := range cs {
			out = append(out, c.ID)
		}
		return out
	}
	rows, err := repo.List(ctx, v, 0, 2+1)
	if err != nil {
		t.Fatal(err)
	}
	p := models.NewPage(rows, 1, 2)
	if !p.HasMore {
		t.Fatal("page 1 should have more")
	}
	equalIDs(t, ids(p.Items), newestFirst[:2])
	rows, _ = repo.List(ctx, v, 4, 2+1)
	p = models.NewPage(rows, 3, 2)
	if p.HasMore {
		t.Fatal("last page should not have more")
	}
	equalIDs(t, ids(p.Items), newestFirst[4:])

	c, err := repo.Get(ctx, newestFirst[0])
	if err != nil {
		t.Fatal(err)
	}
	if c.VideoID != v || c.Content != "comment" || c.Author.ID != bob || c.Author.UserName != "bob" ||
		c.Author.Avatar == nil || *c.Author.Avatar != "avatars/b.png" {
		t.Fatalf("unexpected comment %+v", c)
	}

	if err := repo.Delete(ctx, newestFirst[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, newestFirst[0]); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("deleted comment: err = %v, want pgx.ErrNoRows", err)
	}
	rows, _ = repo.List(ctx, v, 0, 50)
	if len(rows) != 4 {
		t.Fatalf("comments after delete = %d, want 4", len(rows))
	}
	if err := repo.Delete(ctx, uuid.New()); err != nil {
		t.Fatal(err)
	}

	if err := repo.Insert(ctx, uuid.New(), uuid.New(), bob, "x"); err == nil {
		t.Fatal("comment on unknown video must violate the foreign key")
	}
	if err := repo.Insert(ctx, newestFirst[1], v, bob, "dup"); err == nil {
		t.Fatal("duplicate comment id must violate the primary key")
	}
	empty, err := repo.List(ctx, uuid.New(), 0, 10)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("unknown video comments = %#v, %v; want empty non-nil", empty, err)
	}
}
