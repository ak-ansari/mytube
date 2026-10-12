//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var longAgo = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func updatedAt(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) time.Time {
	t.Helper()
	var ts time.Time
	if err := pool.QueryRow(context.Background(), `SELECT updated_at FROM videos WHERE id=$1`, id).Scan(&ts); err != nil {
		t.Fatal(err)
	}
	return ts
}

// resetUpdatedAt backdates updated_at; the UPDATE touches neither stage nor status, so the trigger stays quiet.
func resetUpdatedAt(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	exec(t, pool, `UPDATE videos SET updated_at=$2 WHERE id=$1`, id, longAgo)
	if got := updatedAt(t, pool, id); !got.Equal(longAgo) {
		t.Fatalf("could not backdate updated_at: %v", got)
	}
}

// migration 008: stage/status changes bump videos.updated_at, other writes do not.
func TestVideosTouchUpdatedAtTrigger(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	videos := postgres.NewVideoRepo(pool)
	catalog := postgres.NewVideoCatalogRepo(pool)
	id := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{Status: models.StatusUploadPending})
	exec(t, pool, `UPDATE videos SET stage=1 WHERE id=$1`, id)

	bumped := func(name string) {
		t.Helper()
		if got := updatedAt(t, pool, id); !got.After(longAgo.Add(time.Hour)) || time.Since(got) > time.Minute {
			t.Errorf("%s: updated_at = %v, want ~now", name, got)
		}
		resetUpdatedAt(t, pool, id)
	}
	untouched := func(name string) {
		t.Helper()
		if got := updatedAt(t, pool, id); !got.Equal(longAgo) {
			t.Errorf("%s: updated_at = %v, want unchanged %v", name, got, longAgo)
		}
	}

	resetUpdatedAt(t, pool, id)
	ok, err := videos.AdvanceState(ctx, id.String(), 1, 2, models.StatusUploaded)
	if err != nil || !ok {
		t.Fatalf("AdvanceState: %v %v", ok, err)
	}
	bumped("AdvanceState")

	if err := videos.UpdateState(ctx, id.String(), 2, models.StatusPhase1Failed); err != nil {
		t.Fatal(err)
	}
	bumped("UpdateState status only")

	exec(t, pool, `UPDATE videos SET stage=3 WHERE id=$1`, id)
	bumped("raw stage only")

	// writes that leave stage and status as they are
	exec(t, pool, `UPDATE videos SET views = views + 5 WHERE id=$1`, id)
	untouched("views")
	if err := catalog.IncrementViews(ctx, id); err != nil {
		t.Fatal(err)
	}
	untouched("IncrementViews")
	exec(t, pool, `UPDATE videos SET title='Renamed', description='new', visibility='unlisted', thumbnail='thumbnails/x/x.jpeg' WHERE id=$1`, id)
	untouched("title/description/visibility/thumbnail")
	if err := videos.UpdateVideo(ctx, &models.Video{ID: id, Title: ptr("Confirmed")}); err != nil {
		t.Fatal(err)
	}
	untouched("VideoRepo.UpdateVideo (confirm)")
	exec(t, pool, `UPDATE videos SET stage=stage, status=status WHERE id=$1`, id)
	untouched("stage/status set to the same values")
	if ok, err := videos.AdvanceState(ctx, id.String(), 7, 8, models.StatusPublished); err != nil || ok {
		t.Fatalf("losing AdvanceState = %v %v", ok, err)
	}
	untouched("AdvanceState that matched no row")

	// an explicit updated_at in the same statement as a stage change is overridden by the trigger
	exec(t, pool, `UPDATE videos SET stage=4, updated_at=$2 WHERE id=$1`, id, longAgo.Add(-time.Hour))
	bumped("stage change with explicit updated_at")

	// other videos are not touched
	other := mkVideo(t, pool, mkUser(t, pool, "bob"), videoOpts{})
	resetUpdatedAt(t, pool, other)
	exec(t, pool, `UPDATE videos SET stage=5 WHERE id=$1`, id)
	if got := updatedAt(t, pool, other); !got.Equal(longAgo) {
		t.Errorf("unrelated video bumped: %v", got)
	}
}
