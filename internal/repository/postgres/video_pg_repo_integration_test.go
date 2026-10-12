//go:build integration

package postgres_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestVideoRepo_InsertBasicAndGet(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoRepo(pool)
	owner := mkUser(t, pool, "alice")

	id := uuid.New()
	if err := repo.InsertBasic(ctx, &models.Video{ID: id, UserID: owner, FileKey: "originals/x/original.mp4",
		Status: models.StatusUploadPending, Stage: 1}); err != nil {
		t.Fatal(err)
	}
	v, err := repo.Get(ctx, id.String())
	if err != nil {
		t.Fatal(err)
	}
	if v.ID != id || v.UserID != owner || v.FileKey != "originals/x/original.mp4" {
		t.Fatalf("unexpected video %+v", v)
	}
	if v.Status != models.StatusUploadPending || v.Stage != 1 {
		t.Fatalf("status/stage = %s/%d", v.Status, v.Stage)
	}
	if v.Title != nil || v.Description != nil || v.Thumbnail != nil || v.Visibility != nil {
		t.Fatalf("nullable fields should be nil: %+v", v)
	}
	if v.CreatedAt.IsZero() || v.UpdatedAt.IsZero() {
		t.Fatal("timestamps not set")
	}
}

func TestVideoRepo_InsertBasic_UnknownUserFails(t *testing.T) {
	pool := setup(t)
	err := postgres.NewVideoRepo(pool).InsertBasic(ctxT(t), &models.Video{ID: uuid.New(), UserID: uuid.New(),
		FileKey: "k", Status: models.StatusUploadPending, Stage: 1})
	if err == nil {
		t.Fatal("expected foreign key violation for unknown user")
	}
}

func TestVideoRepo_Get_NotFound(t *testing.T) {
	pool := setup(t)
	repo := postgres.NewVideoRepo(pool)
	if _, err := repo.Get(ctxT(t), uuid.NewString()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("err = %v, want pgx.ErrNoRows", err)
	}
	// an unparsable id becomes uuid.Nil and is simply not found
	if _, err := repo.Get(ctxT(t), "not-a-uuid"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("err = %v, want pgx.ErrNoRows", err)
	}
}

func TestVideoRepo_UpdateVideo(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoRepo(pool)
	id := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{Status: models.StatusUploadPending})

	vis := models.VisibilityUnlisted
	if err := repo.UpdateVideo(ctx, &models.Video{ID: id, Title: ptr("New title"), Thumbnail: ptr("thumbnails/x/t.jpeg"),
		Description: ptr("New desc"), Visibility: &vis}); err != nil {
		t.Fatal(err)
	}
	v, err := repo.Get(ctx, id.String())
	if err != nil {
		t.Fatal(err)
	}
	if *v.Title != "New title" || *v.Thumbnail != "thumbnails/x/t.jpeg" || *v.Description != "New desc" || *v.Visibility != vis {
		t.Fatalf("not updated: %+v", v)
	}

	// UpdateVideo overwrites every field, nil included
	if err := repo.UpdateVideo(ctx, &models.Video{ID: id, Title: ptr("Only title")}); err != nil {
		t.Fatal(err)
	}
	v, _ = repo.Get(ctx, id.String())
	if v.Description != nil || v.Thumbnail != nil || v.Visibility != nil {
		t.Fatalf("expected nil fields after full overwrite: %+v", v)
	}

	bad := models.VideoVisibility("everyone")
	if err := repo.UpdateVideo(ctx, &models.Video{ID: id, Title: ptr("t"), Visibility: &bad}); err == nil {
		t.Fatal("expected visibility check constraint violation")
	}
}

func TestVideoRepo_GetByIds(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoRepo(pool)
	owner := mkUser(t, pool, "alice")
	a := mkVideo(t, pool, owner, videoOpts{})
	b := mkVideo(t, pool, owner, videoOpts{})
	mkVideo(t, pool, owner, videoOpts{})

	got, err := repo.GetByIds(ctx, []string{a.String(), b.String(), uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[uuid.UUID]bool{}
	for _, v := range got {
		seen[v.ID] = true
	}
	if len(got) != 2 || !seen[a] || !seen[b] {
		t.Fatalf("got %d videos %v, want a and b", len(got), seen)
	}

	got, err = repo.GetByIds(ctx, []string{})
	if err != nil || len(got) != 0 {
		t.Fatalf("empty ids: %v, %v", got, err)
	}
	if _, err := repo.GetByIds(ctx, []string{"bogus"}); err == nil {
		t.Fatal("expected parse error for invalid id")
	}
}

func TestVideoRepo_UpdateState(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoRepo(pool)
	id := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{Status: models.StatusUploadPending})

	if err := repo.UpdateState(ctx, id.String(), 5, models.StatusTranscoded); err != nil {
		t.Fatal(err)
	}
	v, _ := repo.Get(ctx, id.String())
	if v.Stage != 5 || v.Status != models.StatusTranscoded {
		t.Fatalf("stage/status = %d/%s", v.Stage, v.Status)
	}
	if err := repo.UpdateState(ctx, id.String(), 5, models.VideoStatus("bogus")); err == nil {
		t.Fatal("expected status check constraint violation")
	}
}

func TestVideoRepo_AdvanceState(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoRepo(pool)
	id := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{Status: models.StatusUploadPending})
	exec(t, pool, `UPDATE videos SET stage=1 WHERE id=$1`, id)

	ok, err := repo.AdvanceState(ctx, id.String(), 1, 2, models.StatusUploaded)
	if err != nil || !ok {
		t.Fatalf("advance 1->2: ok=%v err=%v", ok, err)
	}
	// stale from-stage is a no-op
	ok, err = repo.AdvanceState(ctx, id.String(), 1, 2, models.StatusUploaded)
	if err != nil || ok {
		t.Fatalf("second advance 1->2: ok=%v err=%v, want false", ok, err)
	}
	v, _ := repo.Get(ctx, id.String())
	if v.Stage != 2 || v.Status != models.StatusUploaded {
		t.Fatalf("stage/status = %d/%s", v.Stage, v.Status)
	}

	// failed videos keep their stage but must never advance
	for _, failed := range []models.VideoStatus{models.StatusPhase1Failed, models.StatusPhase2Failed} {
		if err := repo.UpdateState(ctx, id.String(), 2, failed); err != nil {
			t.Fatal(err)
		}
		ok, err = repo.AdvanceState(ctx, id.String(), 2, 3, models.StatusValid)
		if err != nil || ok {
			t.Fatalf("advance of %s video: ok=%v err=%v, want false", failed, ok, err)
		}
	}

	// unknown video
	ok, err = repo.AdvanceState(ctx, uuid.NewString(), 1, 2, models.StatusUploaded)
	if err != nil || ok {
		t.Fatalf("unknown video: ok=%v err=%v", ok, err)
	}
}

// AdvanceState is the pipeline's only guard against double processing: N concurrent callers
// racing the same transition must produce exactly one winner, for every stage.
func TestVideoRepo_AdvanceState_ConcurrentExactlyOneWins(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoRepo(pool)
	id := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{Status: models.StatusUploadPending})
	exec(t, pool, `UPDATE videos SET stage=1 WHERE id=$1`, id)

	const workers = 32
	for from := 1; from < 8; from++ {
		to := from + 1
		status := models.Stages[to].Name
		var wins, errs atomic.Int32
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				ok, err := repo.AdvanceState(ctx, id.String(), from, to, status)
				if err != nil {
					errs.Add(1)
					return
				}
				if ok {
					wins.Add(1)
				}
			}()
		}
		close(start)
		wg.Wait()
		if errs.Load() != 0 {
			t.Fatalf("stage %d->%d: %d errors", from, to, errs.Load())
		}
		if wins.Load() != 1 {
			t.Fatalf("stage %d->%d: %d winners, want exactly 1", from, to, wins.Load())
		}
		v, err := repo.Get(ctx, id.String())
		if err != nil {
			t.Fatal(err)
		}
		if v.Stage != to || v.Status != status {
			t.Fatalf("after %d->%d: stage/status = %d/%s", from, to, v.Stage, v.Status)
		}
	}
}
