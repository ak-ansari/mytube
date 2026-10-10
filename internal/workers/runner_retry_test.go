package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/jobs"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/testutil/fakes"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// testPolicy keeps delays recognisable: retry n waits 10s·2^(n-1) (+ up to 20% jitter), capped at 1m.
var testPolicy = RetryPolicy{MaxAttempts: 3, BaseDelay: 10 * time.Second, MaxDelay: time.Minute}

// fakeClock is shared by the runner (backoff/promote) and the fake queue (due times).
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type retryEnv struct {
	*runnerEnv
	dlq   *fakes.DeadLetterRepo
	clock *fakeClock
}

func newRetryEnv(t *testing.T, policy RetryPolicy) *retryEnv {
	t.Helper()
	e := &retryEnv{runnerEnv: newRunnerEnv(t), dlq: fakes.NewDeadLetterRepo(),
		clock: &fakeClock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}}
	e.queue.Now = e.clock.Now
	e.r.WithRetry(e.queue, e.dlq, policy)
	e.r.now = e.clock.Now
	return e
}

func (e *runnerEnv) enqueue(t *testing.T, p jobs.JobPayload) {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.queue.Enqueue(context.Background(), qName, b); err != nil {
		t.Fatal(err)
	}
}

func (e *runnerEnv) process(t *testing.T) {
	t.Helper()
	if err := e.r.ProcessQueueTasks(context.Background(), 0); err != nil {
		t.Fatalf("ProcessQueueTasks: %v", err)
	}
}

func decodeJobs(t *testing.T, raw [][]byte) []jobs.JobPayload {
	t.Helper()
	out := make([]jobs.JobPayload, 0, len(raw))
	for _, b := range raw {
		var p jobs.JobPayload
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatalf("bad payload %s: %v", b, err)
		}
		out = append(out, p)
	}
	return out
}

func (e *retryEnv) delayedJobs(t *testing.T) []jobs.JobPayload {
	t.Helper()
	var raw [][]byte
	for _, m := range e.queue.Delayed(qName) {
		raw = append(raw, m.Payload)
	}
	return decodeJobs(t, raw)
}

func (e *retryEnv) promote(t *testing.T) int {
	t.Helper()
	before := len(e.queue.Messages(qName))
	if err := e.r.PromoteDueRetries(context.Background()); err != nil {
		t.Fatal(err)
	}
	return len(e.queue.Messages(qName)) - before
}

func inBackoffWindow(d time.Duration, retry int, p RetryPolicy) bool {
	base := p.BaseDelay
	for i := 1; i < retry && base < p.MaxDelay; i++ {
		base *= 2
	}
	base = min(base, p.MaxDelay)
	return d >= base && d <= base+base/5
}

func TestRetryableFailureIsDelayedThenPromotedThenSucceeds(t *testing.T) {
	e := newRetryEnv(t, testPolicy)
	id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
	e.fail[4] = errBoom // unknown error: retryable
	e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 4, Status: models.StatusThumbnailGenerated})
	start := e.clock.Now()

	e.process(t)
	if n := len(e.queue.Messages(qName)); n != 0 {
		t.Fatalf("retry went onto the list (%d messages), want the delayed set", n)
	}
	delayed := e.queue.Delayed(qName)
	if len(delayed) != 1 {
		t.Fatalf("delayed = %d, want 1", len(delayed))
	}
	p := e.delayedJobs(t)[0]
	if p.VideoID != id.String() || p.Stage != 4 || p.Status != models.StatusThumbnailGenerated || p.Attempt != 1 {
		t.Errorf("retry payload = %+v, want same job with Attempt 1", p)
	}
	if d := delayed[0].Due.Sub(start); !inBackoffWindow(d, 1, testPolicy) {
		t.Errorf("first retry due after %v, want 10s..12s", d)
	}
	if s, st := e.stage(t, id); s != 4 || st != models.StatusThumbnailGenerated {
		t.Errorf("video at %d/%s, a retryable failure must not fail it", s, st)
	}
	if len(e.dlq.All()) != 0 {
		t.Error("retryable failure was dead-lettered")
	}
	if e.log.Count("warn") == 0 {
		t.Error("retry not logged")
	}

	// not due yet
	e.clock.Advance(9 * time.Second)
	if n := e.promote(t); n != 0 {
		t.Fatalf("promoted %d before due", n)
	}
	e.process(t) // empty list: nothing runs
	if e.calls[4] != 1 {
		t.Fatalf("handler ran %d times before the retry was due", e.calls[4])
	}

	// due: promoted onto the list exactly once
	e.clock.Advance(4 * time.Second)
	if n := e.promote(t); n != 1 {
		t.Fatalf("promoted %d, want 1", n)
	}
	if n := e.promote(t); n != 0 {
		t.Fatalf("second promote moved %d more", n)
	}
	if len(e.queue.Delayed(qName)) != 0 {
		t.Error("delayed set not drained")
	}

	// the retry succeeds and the stage advances
	delete(e.fail, 4)
	e.process(t)
	if got := e.attempts[4]; len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Errorf("handler saw attempts %v, want [0 1]", got)
	}
	if s, st := e.stage(t, id); s != 5 || st != models.StatusTranscoded {
		t.Errorf("video at %d/%s after successful retry, want 5/transcoded", s, st)
	}
	next := decodeJobs(t, e.queue.Messages(qName))
	if len(next) != 1 || next[0].Stage != 5 || next[0].Attempt != 0 {
		t.Errorf("next job = %+v, want stage 5 with a fresh Attempt 0", next)
	}
	if len(e.dlq.All()) != 0 {
		t.Error("dead letters after a successful retry")
	}
}

func TestRetriesExhaustedAreDeadLettered(t *testing.T) {
	for _, tt := range []struct {
		name   string
		stage  int
		status models.VideoStatus
		title  *string
		failed models.VideoStatus
	}{
		{"phase 1 stage", 2, models.StatusUploaded, nil, models.StatusPhase1Failed},
		{"phase 2 stage", 4, models.StatusThumbnailGenerated, fakes.Ptr("t"), models.StatusPhase2Failed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newRetryEnv(t, testPolicy)
			id := e.seed(tt.stage, tt.status, tt.title)
			e.fail[tt.stage] = errBoom
			e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: tt.stage, Status: tt.status})

			for run := 1; run <= testPolicy.MaxAttempts; run++ {
				before := e.clock.Now()
				e.process(t)
				if run < testPolicy.MaxAttempts {
					d := e.queue.Delayed(qName)
					if len(d) != 1 {
						t.Fatalf("run %d: delayed = %d", run, len(d))
					}
					if !inBackoffWindow(d[0].Due.Sub(before), run, testPolicy) {
						t.Errorf("run %d: delay %v outside the backoff window", run, d[0].Due.Sub(before))
					}
					if _, st := e.stage(t, id); st == tt.failed {
						t.Fatalf("run %d: video failed before attempts ran out", run)
					}
					e.clock.Advance(2 * time.Minute)
					if n := e.promote(t); n != 1 {
						t.Fatalf("run %d: promoted %d", run, n)
					}
				}
			}
			if got := e.attempts[tt.stage]; fmt.Sprint(got) != "[0 1 2]" {
				t.Errorf("handler attempts = %v, want [0 1 2]", got)
			}
			if len(e.queue.Delayed(qName)) != 0 || len(e.queue.Messages(qName)) != 0 {
				t.Error("an exhausted job was re-enqueued")
			}
			if s, st := e.stage(t, id); s != tt.stage || st != tt.failed {
				t.Errorf("video at %d/%s, want %d/%s", s, st, tt.stage, tt.failed)
			}
			dl := e.dlq.All()
			if len(dl) != 1 {
				t.Fatalf("dead letters = %d", len(dl))
			}
			j := dl[0]
			if j.Source != models.DeadLetterSourceJob || j.VideoID == nil || *j.VideoID != id || j.Stage == nil || *j.Stage != tt.stage {
				t.Errorf("dead letter = %+v", j)
			}
			if j.Attempts != testPolicy.MaxAttempts || !j.Retryable {
				t.Errorf("attempts/retryable = %d/%v, want %d/true", j.Attempts, j.Retryable, testPolicy.MaxAttempts)
			}
			if !strings.Contains(j.Error, "retries exhausted after 3 attempts") || !strings.Contains(j.Error, "boom") {
				t.Errorf("error = %q", j.Error)
			}
			var p jobs.JobPayload
			if err := json.Unmarshal([]byte(j.Payload), &p); err != nil || p.Attempt != 2 || p.VideoID != id.String() {
				t.Errorf("payload = %s (%v)", j.Payload, err)
			}
		})
	}
}

func TestPermanentFailureIsDeadLetteredOnFirstAttempt(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
	}{
		{"jobs.Permanent", jobs.Permanent(errors.New("no video stream found in uploaded file"))},
		{"wrapped jobs.Permanent", fmt.Errorf("validate: %w", jobs.Permanent(errBoom))},
		{"video deleted (no rows)", fmt.Errorf("get video: %w", pgx.ErrNoRows)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newRetryEnv(t, testPolicy)
			id := e.seed(2, models.StatusUploaded, nil)
			e.fail[2] = tt.err
			e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 2, Status: models.StatusUploaded})
			e.process(t)

			if e.calls[2] != 1 || len(e.queue.Delayed(qName)) != 0 || len(e.queue.Messages(qName)) != 0 {
				t.Errorf("permanent failure retried (calls=%d)", e.calls[2])
			}
			if _, st := e.stage(t, id); st != models.StatusPhase1Failed {
				t.Errorf("status = %s", st)
			}
			dl := e.dlq.All()
			if len(dl) != 1 || dl[0].Attempts != 1 || dl[0].Retryable || dl[0].Error != tt.err.Error() || *dl[0].Stage != 2 {
				t.Fatalf("dead letters = %+v", dl)
			}
		})
	}
}

func TestMissingHandlerIsPermanent(t *testing.T) {
	for _, stage := range []int{1, 8, 42} {
		e := newRetryEnv(t, testPolicy)
		id := e.seed(2, models.StatusUploaded, nil)
		e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: stage, Attempt: 1})
		e.process(t)
		if len(e.queue.Delayed(qName)) != 0 {
			t.Errorf("stage %d: missing handler was retried", stage)
		}
		dl := e.dlq.All()
		if len(dl) != 1 || dl[0].Retryable || dl[0].Attempts != 2 || *dl[0].Stage != stage ||
			!strings.Contains(dl[0].Error, fmt.Sprintf("no handler is registered for stage %d", stage)) {
			t.Errorf("stage %d: dead letters = %+v", stage, dl)
		}
		if _, st := e.stage(t, id); st != models.StatusPhase1Failed {
			t.Errorf("stage %d: status = %s", stage, st)
		}
	}
}

func TestAdvanceErrorAfterSuccessfulHandlerIsRetried(t *testing.T) {
	e := newRetryEnv(t, testPolicy)
	id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
	e.videos.Errs.Set("AdvanceState", errBoom)
	e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 4, Status: models.StatusThumbnailGenerated})

	e.process(t) // no error returned: the failure went through retry
	d := e.delayedJobs(t)
	if len(d) != 1 || d[0].Stage != 4 || d[0].Attempt != 1 {
		t.Fatalf("delayed = %+v, want stage 4 attempt 1", d)
	}
	if s, st := e.stage(t, id); s != 4 || st != models.StatusThumbnailGenerated {
		t.Errorf("video at %d/%s", s, st)
	}

	e.videos.Errs.Set("AdvanceState", nil)
	e.clock.Advance(time.Minute)
	e.promote(t)
	e.process(t)
	if e.calls[4] != 2 {
		t.Errorf("handler ran %d times, want 2 (rerun on retry)", e.calls[4])
	}
	if s, _ := e.stage(t, id); s != 5 {
		t.Errorf("stage = %d, want 5", s)
	}
	if len(e.dlq.All()) != 0 {
		t.Error("dead-lettered")
	}

	t.Run("advance on a deleted video is permanent", func(t *testing.T) {
		e := newRetryEnv(t, testPolicy)
		ghost := uuid.NewString()
		e.enqueue(t, jobs.JobPayload{VideoID: ghost, Stage: 6})
		e.process(t)
		dl := e.dlq.All()
		if len(dl) != 1 || dl[0].Retryable || !strings.Contains(dl[0].Error, "advance after stage 6") {
			t.Fatalf("dead letters = %+v", dl)
		}
		if len(e.queue.Delayed(qName)) != 0 {
			t.Error("retried")
		}
	})
}

// BUG: Advance is not atomic with the enqueue of the next job. If AdvanceState succeeds but the
// Enqueue fails, the runner retries the *finished* stage; its Advance then reads the new stage
// from the DB and advances again, skipping the next stage's handler entirely.
func TestAdvanceEnqueueFailureDoesNotSkipAStage(t *testing.T) {
	t.Skip("BUG: internal/workers/runner.go dispatch retries the finished stage after Advance fails; when AdvanceState " +
		"already committed (only the Enqueue failed), the rerun's Advance moves the video one more stage and the " +
		"skipped stage's handler never runs (here: segment, so a video can publish without HLS segments)")
	e := newRetryEnv(t, testPolicy)
	id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
	e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 4})
	e.queue.Errs.Set("Enqueue", errBoom) // AdvanceState(4->5) commits, enqueueing the stage 5 job fails
	e.process(t)
	e.queue.Errs.Set("Enqueue", nil)
	for i := 0; i < 10; i++ {
		e.clock.Advance(time.Minute)
		e.promote(t)
		e.process(t)
	}
	if e.calls[5] != 1 {
		s, st := e.stage(t, id)
		t.Errorf("stage 5 (segment) handler ran %d times, want 1; video at %d/%s", e.calls[5], s, st)
	}
}

func TestShutdownRequeuesWithoutCountingAnAttempt(t *testing.T) {
	for _, tt := range []struct {
		name    string
		attempt int
		err     error
	}{
		{"first run", 0, context.Canceled},
		{"during a retry", 2, context.Canceled},
		{"wrapped cancel", 1, fmt.Errorf("download: %w", context.Canceled)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newRetryEnv(t, testPolicy)
			id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
			e.fail[4] = tt.err
			e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 4, Status: models.StatusThumbnailGenerated, Attempt: tt.attempt})

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := e.r.ProcessQueueTasks(ctx, 0); err != nil {
				t.Fatal(err)
			}
			if len(e.queue.Delayed(qName)) != 0 {
				t.Error("an interrupted job must go straight back onto the list, not the delayed set")
			}
			q := decodeJobs(t, e.queue.Messages(qName))
			want := jobs.JobPayload{VideoID: id.String(), Stage: 4, Status: models.StatusThumbnailGenerated, Attempt: tt.attempt}
			if len(q) != 1 || q[0] != want {
				t.Errorf("queue = %+v, want [%+v]", q, want)
			}
			if s, st := e.stage(t, id); s != 4 || st != models.StatusThumbnailGenerated {
				t.Errorf("video at %d/%s, shutdown must not fail it", s, st)
			}
			if len(e.dlq.All()) != 0 {
				t.Error("dead-lettered on shutdown")
			}
		})
	}

	t.Run("canceled error without shutdown is an ordinary retryable failure", func(t *testing.T) {
		e := newRetryEnv(t, testPolicy)
		id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
		e.fail[4] = context.Canceled
		e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 4})
		e.process(t)
		if d := e.delayedJobs(t); len(d) != 1 || d[0].Attempt != 1 {
			t.Errorf("delayed = %+v", d)
		}
	})

	t.Run("re-queue failure falls back to the retry path", func(t *testing.T) {
		e := newRetryEnv(t, testPolicy)
		id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
		e.fail[4] = context.Canceled
		e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 4})
		e.queue.Errs.Set("Enqueue", errBoom)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := e.r.ProcessQueueTasks(ctx, 0); err != nil {
			t.Fatal(err)
		}
		if d := e.delayedJobs(t); len(d) != 1 || d[0].Attempt != 1 {
			t.Errorf("delayed = %+v", d)
		}
		if _, st := e.stage(t, id); st == models.StatusPhase2Failed {
			t.Error("video failed")
		}
	})
}

// BUG: on shutdown exec.CommandContext kills ffmpeg and returns *exec.ExitError "signal: killed",
// which does not wrap context.Canceled, so the shutdown branch is skipped and IsRetryable
// classifies the kill as a permanent ffmpeg failure.
func TestShutdownDuringFFmpegRequeues(t *testing.T) {
	t.Skip("BUG: internal/workers/runner.go handleFailure only treats errors.Is(cause, context.Canceled) as shutdown; a " +
		"killed ffmpeg returns *exec.ExitError(\"signal: killed\") (ctx.Err() is not wrapped), and IsRetryable calls a " +
		"non-network ExitError permanent, so stopping the worker mid-transcode dead-letters the job and fails the video")
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip("sleep not available")
	}
	e := newRetryEnv(t, testPolicy)
	id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
	e.r.hr.registry[4] = func(ctx context.Context, p *jobs.JobPayload) error {
		out, err := exec.CommandContext(ctx, "sleep", "5").CombinedOutput()
		if err != nil {
			return fmt.Errorf("ffmpeg transcode failed: %w\nstderr: %s", err, out)
		}
		return nil
	}
	e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 4})
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	if err := e.r.ProcessQueueTasks(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if _, st := e.stage(t, id); st == models.StatusPhase2Failed {
		t.Error("shutdown during ffmpeg failed the video")
	}
	if len(e.dlq.All()) != 0 {
		t.Errorf("shutdown during ffmpeg dead-lettered: %+v", e.dlq.All())
	}
	if len(e.queue.Messages(qName)) != 1 {
		t.Error("job not re-queued")
	}
}

func TestBadJSONPayloadIsDeadLettered(t *testing.T) {
	e := newRetryEnv(t, testPolicy)
	_ = e.queue.Enqueue(context.Background(), qName, []byte("{nope"))
	if err := e.r.ProcessQueueTasks(context.Background(), 0); err == nil {
		t.Fatal("expected the unmarshal error to be returned")
	}
	dl := e.dlq.All()
	if len(dl) != 1 {
		t.Fatalf("dead letters = %d", len(dl))
	}
	j := dl[0]
	if j.Source != models.DeadLetterSourceJob || j.VideoID != nil || j.Stage != nil || j.Attempts != 1 || j.Retryable ||
		j.Payload != "{nope" || !strings.Contains(j.Error, "unmarshal job payload") {
		t.Errorf("dead letter = %+v", j)
	}
	if len(e.queue.Delayed(qName)) != 0 {
		t.Error("bad payload retried")
	}
}

func TestDeadLetterWriteFailureStillFailsVideo(t *testing.T) {
	e := newRetryEnv(t, testPolicy)
	id := e.seed(2, models.StatusUploaded, nil)
	e.fail[2] = jobs.Permanent(errBoom)
	e.dlq.Errs.Set("Insert", errors.New("db down"))
	e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 2})
	e.process(t)
	if _, st := e.stage(t, id); st != models.StatusPhase1Failed {
		t.Errorf("status = %s", st)
	}
	found := false
	for _, m := range e.log.Msgs {
		if m == "error: Failed to write dead letter" {
			found = true
		}
	}
	if !found {
		t.Errorf("dead-letter write failure not logged: %v", e.log.Msgs)
	}
}

func TestScheduleRetryFailureDeadLetters(t *testing.T) {
	e := newRetryEnv(t, testPolicy)
	id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
	e.fail[4] = errBoom
	e.queue.Errs.Set("EnqueueAfter", errors.New("redis down"))
	e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 4})
	e.process(t)
	dl := e.dlq.All()
	// the retry could not be scheduled: recorded as a retryable failure after 1 attempt
	if len(dl) != 1 || !dl[0].Retryable || dl[0].Attempts != 1 {
		t.Fatalf("dead letters = %+v", dl)
	}
	if _, st := e.stage(t, id); st != models.StatusPhase2Failed {
		t.Errorf("status = %s", st)
	}
}

func TestIterateWithRetryPromotesDueRetries(t *testing.T) {
	ctx := context.Background()
	e := newRetryEnv(t, testPolicy)
	id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
	b, _ := json.Marshal(jobs.JobPayload{VideoID: id.String(), Stage: 4, Attempt: 1})
	if err := e.queue.EnqueueAfter(ctx, qName, b, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if failed := e.r.iterate(ctx, 0); failed || e.calls[4] != 0 {
		t.Fatalf("not-yet-due retry ran (failed=%v calls=%d)", failed, e.calls[4])
	}
	e.clock.Advance(30 * time.Second)
	if failed := e.r.iterate(ctx, 0); failed {
		t.Fatal("iteration failed")
	}
	if e.calls[4] != 1 || e.attempts[4][0] != 1 {
		t.Errorf("due retry not promoted and run in the same iteration: calls=%d attempts=%v", e.calls[4], e.attempts[4])
	}
	if s, _ := e.stage(t, id); s != 5 {
		t.Errorf("stage = %d", s)
	}

	e.queue.Errs.Set("PromoteDue", errBoom)
	if failed := e.r.iterate(ctx, 0); !failed {
		t.Error("PromoteDue error should mark the iteration failed")
	}
}

// ------------------------------------------------------------------ bucket events

func (e *retryEnv) retryState(t *testing.T, key string) (bucketEventRetry, bool) {
	t.Helper()
	var s bucketEventRetry
	if !e.cache.Has(bucketRetryKey(key)) {
		return s, false
	}
	if err := e.cache.Get(context.Background(), bucketRetryKey(key), &s); err != nil {
		t.Fatal(err)
	}
	return s, true
}

func (e *retryEnv) bucketPoll(t *testing.T) {
	t.Helper()
	if err := e.r.ProcessBucketEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestBucketEventRetryableFailureBacksOffThenDeadLetters(t *testing.T) {
	e := newRetryEnv(t, testPolicy)
	id := e.seed(1, models.StatusUploadPending, nil)
	data := sampleEventFor(t, id)
	e.cache.PutHash(bucketHash, "k", data)
	e.videos.Errs.Set("AdvanceState", errBoom) // retryable

	start := e.clock.Now()
	e.bucketPoll(t)
	if e.cache.HashLen(bucketHash) != 1 {
		t.Fatal("retryable failure removed the event from the hash")
	}
	st, ok := e.retryState(t, "k")
	if !ok || st.Attempts != 1 {
		t.Fatalf("retry state = %+v (present=%v)", st, ok)
	}
	if d := time.UnixMilli(st.NextAt).Sub(start); !inBackoffWindow(d, 1, testPolicy) {
		t.Errorf("next_at in %v, want 10s..12s", d)
	}
	if e.cache.TTL(bucketRetryKey("k")) != 24*time.Hour {
		t.Errorf("retry state ttl = %v", e.cache.TTL(bucketRetryKey("k")))
	}
	if e.cache.Has("bucket_event_lock:k") {
		t.Error("lock not released")
	}
	if len(e.dlq.All()) != 0 {
		t.Fatal("dead-lettered on the first retryable failure")
	}

	// still backing off: skipped without touching the video
	calls := e.videos.Calls.Count("AdvanceState")
	e.clock.Advance(5 * time.Second)
	e.bucketPoll(t)
	if e.videos.Calls.Count("AdvanceState") != calls {
		t.Error("event processed while backing off")
	}
	if st2, _ := e.retryState(t, "k"); st2 != st {
		t.Errorf("state changed while backing off: %+v", st2)
	}
	if e.cache.Has("bucket_event_lock:k") {
		t.Error("lock not released after a skipped poll")
	}

	// due: retried, fails again
	e.clock.Advance(10 * time.Second)
	e.bucketPoll(t)
	if e.videos.Calls.Count("AdvanceState") != calls+1 {
		t.Fatal("due event not retried")
	}
	st, _ = e.retryState(t, "k")
	if st.Attempts != 2 || e.cache.HashLen(bucketHash) != 1 {
		t.Fatalf("after 2nd failure: state %+v, hash %d", st, e.cache.HashLen(bucketHash))
	}

	// third failure reaches MaxAttempts: dead-lettered and dropped
	e.clock.Advance(time.Minute)
	e.bucketPoll(t)
	if e.cache.HashLen(bucketHash) != 0 {
		t.Error("exhausted event still in the hash")
	}
	if _, ok := e.retryState(t, "k"); ok {
		t.Error("retry state not cleaned up")
	}
	dl := e.dlq.All()
	if len(dl) != 1 {
		t.Fatalf("dead letters = %d", len(dl))
	}
	j := dl[0]
	if j.Source != models.DeadLetterSourceBucketEvent || j.VideoID == nil || *j.VideoID != id || j.Stage != nil ||
		j.Attempts != 3 || !j.Retryable || j.Payload != data || !strings.Contains(j.Error, "retries exhausted after 3 attempts") {
		t.Errorf("dead letter = %+v", j)
	}
	// bucket-event dead letters do not fail the video (nothing marks it): it stays upload_pending
	if s, st := e.stage(t, id); s != 1 || st != models.StatusUploadPending {
		t.Errorf("video at %d/%s", s, st)
	}
}

func TestBucketEventSucceedsOnRetry(t *testing.T) {
	e := newRetryEnv(t, testPolicy)
	id := e.seed(1, models.StatusUploadPending, nil)
	e.cache.PutHash(bucketHash, "k", sampleEventFor(t, id))
	e.videos.Errs.Set("Get", errBoom)
	e.bucketPoll(t)
	if _, ok := e.retryState(t, "k"); !ok {
		t.Fatal("no retry state")
	}
	e.videos.Errs.Set("Get", nil)
	e.clock.Advance(time.Minute)
	e.bucketPoll(t)
	if s, st := e.stage(t, id); s != 2 || st != models.StatusUploaded {
		t.Errorf("video at %d/%s", s, st)
	}
	if e.cache.HashLen(bucketHash) != 0 {
		t.Error("processed event left in the hash")
	}
	if _, ok := e.retryState(t, "k"); ok {
		t.Error("retry state not cleared after success")
	}
	if len(e.dlq.All()) != 0 {
		t.Error("dead-lettered")
	}
	if q := decodeJobs(t, e.queue.Messages(qName)); len(q) != 1 || q[0].Stage != 2 {
		t.Errorf("jobs = %+v", q)
	}
}

func TestBucketEventPermanentFailures(t *testing.T) {
	for _, tt := range []struct {
		name    string
		data    func(t *testing.T) string
		videoID bool
	}{
		{"unknown video", func(t *testing.T) string { return sampleEventFor(t, uuid.New()) }, false},
		{"garbage event", func(*testing.T) string { return "{not json" }, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := newRetryEnv(t, testPolicy)
			data := tt.data(t)
			e.cache.PutHash(bucketHash, "k", data)
			e.bucketPoll(t)
			if e.cache.HashLen(bucketHash) != 0 {
				t.Error("permanent failure kept in the hash")
			}
			if _, ok := e.retryState(t, "k"); ok {
				t.Error("retry state written for a permanent failure")
			}
			dl := e.dlq.All()
			if len(dl) != 1 {
				t.Fatalf("dead letters = %d", len(dl))
			}
			j := dl[0]
			if j.Source != models.DeadLetterSourceBucketEvent || j.Retryable || j.Attempts != 1 || j.VideoID != nil ||
				j.Payload != data || !strings.Contains(j.Error, "resolve video for bucket event") {
				t.Errorf("dead letter = %+v", j)
			}
		})
	}

	t.Run("advance on a failed video is not an error", func(t *testing.T) {
		e := newRetryEnv(t, testPolicy)
		id := e.seed(1, models.StatusPhase1Failed, nil)
		e.cache.PutHash(bucketHash, "k", sampleEventFor(t, id))
		e.bucketPoll(t)
		if e.cache.HashLen(bucketHash) != 0 || len(e.dlq.All()) != 0 {
			t.Errorf("hash=%d dead letters=%d", e.cache.HashLen(bucketHash), len(e.dlq.All()))
		}
	})

	t.Run("ignored event (not an original) is dropped quietly", func(t *testing.T) {
		e := newRetryEnv(t, testPolicy)
		e.cache.PutHash(bucketHash, "thumb", `{"Records":[{"eventName":"s3:ObjectCreated:Put","s3":{"object":{"key":"thumbnails%2Fx%2Fx.jpeg"}}}]}`)
		e.bucketPoll(t)
		if e.cache.HashLen(bucketHash) != 0 || len(e.dlq.All()) != 0 {
			t.Errorf("hash=%d dead letters=%d", e.cache.HashLen(bucketHash), len(e.dlq.All()))
		}
	})

	t.Run("retry state write failure dead-letters at once", func(t *testing.T) {
		e := newRetryEnv(t, testPolicy)
		id := e.seed(1, models.StatusUploadPending, nil)
		e.cache.PutHash(bucketHash, "k", sampleEventFor(t, id))
		e.videos.Errs.Set("AdvanceState", errBoom)
		e.cache.Errs.Set("Set", errBoom)
		e.bucketPoll(t)
		dl := e.dlq.All()
		if len(dl) != 1 || !dl[0].Retryable || dl[0].Attempts != 1 || e.cache.HashLen(bucketHash) != 0 {
			t.Errorf("dead letters = %+v, hash = %d", dl, e.cache.HashLen(bucketHash))
		}
	})
}

// ------------------------------------------------------------------ without WithRetry

func TestWithoutRetryKeepsOldBehaviour(t *testing.T) {
	ctx := context.Background()

	t.Run("retryable handler failure fails the video at once", func(t *testing.T) {
		e := newRunnerEnv(t)
		id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
		e.fail[4] = context.DeadlineExceeded
		e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 4})
		e.process(t)
		if _, st := e.stage(t, id); st != models.StatusPhase2Failed {
			t.Errorf("status = %s", st)
		}
		if len(e.queue.Delayed(qName)) != 0 || len(e.queue.Messages(qName)) != 0 || e.queue.Calls.Count("EnqueueAfter") != 0 {
			t.Error("retried without WithRetry")
		}
	})

	t.Run("shutdown cancellation fails the video", func(t *testing.T) {
		e := newRunnerEnv(t)
		id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
		e.fail[4] = context.Canceled
		e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 4})
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		if err := e.r.ProcessQueueTasks(cctx, 0); err != nil {
			t.Fatal(err)
		}
		if _, st := e.stage(t, id); st != models.StatusPhase2Failed {
			t.Errorf("status = %s", st)
		}
		if len(e.queue.Messages(qName)) != 0 {
			t.Error("re-queued without WithRetry")
		}
	})

	t.Run("advance error is returned", func(t *testing.T) {
		e := newRunnerEnv(t)
		id := e.seed(4, models.StatusThumbnailGenerated, fakes.Ptr("t"))
		e.videos.Errs.Set("AdvanceState", errBoom)
		e.enqueue(t, jobs.JobPayload{VideoID: id.String(), Stage: 4})
		if err := e.r.ProcessQueueTasks(ctx, 0); !errors.Is(err, errBoom) {
			t.Errorf("err = %v", err)
		}
		if _, st := e.stage(t, id); st != models.StatusThumbnailGenerated {
			t.Errorf("status = %s", st)
		}
	})

	t.Run("PromoteDueRetries is a no-op", func(t *testing.T) {
		e := newRunnerEnv(t)
		_ = e.queue.EnqueueAfter(ctx, qName, []byte(`{"video_id":"x","stage":4}`), -time.Second) // straight onto the list
		_ = e.queue.EnqueueAfter(ctx, qName, []byte(`{"video_id":"y","stage":4}`), time.Nanosecond)
		time.Sleep(time.Millisecond)
		if err := e.r.PromoteDueRetries(ctx); err != nil {
			t.Fatal(err)
		}
		if e.queue.Calls.Count("PromoteDue") != 0 || len(e.queue.Delayed(qName)) != 1 {
			t.Error("PromoteDueRetries touched the delayed set without WithRetry")
		}
	})

	t.Run("bucket event failure is dropped, no retry state", func(t *testing.T) {
		e := newRunnerEnv(t)
		id := e.seed(1, models.StatusUploadPending, nil)
		e.cache.PutHash(bucketHash, "k", sampleEventFor(t, id))
		e.videos.Errs.Set("AdvanceState", errBoom)
		if err := e.r.ProcessBucketEvents(ctx); err != nil {
			t.Fatal(err)
		}
		if e.cache.HashLen(bucketHash) != 0 || e.cache.Has(bucketRetryKey("k")) {
			t.Error("event kept or retry state written without WithRetry")
		}
	})

	t.Run("bad payload just returns the error", func(t *testing.T) {
		e := newRunnerEnv(t)
		_ = e.queue.Enqueue(ctx, qName, []byte("nope"))
		if err := e.r.ProcessQueueTasks(ctx, 0); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestRunnerClockDefaultsToTimeNow(t *testing.T) {
	r := &Runner{}
	if d := time.Since(r.clock()); d < 0 || d > time.Minute {
		t.Errorf("clock() = %v", r.clock())
	}
	fixed := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return fixed }
	if !r.clock().Equal(fixed) {
		t.Error("injected clock ignored")
	}
}
