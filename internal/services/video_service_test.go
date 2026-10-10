package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/api/dto"
	"github.com/ak-ansari/mytube/internal/index"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/testutil/fakes"
	"github.com/google/uuid"
)

type videoEnv struct {
	videos *fakes.VideoRepo
	metas  *fakes.VideoMetadataRepo
	index  *fakes.VideoIndex
	store  *fakes.ObjectStore
	cache  *fakes.Cache
	queue  *fakes.Queue
	svc    *services.VideoService
}

func newVideoEnv() *videoEnv {
	e := &videoEnv{
		videos: fakes.NewVideoRepo(), metas: fakes.NewVideoMetadataRepo(), index: fakes.NewVideoIndex(),
		store: fakes.NewObjectStore(), cache: fakes.NewCache(), queue: fakes.NewQueue(),
	}
	sm := services.NewVideoStateMachine()
	pc := services.NewPipelineCoordinator(e.videos, sm, e.queue, qName)
	e.svc = services.NewVideoService(e.index, e.store, e.metas, e.videos, e.queue, e.cache, qName, sm, pc)
	return e
}

func (e *videoEnv) seed(stage int, status models.VideoStatus, thumb *string) uuid.UUID {
	id := uuid.New()
	key := "originals/" + id.String() + "/original.mp4"
	e.videos.Put(&models.Video{ID: id, Stage: stage, Status: status, FileKey: key})
	e.metas.Put(&models.VideoMetadata{ID: uuid.New(), VideoId: id, OriginalObjectKey: key, Thumbnail: thumb})
	return id
}

func TestUploadPreSign(t *testing.T) {
	ctx := context.Background()
	user := &models.User{ID: uuid.New()}

	t.Run("success", func(t *testing.T) {
		e := newVideoEnv()
		res, err := e.svc.UploadPreSign(ctx, &dto.UploadVideoDto{Filename: "clip.final.MKV", Size: 1234}, user)
		if err != nil {
			t.Fatal(err)
		}
		wantKey := "originals/" + res.ID + "/original.MKV"
		if res.Key != wantKey || res.Url != "https://put.test/"+wantKey+"?size=1234" {
			t.Errorf("result = %+v", res)
		}
		v, err := e.videos.Get(ctx, res.ID)
		if err != nil || v.Stage != 1 || v.Status != models.StatusUploadPending || v.UserID != user.ID || v.FileKey != wantKey {
			t.Errorf("video row = %+v, %v", v, err)
		}
		m, err := e.metas.Get(ctx, res.ID)
		if err != nil || m.Filename != "clip.final.MKV" || m.Size != 1234 || m.OriginalObjectKey != wantKey {
			t.Errorf("metadata = %+v, %v", m, err)
		}
		var cached string
		if err := e.cache.Get(ctx, "key:"+wantKey, &cached); err != nil || cached != res.ID {
			t.Errorf("key->id cache = %q, %v", cached, err)
		}
		if e.cache.TTL("key:"+wantKey) != 24*time.Hour {
			t.Errorf("cache ttl = %v", e.cache.TTL("key:"+wantKey))
		}
	})

	t.Run("no extension", func(t *testing.T) {
		e := newVideoEnv()
		if _, err := e.svc.UploadPreSign(ctx, &dto.UploadVideoDto{Filename: "video", Size: 1}, user); !errors.Is(err, services.ErrBadRequest) {
			t.Fatalf("err = %v", err)
		}
		if e.videos.Calls.Count("InsertBasic") != 0 {
			t.Error("row inserted for invalid file name")
		}
	})

	for _, step := range []struct {
		name   string
		inject func(e *videoEnv)
	}{
		{"video insert", func(e *videoEnv) { e.videos.Errs.Set("InsertBasic", errBoom) }},
		{"metadata insert", func(e *videoEnv) { e.metas.Errs.Set("InsertBasic", errBoom) }},
		{"presign", func(e *videoEnv) { e.store.Errs.Set("GerPreSignedPutUrl", errBoom) }},
		{"cache", func(e *videoEnv) { e.cache.Errs.Set("Set", errBoom) }},
	} {
		t.Run(step.name+" error", func(t *testing.T) {
			e := newVideoEnv()
			step.inject(e)
			if _, err := e.svc.UploadPreSign(ctx, &dto.UploadVideoDto{Filename: "a.mp4", Size: 1}, user); !errors.Is(err, errBoom) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func confirmDto(title string) dto.VideoConfirmDto {
	return dto.VideoConfirmDto{Title: title, Description: "desc", Visibility: models.VisibilityPublic}
}

func TestConfirmVideoValidation(t *testing.T) {
	ctx := context.Background()
	e := newVideoEnv()
	ok := e.seed(2, models.StatusUploaded, nil)
	past := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
	failed := e.seed(2, models.StatusPhase1Failed, nil)
	longDesc := confirmDto("Title")
	longDesc.Description = strings.Repeat("d", 5001)
	foreignThumb := confirmDto("Title")
	foreignThumb.Thumbnail = "thumbnails/" + uuid.NewString() + "/a.png"

	tests := []struct {
		name string
		id   string
		in   dto.VideoConfirmDto
		want error
	}{
		{"bad id", "not-a-uuid", confirmDto("Title"), services.ErrBadRequest},
		{"short title", ok.String(), confirmDto("  ab  "), services.ErrBadRequest},
		{"long title", ok.String(), confirmDto(strings.Repeat("t", 101)), services.ErrBadRequest},
		{"long description", ok.String(), longDesc, services.ErrBadRequest},
		{"foreign thumbnail", ok.String(), foreignThumb, services.ErrBadRequest},
		{"missing video", uuid.NewString(), confirmDto("Title"), services.ErrNotFound},
		{"already past phase 1", past.String(), confirmDto("Title"), services.ErrBadRequest},
		{"failed video", failed.String(), confirmDto("Title"), services.ErrBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := e.svc.ConfirmVideo(ctx, tt.id, tt.in); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
	if e.videos.Calls.Count("UpdateVideo") != 0 || e.index.Calls.Count("InsertVideo") != 0 {
		t.Error("invalid confirmations must not write anything")
	}
}

func TestConfirmVideo(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name        string
		stage       int
		status      models.VideoStatus
		metaThumb   *string
		wantStage   int
		wantAdvance bool
	}{
		{"before validation: worker continues later", 2, models.StatusUploaded, nil, 2, false},
		{"valid but no thumbnail yet", 3, models.StatusValid, nil, 3, false},
		{"valid with empty thumbnail", 3, models.StatusValid, fakes.Ptr(""), 3, false},
		{"phase 1 done: confirm advances", 3, models.StatusValid, fakes.Ptr("thumbnails/x/x.jpeg"), 4, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newVideoEnv()
			id := e.seed(tt.stage, tt.status, tt.metaThumb)
			in := confirmDto("  My video  ")
			in.Thumbnail = "thumbnails/" + id.String() + "/custom.png"
			v, err := e.svc.ConfirmVideo(ctx, id.String(), in)
			if err != nil {
				t.Fatal(err)
			}
			if *v.Title != "My video" || *v.Description != "desc" || *v.Visibility != models.VisibilityPublic || *v.Thumbnail != in.Thumbnail {
				t.Errorf("video = %+v", v)
			}
			if v.Stage != tt.wantStage {
				t.Errorf("stage = %d, want %d", v.Stage, tt.wantStage)
			}
			doc := e.index.Docs[id.String()]
			if doc == nil || *doc.Title != "My video" {
				t.Errorf("index doc = %+v", doc)
			}
			if got := len(e.queue.Messages(qName)); (got == 1) != tt.wantAdvance {
				t.Errorf("%d jobs enqueued, advance expected=%v", got, tt.wantAdvance)
			}
		})
	}
	t.Run("empty thumbnail keeps auto one", func(t *testing.T) {
		e := newVideoEnv()
		id := e.seed(2, models.StatusUploaded, nil)
		if _, err := e.svc.ConfirmVideo(ctx, id.String(), confirmDto("Title")); err != nil {
			t.Fatal(err)
		}
	})
}

func TestConfirmVideoErrors(t *testing.T) {
	ctx := context.Background()
	t.Run("postgres first: ES untouched when UpdateVideo fails", func(t *testing.T) {
		e := newVideoEnv()
		id := e.seed(2, models.StatusUploaded, nil)
		e.videos.Errs.Set("UpdateVideo", errBoom)
		if _, err := e.svc.ConfirmVideo(ctx, id.String(), confirmDto("Title")); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
		if e.index.Calls.Count("InsertVideo") != 0 {
			t.Error("indexed despite failed Postgres write")
		}
	})
	for _, step := range []struct {
		name   string
		inject func(e *videoEnv)
	}{
		{"index", func(e *videoEnv) { e.index.Errs.Set("InsertVideo", errBoom) }},
		{"metadata", func(e *videoEnv) { e.metas.Errs.Set("Get", errBoom) }},
		{"advance", func(e *videoEnv) { e.queue.Errs.Set("Enqueue", errBoom) }},
	} {
		t.Run(step.name+" error", func(t *testing.T) {
			e := newVideoEnv()
			id := e.seed(3, models.StatusValid, fakes.Ptr("thumbnails/x.jpeg"))
			step.inject(e)
			if _, err := e.svc.ConfirmVideo(ctx, id.String(), confirmDto("Title")); !errors.Is(err, errBoom) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestGetVideoKey(t *testing.T) {
	ctx := context.Background()
	e := newVideoEnv()
	id := e.seed(1, models.StatusUploadPending, nil)
	want := "originals/" + id.String() + "/original.mp4"

	got, err := e.svc.GetVideoKey(ctx, id.String())
	if err != nil || got != want {
		t.Fatalf("GetVideoKey = %q, %v", got, err)
	}
	if !e.cache.Has("key:" + id.String()) {
		t.Error("id->key not cached")
	}
	before := e.metas.Calls.Count("Get")
	if got, _ := e.svc.GetVideoKey(ctx, id.String()); got != want || e.metas.Calls.Count("Get") != before {
		t.Error("second lookup should be served from cache")
	}
	if _, err := e.svc.GetVideoKey(ctx, uuid.NewString()); err == nil {
		t.Error("missing video: expected error")
	}
	e2 := newVideoEnv()
	id2 := e2.seed(1, models.StatusUploadPending, nil)
	e2.cache.Errs.Set("Set", errBoom)
	if _, err := e2.svc.GetVideoKey(ctx, id2.String()); !errors.Is(err, errBoom) {
		t.Errorf("cache set error = %v", err)
	}
}

func TestGetVideoByKeyAndParseBucketEvent(t *testing.T) {
	ctx := context.Background()
	e := newVideoEnv()
	id := e.seed(1, models.StatusUploadPending, nil)
	key := "originals/" + id.String() + "/original.mp4"
	ev := func(event, key string) string {
		b, _ := json.Marshal(storage.BucketEvent{Event: event, ObjectKey: key})
		return string(b)
	}

	// cache miss falls back to the metadata repo
	if got, err := e.svc.GetVideoByKey(ctx, key); err != nil || got != id.String() {
		t.Fatalf("GetVideoByKey = %q, %v", got, err)
	}
	// cache hit wins
	_ = e.cache.Set(ctx, "key:"+key, "cached-id", time.Hour)
	if got, _ := e.svc.GetVideoByKey(ctx, key); got != "cached-id" {
		t.Errorf("cache hit = %q", got)
	}
	if _, err := e.svc.GetVideoByKey(ctx, "originals/none/original.mp4"); err == nil {
		t.Error("unknown key: expected error")
	}

	tests := []struct {
		name    string
		raw     string
		wantID  string
		process bool
		wantErr bool
	}{
		{"original created", ev(storage.BucketEventCreated, key), "cached-id", true, false},
		{"original deleted", ev(storage.BucketEventDeleted, key), "", false, false},
		{"thumbnail created", ev(storage.BucketEventCreated, "thumbnails/x/x.jpeg"), "", false, false},
		{"unknown event", ev("", key), "", false, false},
		{"unparseable", "{", "", false, true},
		{"unknown original", ev(storage.BucketEventCreated, "originals/none/original.mp4"), "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, process, err := e.svc.ParseBucketEvent(ctx, tt.raw)
			if (err != nil) != tt.wantErr || process != tt.process || id != tt.wantID {
				t.Fatalf("got (%q, %v, %v)", id, process, err)
			}
		})
	}
}

func TestSearchAndSuggest(t *testing.T) {
	ctx := context.Background()
	e := newVideoEnv()
	put := func(title string, status models.VideoStatus, vis *models.VideoVisibility) uuid.UUID {
		id := uuid.New()
		e.videos.Put(&models.Video{ID: id, Title: &title, Status: status, Visibility: vis})
		_ = e.index.InsertVideo(ctx, &models.Video{ID: id, Title: &title})
		return id
	}
	pub := fakes.Ptr(models.VisibilityPublic)
	a := put("cats one", models.StatusPublished, pub)
	put("cats private", models.StatusPublished, fakes.Ptr(models.VisibilityPrivate))
	put("cats processing", models.StatusTranscoded, pub)
	put("cats no vis", models.StatusPublished, nil)
	b := put("cats two", models.StatusPublished, pub)
	e.index.ExtraIDs = []string{"junk"}

	got, err := e.svc.SearchVideo(ctx, "cats", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != a || got[1].ID != b {
		t.Fatalf("search = %v", got)
	}
	if e.index.LastOffset != 0 || e.index.LastSize != 20 {
		t.Errorf("default paging offset=%d size=%d", e.index.LastOffset, e.index.LastSize)
	}
	if _, err := e.svc.SearchVideo(ctx, "cats", "3", "5"); err != nil || e.index.LastOffset != 10 || e.index.LastSize != 5 {
		t.Errorf("paging offset=%d size=%d err=%v", e.index.LastOffset, e.index.LastSize, err)
	}
	if _, err := e.svc.SearchVideo(ctx, "cats", "-1", "abc"); err != nil || e.index.LastOffset != 0 || e.index.LastSize != 20 {
		t.Errorf("invalid paging offset=%d size=%d err=%v", e.index.LastOffset, e.index.LastSize, err)
	}

	e.index.Suggestions = []index.Suggestion{{ID: a.String(), Text: "cats one"}, {ID: "junk", Text: "junk"}, {ID: uuid.NewString(), Text: "gone"}, {ID: b.String(), Text: "cats two"}}
	texts, err := e.svc.SuggestVideo(ctx, "ca")
	if err != nil || strings.Join(texts, "|") != "cats one|cats two" {
		t.Fatalf("suggest = %v, %v", texts, err)
	}
	e.index.Suggestions = nil
	if texts, err := e.svc.SuggestVideo(ctx, "zz"); err != nil || texts == nil || len(texts) != 0 {
		t.Errorf("empty suggest = %#v, %v", texts, err)
	}

	e.videos.Errs.Set("GetByIds", errBoom)
	if _, err := e.svc.SearchVideo(ctx, "cats", "", ""); !errors.Is(err, errBoom) {
		t.Errorf("search repo error = %v", err)
	}
	e.index.Suggestions = []index.Suggestion{{ID: a.String(), Text: "x"}}
	if _, err := e.svc.SuggestVideo(ctx, "c"); !errors.Is(err, errBoom) {
		t.Errorf("suggest repo error = %v", err)
	}
	e.index.Errs.Set("SearchVideo", errBoom)
	e.index.Errs.Set("SuggestVideos", errBoom)
	if _, err := e.svc.SearchVideo(ctx, "cats", "", ""); !errors.Is(err, errBoom) {
		t.Errorf("search index error = %v", err)
	}
	if _, err := e.svc.SuggestVideo(ctx, "c"); !errors.Is(err, errBoom) {
		t.Errorf("suggest index error = %v", err)
	}
}

func TestVideoServicePassthroughs(t *testing.T) {
	ctx := context.Background()
	e := newVideoEnv()
	id := e.seed(1, models.StatusUploadPending, nil)
	sid := id.String()

	if err := e.svc.UpdateMeta(ctx, sid, "sha", 61, "h264", "aac", 1920, 1080); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.UpdateQualities(ctx, sid, []string{"240p", "720p"}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.UpdateManifest(ctx, sid, "segments/"+sid+"/master.m3u8"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.UpdateThumbnail(ctx, sid, "thumbnails/"+sid+"/"+sid+".jpeg"); err != nil {
		t.Fatal(err)
	}
	m, err := e.svc.GetMeta(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	if *m.SHA256 != "sha" || *m.DurationSeconds != 61 || *m.Height != 1080 || len(m.AvailableQualities) != 2 ||
		*m.ManifestPath != "segments/"+sid+"/master.m3u8" || *m.Thumbnail != "thumbnails/"+sid+"/"+sid+".jpeg" {
		t.Errorf("meta = %+v", m)
	}
	if m2, err := e.svc.GetVideo(ctx, sid); err != nil || m2.VideoId != id {
		t.Errorf("GetVideo = %+v, %v", m2, err)
	}
	if u, err := e.svc.GetDownloadUrl(ctx, "k"); err != nil || u != "https://get.test/k" {
		t.Errorf("GetDownloadUrl = %q, %v", u, err)
	}
	if p := e.svc.GetTranscodingPath("v1", "720p"); p != "transcoded/v1/720p.mp4" {
		t.Errorf("GetTranscodingPath = %q", p)
	}
	if p := e.svc.GetHlsDir("v1"); p != "segments/v1" {
		t.Errorf("GetHlsDir = %q", p)
	}
	if s := e.svc.DownloadVideo(); s == "" {
		t.Error("DownloadVideo returned empty string")
	}
	sum, err := e.svc.CalculateChecksum(strings.NewReader("abc"))
	if err != nil || sum != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Errorf("CalculateChecksum = %q, %v", sum, err)
	}
	if _, err := e.svc.CalculateChecksum(errReader{}); !errors.Is(err, errBoom) {
		t.Errorf("checksum read error = %v", err)
	}
	if err := e.svc.UpdateManifest(ctx, uuid.NewString(), "x"); err == nil {
		t.Error("update of missing metadata: expected error")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errBoom }
