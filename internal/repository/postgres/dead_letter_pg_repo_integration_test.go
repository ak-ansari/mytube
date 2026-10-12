//go:build integration

package postgres_test

import (
	"strings"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/google/uuid"
)

var _ repository.DeadLetterRepository = (*postgres.DeadLetterRepo)(nil)

func TestDeadLetterRepo_InsertAndList(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewDeadLetterRepo(pool)
	videoID := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{})

	// no video and no stage (e.g. an unparsable job payload or bucket event)
	orphan := &models.DeadLetterJob{Source: models.DeadLetterSourceBucketEvent, Attempts: 1, Error: "resolve video: no rows", Payload: "{not json"}
	before := time.Now().Add(-time.Minute)
	if err := repo.Insert(ctx, orphan); err != nil {
		t.Fatalf("insert with nil video_id/stage: %v", err)
	}
	if orphan.ID == uuid.Nil {
		t.Error("Insert should assign an id")
	}
	if orphan.CreatedAt.Before(before) || orphan.CreatedAt.After(time.Now().Add(time.Minute)) {
		t.Errorf("created_at = %v", orphan.CreatedAt)
	}
	var vid, stage any
	if err := pool.QueryRow(ctx, `SELECT video_id, stage FROM dead_letter_jobs WHERE id=$1`, orphan.ID).Scan(&vid, &stage); err != nil || vid != nil || stage != nil {
		t.Errorf("orphan row video_id=%v stage=%v err=%v, want NULLs", vid, stage, err)
	}

	// two entries for the video, inserted in order
	explicitID := uuid.New()
	first := &models.DeadLetterJob{ID: explicitID, Source: models.DeadLetterSourceJob, VideoID: &videoID, Stage: ptr(4),
		Attempts: 4, Retryable: true, Error: "retries exhausted after 4 attempts: boom", Payload: `{"video_id":"x","stage":4,"attempt":3}`}
	if err := repo.Insert(ctx, first); err != nil {
		t.Fatal(err)
	}
	if first.ID != explicitID {
		t.Errorf("explicit id replaced: %s", first.ID)
	}
	time.Sleep(5 * time.Millisecond)
	long := strings.Repeat("e", 20_000) // error/payload are TEXT: no length limit
	second := &models.DeadLetterJob{Source: models.DeadLetterSourceJob, VideoID: &videoID, Stage: ptr(2),
		Attempts: 1, Retryable: false, Error: long, Payload: long}
	if err := repo.Insert(ctx, second); err != nil {
		t.Fatal(err)
	}

	got, err := repo.ListByVideo(ctx, videoID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != second.ID || got[1].ID != first.ID {
		t.Fatalf("ListByVideo = %+v, want [second, first] (newest first)", got)
	}
	g := got[1]
	if g.Source != models.DeadLetterSourceJob || g.VideoID == nil || *g.VideoID != videoID || g.Stage == nil || *g.Stage != 4 ||
		g.Attempts != 4 || !g.Retryable || g.Error != first.Error || g.Payload != first.Payload || !g.CreatedAt.Equal(first.CreatedAt) {
		t.Errorf("round trip = %+v, want %+v", g, first)
	}
	if got[0].Retryable || got[0].Error != long || got[0].Payload != long {
		t.Error("second entry did not round-trip")
	}

	// unknown video: empty, non-nil
	none, err := repo.ListByVideo(ctx, uuid.New())
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("unknown video = %v, %v", none, err)
	}

	// no FK: the log survives the video being deleted
	exec(t, pool, `DELETE FROM videos WHERE id=$1`, videoID)
	if n := count(t, pool, `SELECT COUNT(*) FROM videos WHERE id=$1`, videoID); n != 0 {
		t.Fatal("video not deleted")
	}
	after, err := repo.ListByVideo(ctx, videoID)
	if err != nil || len(after) != 2 {
		t.Fatalf("after video delete: %d entries, %v", len(after), err)
	}
	if n := count(t, pool, `SELECT COUNT(*) FROM dead_letter_jobs`); n != 3 {
		t.Errorf("total rows = %d, want 3", n)
	}
}

func TestDeadLetterRepo_Constraints(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewDeadLetterRepo(pool)

	if err := repo.Insert(ctx, &models.DeadLetterJob{Source: "cron", Attempts: 1, Error: "e", Payload: "p"}); err == nil {
		t.Error("source outside ('job','bucket_event') should violate the CHECK constraint")
	}
	if err := repo.Insert(ctx, &models.DeadLetterJob{Source: models.DeadLetterSourceJob, Attempts: 1, Error: "e"}); err != nil {
		t.Errorf("empty payload is allowed (NOT NULL only): %v", err)
	}
	dup := uuid.New()
	if err := repo.Insert(ctx, &models.DeadLetterJob{ID: dup, Source: models.DeadLetterSourceJob, Attempts: 1, Error: "e", Payload: "p"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Insert(ctx, &models.DeadLetterJob{ID: dup, Source: models.DeadLetterSourceJob, Attempts: 1, Error: "e", Payload: "p"}); err == nil {
		t.Error("duplicate id should fail")
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		t.Fatal(ctxErr)
	}
}
