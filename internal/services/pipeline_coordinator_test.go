package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/testutil/fakes"
	"github.com/google/uuid"
)

const qName = "jobs"

type pipeEnv struct {
	videos *fakes.VideoRepo
	queue  *fakes.Queue
	pc     *services.PipelineCoordinator
}

func newPipeEnv() *pipeEnv {
	e := &pipeEnv{videos: fakes.NewVideoRepo(), queue: fakes.NewQueue()}
	e.pc = services.NewPipelineCoordinator(e.videos, services.NewVideoStateMachine(), e.queue, qName)
	return e
}

func (e *pipeEnv) seed(stage int, status models.VideoStatus, title *string) string {
	id := uuid.New()
	e.videos.Put(&models.Video{ID: id, Stage: stage, Status: status, Title: title})
	return id.String()
}

func (e *pipeEnv) jobs(t *testing.T) []jobs.JobPayload {
	t.Helper()
	var out []jobs.JobPayload
	for _, raw := range e.queue.Messages(qName) {
		var p jobs.JobPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			t.Fatalf("bad job %s: %v", raw, err)
		}
		out = append(out, p)
	}
	return out
}

func TestAdvance(t *testing.T) {
	ctx := context.Background()
	title := fakes.Ptr("Titled")
	tests := []struct {
		name       string
		stage      int
		status     models.VideoStatus
		title      *string
		wantStage  int
		wantStatus models.VideoStatus
		wantJob    bool
	}{
		{"upload pending -> uploaded", 1, models.StatusUploadPending, nil, 2, models.StatusUploaded, true},
		{"uploaded -> valid", 2, models.StatusUploaded, nil, 3, models.StatusValid, true},
		{"phase boundary waits for title", 3, models.StatusValid, nil, 3, models.StatusValid, false},
		{"phase boundary with title", 3, models.StatusValid, title, 4, models.StatusThumbnailGenerated, true},
		{"thumbnail -> transcoded", 4, models.StatusThumbnailGenerated, title, 5, models.StatusTranscoded, true},
		{"transcoded -> segments", 5, models.StatusTranscoded, title, 6, models.StatusSegmentGenerated, true},
		{"segments -> publish job", 6, models.StatusSegmentGenerated, title, 7, models.StatusPublished, true},
		{"publish -> terminal, no job", 7, models.StatusPublished, title, 8, models.StatusPublished, false},
		{"failed video is not advanced", 2, models.StatusPhase1Failed, nil, 2, models.StatusPhase1Failed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newPipeEnv()
			id := e.seed(tt.stage, tt.status, tt.title)
			if err := e.pc.Advance(ctx, id); err != nil {
				t.Fatal(err)
			}
			v, _ := e.videos.Get(ctx, id)
			if v.Stage != tt.wantStage || v.Status != tt.wantStatus {
				t.Errorf("video at %d/%s, want %d/%s", v.Stage, v.Status, tt.wantStage, tt.wantStatus)
			}
			js := e.jobs(t)
			if !tt.wantJob {
				if len(js) != 0 {
					t.Errorf("unexpected jobs %+v", js)
				}
				return
			}
			want := jobs.JobPayload{VideoID: id, Stage: tt.wantStage, Status: tt.wantStatus}
			if len(js) != 1 || js[0] != want {
				t.Errorf("jobs = %+v, want [%+v]", js, want)
			}
		})
	}
}

func TestAdvanceErrors(t *testing.T) {
	ctx := context.Background()
	t.Run("missing video", func(t *testing.T) {
		e := newPipeEnv()
		if err := e.pc.Advance(ctx, uuid.NewString()); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("terminal stage has no next stage", func(t *testing.T) {
		e := newPipeEnv()
		id := e.seed(8, models.StatusPublished, fakes.Ptr("t"))
		if err := e.pc.Advance(ctx, id); !errors.Is(err, services.ErrorInvalidStage) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unknown stage", func(t *testing.T) {
		e := newPipeEnv()
		id := e.seed(0, "", nil)
		if err := e.pc.Advance(ctx, id); !errors.Is(err, services.ErrorInvalidStage) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("AdvanceState error", func(t *testing.T) {
		e := newPipeEnv()
		id := e.seed(1, models.StatusUploadPending, nil)
		e.videos.Errs.Set("AdvanceState", errBoom)
		if err := e.pc.Advance(ctx, id); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
		if len(e.jobs(t)) != 0 {
			t.Error("job enqueued despite failed state update")
		}
	})
	t.Run("enqueue error", func(t *testing.T) {
		e := newPipeEnv()
		id := e.seed(1, models.StatusUploadPending, nil)
		e.queue.Errs.Set("Enqueue", errBoom)
		if err := e.pc.Advance(ctx, id); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
	})
}

// barrierRepo makes every caller read the video before any of them writes, reproducing the race
// that AdvanceState's "WHERE stage = from" guard exists for.
type barrierRepo struct {
	*fakes.VideoRepo
	wg *sync.WaitGroup
}

func (r barrierRepo) Get(ctx context.Context, id string) (*models.Video, error) {
	v, err := r.VideoRepo.Get(ctx, id)
	r.wg.Done()
	r.wg.Wait()
	return v, err
}

func TestAdvanceConcurrentCallersEnqueueOnce(t *testing.T) {
	ctx := context.Background()
	const n = 16
	videos, q := fakes.NewVideoRepo(), fakes.NewQueue()
	id := uuid.New()
	videos.Put(&models.Video{ID: id, Stage: 3, Status: models.StatusValid, Title: fakes.Ptr("Titled")})
	var barrier sync.WaitGroup
	barrier.Add(n)
	pc := services.NewPipelineCoordinator(barrierRepo{videos, &barrier}, services.NewVideoStateMachine(), q, qName)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := pc.Advance(ctx, id.String()); err != nil {
				t.Errorf("Advance: %v", err)
			}
		}()
	}
	wg.Wait()
	if msgs := q.Messages(qName); len(msgs) != 1 {
		t.Fatalf("%d jobs enqueued, want exactly 1", len(msgs))
	}
	if v, _ := videos.Get(ctx, id.String()); v.Stage != 4 {
		t.Errorf("stage = %d, want 4", v.Stage)
	}
}

func TestFail(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		stage int
		want  models.VideoStatus
	}{
		{1, models.StatusPhase1Failed},
		{2, models.StatusPhase1Failed},
		{3, models.StatusPhase1Failed},
		{4, models.StatusPhase2Failed},
		{6, models.StatusPhase2Failed},
		{7, models.StatusPhase2Failed},
	}
	for _, tt := range tests {
		e := newPipeEnv()
		id := e.seed(tt.stage, "whatever", nil)
		if err := e.pc.Fail(ctx, id, "ffmpeg exploded"); err != nil {
			t.Fatalf("stage %d: %v", tt.stage, err)
		}
		v, _ := e.videos.Get(ctx, id)
		if v.Status != tt.want || v.Stage != tt.stage {
			t.Errorf("stage %d: video now %d/%s, want %d/%s", tt.stage, v.Stage, v.Status, tt.stage, tt.want)
		}
		if len(e.jobs(t)) != 0 {
			t.Error("Fail must not enqueue")
		}
		// a failed video can no longer be advanced
		_ = e.pc.Advance(ctx, id)
		if v2, _ := e.videos.Get(ctx, id); v2.Status != tt.want {
			t.Errorf("failed video advanced to %s", v2.Status)
		}
	}
}

func TestFailErrors(t *testing.T) {
	ctx := context.Background()
	e := newPipeEnv()
	if err := e.pc.Fail(ctx, uuid.NewString(), "x"); err == nil {
		t.Error("missing video: expected error")
	}
	id := e.seed(42, "", nil)
	if err := e.pc.Fail(ctx, id, "x"); !errors.Is(err, services.ErrorInvalidStage) {
		t.Errorf("unknown stage: %v", err)
	}
	id = e.seed(2, models.StatusUploaded, nil)
	e.videos.Errs.Set("UpdateState", errBoom)
	if err := e.pc.Fail(ctx, id, "x"); !errors.Is(err, errBoom) {
		t.Errorf("UpdateState error: %v", err)
	}
}
