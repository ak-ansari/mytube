package workers

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/storage"
	"github.com/ak-ansari/mytube/internal/testutil/fakes"
	"github.com/ak-ansari/mytube/internal/util"
	"github.com/google/uuid"
)

const (
	qName      = "jobs"
	bucketHash = "bucketevents"
)

var errBoom = errors.New("boom")

func TestHandlerRegistry(t *testing.T) {
	hr := NewHandlerRegistry(&Validate{}, &Thumbnail{}, &Transcode{}, &Segment{}, &Publish{}, &Checksum{})
	for stage := 2; stage <= 7; stage++ {
		if h, err := hr.GetHandler(stage); err != nil || h == nil {
			t.Errorf("stage %d: %v", stage, err)
		}
	}
	// stage 1 is driven by the bucket event, stage 8 is terminal
	for _, stage := range []int{0, 1, 8, 9, -1} {
		if _, err := hr.GetHandler(stage); err == nil {
			t.Errorf("stage %d: expected no handler", stage)
		}
	}
	// every non-terminal pipeline stage after the upload has a handler
	for order, s := range models.Stages {
		if order >= 2 && !s.IsTerminal {
			if _, err := hr.GetHandler(order); err != nil {
				t.Errorf("non-terminal stage %d (%s) has no handler", order, s.Name)
			}
		}
	}
}

func TestTargetSizes(t *testing.T) {
	labels := func(qs []util.Quality) string {
		s := ""
		for _, q := range qs {
			s += q.Label + ","
		}
		return s
	}
	tests := []struct {
		height int
		want   string
	}{
		{0, "240p,"},
		{-1, "240p,"},
		{144, "240p,"},
		{240, "240p,"},
		{359, "240p,"},
		{360, "240p,360p,"},
		{720, "240p,360p,480p,720p,"},
		{1080, "240p,360p,480p,720p,1080p,"},
		{2160, "240p,360p,480p,720p,1080p,"},
	}
	for _, tt := range tests {
		if got := labels(targetSizes(tt.height)); got != tt.want {
			t.Errorf("targetSizes(%d) = %s, want %s", tt.height, got, tt.want)
		}
	}
}

func TestParseDur(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"12.4", 12, false},
		{"12.5", 13, false},
		{"0", 0, false},
		{"3600.000000", 3600, false},
		{"", 0, true},
		{"N/A", 0, true},
	}
	for _, tt := range tests {
		got, err := parseDur(tt.in)
		if (err != nil) != tt.wantErr || (!tt.wantErr && got != tt.want) {
			t.Errorf("parseDur(%q) = %d, %v", tt.in, got, err)
		}
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	_ = os.WriteFile(f, []byte("x"), 0o600)
	if !FileExists(f) || !FileExists(dir) || FileExists(filepath.Join(dir, "nope")) {
		t.Error("FileExists")
	}
}

func TestNoopStages(t *testing.T) {
	l := &fakes.Logger{}
	p := &jobs.JobPayload{VideoID: "v"}
	if err := NewChecksum(l).Handle(context.Background(), p); err != nil {
		t.Error(err)
	}
	if err := NewPublish(nil, l).Handle(context.Background(), p); err != nil {
		t.Error(err)
	}
	if l.Count("success") != 2 {
		t.Errorf("log = %v", l.Msgs)
	}
	// constructors only store their deps
	if NewValidate(nil, nil, nil, l).log != l || NewThumbnail(nil, nil, nil, l).log != l ||
		NewTranscoder(nil, nil, nil, l).log != l || NewSegment(nil, nil, nil, l).log != l {
		t.Error("constructors should keep the logger")
	}
}

// ------------------------------------------------------------------ runner

type runnerEnv struct {
	videos *fakes.VideoRepo
	metas  *fakes.VideoMetadataRepo
	store  *fakes.ObjectStore
	cache  *fakes.Cache
	queue  *fakes.Queue
	log    *fakes.Logger
	mu     sync.Mutex
	calls  map[int]int
	fail   map[int]error
	r      *Runner
}

// minioEvent decodes just enough of a MinIO notification (as in testdata/sample_event.json).
func minioEvent(raw string) (*storage.BucketEvent, error) {
	var p struct {
		Records []struct {
			EventName string `json:"eventName"`
			S3        struct {
				Object struct {
					Key string `json:"key"`
				} `json:"object"`
			} `json:"s3"`
		} `json:"Records"`
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, err
	}
	if len(p.Records) == 0 {
		return nil, errors.New("no records")
	}
	key, err := url.QueryUnescape(p.Records[0].S3.Object.Key)
	if err != nil {
		return nil, err
	}
	ev := ""
	if len(p.Records[0].EventName) > 16 && p.Records[0].EventName[:16] == "s3:ObjectCreated" {
		ev = storage.BucketEventCreated
	}
	return &storage.BucketEvent{Event: ev, ObjectKey: key}, nil
}

func newRunnerEnv(t *testing.T) *runnerEnv {
	t.Helper()
	e := &runnerEnv{
		videos: fakes.NewVideoRepo(), metas: fakes.NewVideoMetadataRepo(), store: fakes.NewObjectStore(),
		cache: fakes.NewCache(), queue: fakes.NewQueue(), log: &fakes.Logger{},
		calls: map[int]int{}, fail: map[int]error{},
	}
	e.store.ParseFunc = minioEvent
	sm := services.NewVideoStateMachine()
	pc := services.NewPipelineCoordinator(e.videos, sm, e.queue, qName)
	vs := services.NewVideoService(fakes.NewVideoIndex(), e.store, e.metas, e.videos, e.queue, e.cache, qName, sm, pc)
	hr := &HandlerRegistry{registry: map[int]Handler{}}
	for stage := 2; stage <= 7; stage++ {
		stage := stage
		hr.registry[stage] = func(ctx context.Context, p *jobs.JobPayload) error {
			e.mu.Lock()
			defer e.mu.Unlock()
			e.calls[stage]++
			return e.fail[stage]
		}
	}
	e.r = NewRunner(e.queue, e.cache, qName, bucketHash, e.log, hr, pc, vs)
	return e
}

func (e *runnerEnv) seed(stage int, status models.VideoStatus, title *string) uuid.UUID {
	id := uuid.New()
	key := "originals/" + id.String() + "/original.mp4"
	e.videos.Put(&models.Video{ID: id, Stage: stage, Status: status, FileKey: key, Title: title})
	e.metas.Put(&models.VideoMetadata{VideoId: id, OriginalObjectKey: key})
	return id
}

func (e *runnerEnv) stage(t *testing.T, id uuid.UUID) (int, models.VideoStatus) {
	t.Helper()
	v, err := e.videos.Get(context.Background(), id.String())
	if err != nil {
		t.Fatal(err)
	}
	return v.Stage, v.Status
}

func sampleEventFor(t *testing.T, id uuid.UUID) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/sample_event.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	rec := m["Records"].([]any)[0].(map[string]any)
	rec["s3"].(map[string]any)["object"].(map[string]any)["key"] = url.QueryEscape("originals/" + id.String() + "/original.mp4")
	b, _ := json.Marshal(m)
	return string(b)
}

func TestSampleEventParses(t *testing.T) {
	raw, _ := os.ReadFile("testdata/sample_event.json")
	ev, err := minioEvent(string(raw))
	if err != nil || ev.Event != storage.BucketEventCreated || ev.ObjectKey != "originals/60b45786-76b6-4983-905d-926f97197367/original.mkv" {
		t.Fatalf("sample event = %+v, %v", ev, err)
	}
}

func TestProcessBucketEvents(t *testing.T) {
	ctx := context.Background()
	e := newRunnerEnv(t)
	uploaded := e.seed(1, models.StatusUploadPending, nil)
	unknown := uuid.New() // event for a key nobody presigned
	locked := e.seed(1, models.StatusUploadPending, nil)

	e.cache.PutHash(bucketHash, "ok", sampleEventFor(t, uploaded))
	e.cache.PutHash(bucketHash, "garbage", "{not json")
	e.cache.PutHash(bucketHash, "unknown", sampleEventFor(t, unknown))
	e.cache.PutHash(bucketHash, "thumb", `{"Records":[{"eventName":"s3:ObjectCreated:Put","s3":{"object":{"key":"thumbnails%2Fx%2Fx.jpeg"}}}]}`)
	e.cache.PutHash(bucketHash, "locked", sampleEventFor(t, locked))
	_, _ = e.cache.SetNX(ctx, "bucket_event_lock:locked", "1", time.Minute)

	if err := e.r.ProcessBucketEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if s, st := e.stage(t, uploaded); s != 2 || st != models.StatusUploaded {
		t.Errorf("uploaded video at %d/%s", s, st)
	}
	if s, _ := e.stage(t, locked); s != 1 {
		t.Error("event locked by another worker was processed")
	}
	if n := e.cache.HashLen(bucketHash); n != 1 {
		t.Errorf("%d events left, want only the locked one", n)
	}
	for _, k := range []string{"ok", "garbage", "unknown", "thumb"} {
		if e.cache.Has("bucket_event_lock:" + k) {
			t.Errorf("lock for %s not released", k)
		}
	}
	msgs := e.queue.Messages(qName)
	if len(msgs) != 1 {
		t.Fatalf("jobs = %d", len(msgs))
	}
	var p jobs.JobPayload
	_ = json.Unmarshal(msgs[0], &p)
	if p.VideoID != uploaded.String() || p.Stage != 2 {
		t.Errorf("job = %+v", p)
	}
	if e.log.Count("error") < 2 {
		t.Errorf("dropped events should be logged: %v", e.log.Msgs)
	}

	e.cache.Errs.Set("GetAllFromHash", errBoom)
	if err := e.r.ProcessBucketEvents(ctx); !errors.Is(err, errBoom) {
		t.Errorf("hash read error = %v", err)
	}
}

func TestProcessBucketEventAdvanceFailureIsDropped(t *testing.T) {
	ctx := context.Background()
	e := newRunnerEnv(t)
	id := e.seed(8, models.StatusPublished, fakes.Ptr("t")) // terminal: Advance errors
	e.cache.PutHash(bucketHash, "k", sampleEventFor(t, id))
	if err := e.r.ProcessBucketEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if e.cache.HashLen(bucketHash) != 0 {
		t.Error("failed event should be dropped, not retried")
	}
	e.cache.PutHash(bucketHash, "k2", sampleEventFor(t, id))
	e.cache.Errs.Set("DeleteFromHash", errBoom)
	if err := e.r.ProcessBucketEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if e.cache.Has("bucket_event_lock:k2") {
		t.Error("lock must be released even when the hash delete fails")
	}
}

func TestProcessQueueTasks(t *testing.T) {
	ctx := context.Background()
	t.Run("empty queue", func(t *testing.T) {
		e := newRunnerEnv(t)
		if err := e.r.ProcessQueueTasks(ctx, 0); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("dequeue error", func(t *testing.T) {
		e := newRunnerEnv(t)
		e.queue.Errs.Set("Dequeue", errBoom)
		if err := e.r.ProcessQueueTasks(ctx, 0); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("bad payload", func(t *testing.T) {
		e := newRunnerEnv(t)
		_ = e.queue.Enqueue(ctx, qName, []byte("nope"))
		if err := e.r.ProcessQueueTasks(ctx, 0); err == nil {
			t.Fatal("expected unmarshal error")
		}
	})
	t.Run("success advances and enqueues next", func(t *testing.T) {
		e := newRunnerEnv(t)
		id := e.seed(2, models.StatusUploaded, nil)
		b, _ := json.Marshal(jobs.JobPayload{VideoID: id.String(), Stage: 2, Status: models.StatusUploaded})
		_ = e.queue.Enqueue(ctx, qName, b)
		if err := e.r.ProcessQueueTasks(ctx, 0); err != nil {
			t.Fatal(err)
		}
		if e.calls[2] != 1 {
			t.Errorf("validate handler calls = %d", e.calls[2])
		}
		if s, st := e.stage(t, id); s != 3 || st != models.StatusValid {
			t.Errorf("video at %d/%s", s, st)
		}
		if n := len(e.queue.Messages(qName)); n != 1 {
			t.Errorf("next jobs = %d", n)
		}
	})
	t.Run("handler failure marks video failed, no retry", func(t *testing.T) {
		e := newRunnerEnv(t)
		id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
		e.fail[4] = errBoom
		b, _ := json.Marshal(jobs.JobPayload{VideoID: id.String(), Stage: 4})
		_ = e.queue.Enqueue(ctx, qName, b)
		if err := e.r.ProcessQueueTasks(ctx, 0); err != nil {
			t.Fatal(err)
		}
		if s, st := e.stage(t, id); s != 4 || st != models.StatusPhase2Failed {
			t.Errorf("video at %d/%s", s, st)
		}
		if n := len(e.queue.Messages(qName)); n != 0 {
			t.Errorf("job retried/enqueued: %d", n)
		}
	})
	t.Run("unknown stage marks video failed", func(t *testing.T) {
		e := newRunnerEnv(t)
		id := e.seed(2, models.StatusUploaded, nil)
		b, _ := json.Marshal(jobs.JobPayload{VideoID: id.String(), Stage: 1})
		_ = e.queue.Enqueue(ctx, qName, b)
		if err := e.r.ProcessQueueTasks(ctx, 0); err != nil {
			t.Fatal(err)
		}
		if _, st := e.stage(t, id); st != models.StatusPhase1Failed {
			t.Errorf("status = %s", st)
		}
	})
	t.Run("fail on missing video is logged", func(t *testing.T) {
		e := newRunnerEnv(t)
		e.fail[3] = errBoom
		b, _ := json.Marshal(jobs.JobPayload{VideoID: uuid.NewString(), Stage: 3})
		_ = e.queue.Enqueue(ctx, qName, b)
		if err := e.r.ProcessQueueTasks(ctx, 0); err != nil {
			t.Fatal(err)
		}
		if e.log.Count("error") != 2 {
			t.Errorf("log = %v", e.log.Msgs)
		}
	})
	t.Run("phase boundary waits for title", func(t *testing.T) {
		e := newRunnerEnv(t)
		id := e.seed(3, models.StatusValid, nil)
		b, _ := json.Marshal(jobs.JobPayload{VideoID: id.String(), Stage: 3})
		_ = e.queue.Enqueue(ctx, qName, b)
		if err := e.r.ProcessQueueTasks(ctx, 0); err != nil {
			t.Fatal(err)
		}
		if s, _ := e.stage(t, id); s != 3 {
			t.Errorf("advanced without a title to %d", s)
		}
	})
}

func TestIterate(t *testing.T) {
	ctx := context.Background()
	e := newRunnerEnv(t)
	if failed := e.r.iterate(ctx, 0); failed {
		t.Error("idle iteration reported failure")
	}
	e.cache.Errs.Set("GetAllFromHash", errBoom)
	if failed := e.r.iterate(ctx, 0); !failed {
		t.Error("infrastructure error should report failure")
	}
	e.cache.Errs.Set("GetAllFromHash", nil)
	e.r.hr.registry[5] = func(context.Context, *jobs.JobPayload) error { panic("kaboom") }
	b, _ := json.Marshal(jobs.JobPayload{VideoID: uuid.NewString(), Stage: 5})
	_ = e.queue.Enqueue(ctx, qName, b)
	if failed := e.r.iterate(ctx, 0); !failed {
		t.Error("panic should be recovered and reported as failure")
	}
	if e.log.Count("error") == 0 {
		t.Error("panic not logged")
	}
}

func TestStartAndWait(t *testing.T) {
	e := newRunnerEnv(t)
	id := e.seed(1, models.StatusUploadPending, fakes.Ptr("Titled"))
	e.cache.PutHash(bucketHash, "ev", sampleEventFor(t, id))

	ctx, cancel := context.WithCancel(context.Background())
	e.r.Start(ctx)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if v, _ := e.videos.Get(context.Background(), id.String()); v.Stage == 8 {
			break
		}
		if time.Now().After(deadline) {
			v, _ := e.videos.Get(context.Background(), id.String())
			cancel()
			e.r.Wait()
			t.Fatalf("pipeline did not finish, video at stage %d/%s", v.Stage, v.Status)
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	done := make(chan struct{})
	go func() { e.r.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return after cancel")
	}
	for stage := 2; stage <= 7; stage++ {
		if e.calls[stage] != 1 {
			t.Errorf("stage %d handler ran %d times", stage, e.calls[stage])
		}
	}
	if _, st := e.stage(t, id); st != models.StatusPublished {
		t.Errorf("final status = %s", st)
	}
}

// The media stages shell out to ffmpeg; only their pre-ffmpeg error paths are unit-testable.
func TestMediaStagesFailBeforeFFmpeg(t *testing.T) {
	ctx := context.Background()
	e := newRunnerEnv(t)
	vs := e.r.vs
	missing := &jobs.JobPayload{VideoID: uuid.NewString()}

	if err := NewValidate(vs, e.store, nil, e.log).Handle(ctx, missing); err == nil {
		t.Error("validate: missing video should fail")
	}
	if err := NewThumbnail(vs, nil, e.store, e.log).Handle(ctx, missing); err == nil {
		t.Error("thumbnail: missing video should fail")
	}
	if err := NewTranscoder(vs, e.store, nil, e.log).Handle(ctx, missing); err == nil {
		t.Error("transcode: missing video should fail")
	}
	if err := NewSegment(vs, e.store, nil, e.log).Handle(ctx, missing); err == nil {
		t.Error("segment: missing video should fail")
	}

	id := e.seed(2, models.StatusUploaded, nil)
	p := &jobs.JobPayload{VideoID: id.String()}
	if err := NewSegment(vs, e.store, nil, e.log).Handle(ctx, p); err == nil || !strings.Contains(err.Error(), "no available qualities") {
		t.Errorf("segment without renditions: %v", err)
	}

	// validate: object missing from the store
	if err := NewValidate(vs, e.store, nil, e.log).Handle(ctx, p); err == nil {
		t.Error("validate: missing object should fail")
	}
	// validate: uploaded file larger than declared at presign (declared size is 0 here)
	key := "originals/" + id.String() + "/original.mp4"
	_, _ = e.store.Put(ctx, id.String(), key, strings.NewReader("more bytes than declared"), 0)
	if err := NewValidate(vs, e.store, nil, e.log).Handle(ctx, p); err == nil || !strings.Contains(err.Error(), "exceeds declared size") {
		t.Errorf("validate oversize: %v", err)
	}
	// validate: metadata lookup failure
	e.metas.Errs.Set("Get", errBoom)
	if err := NewValidate(vs, e.store, nil, e.log).Handle(ctx, p); !errors.Is(err, errBoom) {
		t.Errorf("validate meta error: %v", err)
	}
	e.metas.Errs.Set("Get", nil)

	// thumbnail/transcode: presign failure
	e.store.Errs.Set("GetUrl", errBoom)
	if err := NewThumbnail(vs, nil, e.store, e.log).Handle(ctx, p); !errors.Is(err, errBoom) {
		t.Errorf("thumbnail url error: %v", err)
	}
	if err := NewTranscoder(vs, e.store, nil, e.log).Handle(ctx, p); !errors.Is(err, errBoom) {
		t.Errorf("transcode url error: %v", err)
	}
}
