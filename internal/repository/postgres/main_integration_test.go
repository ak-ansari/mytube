//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/ak-ansari/mytube/internal/testutil/integration"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pg is a throwaway database (mytube_test by default) recreated and migrated via the app's own
// db.NewPool (goose) once per package run. Every test truncates all tables first.
var pg *integration.PGPool

func TestMain(m *testing.M) {
	pg = integration.SetupPG(integration.TestDBName())
	code := m.Run()
	pg.Close()
	os.Exit(code)
}

// setup returns a pool on an empty test database, skipping when Postgres is unreachable.
func setup(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := pg.Require(t)
	integration.Truncate(t, pool)
	return pool
}

func ctxT(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func ptr[T any](v T) *T { return &v }

func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func count(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", sql, err)
	}
	return n
}

// mkUser creates a user through UserRepo.Upsert.
func mkUser(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := postgres.NewUserRepo(pool).Upsert(context.Background(), &models.User{
		ID: id, UserName: name, ChannelName: name + " channel", Email: name + "@example.test",
	})
	if err != nil {
		t.Fatalf("upsert user %s: %v", name, err)
	}
	return id
}

type videoOpts struct {
	Status     models.VideoStatus      // default published
	Visibility *models.VideoVisibility // default public; use noVisibility for NULL
	Title      string
	CreatedAt  time.Time // default now
	NoMeta     bool      // skip the video_metadata row
	MetaThumb  string
	Duration   int
}

var noVisibility = ptr(models.VideoVisibility(""))

// mkVideo inserts a video through VideoRepo.InsertBasic (+ VideoMetadataRepo.InsertBasic),
// then moves it to the requested status/visibility with plain SQL.
func mkVideo(t *testing.T, pool *pgxpool.Pool, owner uuid.UUID, o videoOpts) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	key := "originals/" + id.String() + "/original.mp4"
	if err := postgres.NewVideoRepo(pool).InsertBasic(ctx, &models.Video{
		ID: id, UserID: owner, FileKey: key, Status: models.StatusUploadPending, Stage: 1,
	}); err != nil {
		t.Fatalf("insert video: %v", err)
	}
	if !o.NoMeta {
		if err := postgres.NewVideoMetadataRepo(pool).InsertBasic(ctx, &models.VideoMetadata{
			ID: uuid.New(), VideoId: id, Filename: "original.mp4", OriginalObjectKey: key, Size: 1234,
		}); err != nil {
			t.Fatalf("insert metadata: %v", err)
		}
		if o.MetaThumb != "" {
			exec(t, pool, `UPDATE video_metadata SET thumbnail=$2 WHERE video_id=$1`, id, o.MetaThumb)
		}
		if o.Duration != 0 {
			exec(t, pool, `UPDATE video_metadata SET duration_seconds=$2 WHERE video_id=$1`, id, o.Duration)
		}
	}
	status := o.Status
	if status == "" {
		status = models.StatusPublished
	}
	var vis any = string(models.VisibilityPublic)
	if o.Visibility != nil {
		if *o.Visibility == "" {
			vis = nil
		} else {
			vis = string(*o.Visibility)
		}
	}
	title := o.Title
	if title == "" {
		title = "video " + id.String()[:8]
	}
	created := o.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	exec(t, pool, `UPDATE videos SET status=$2, visibility=$3, title=$4, description='desc', created_at=$5 WHERE id=$1`,
		id, status, vis, title, created)
	return id
}

func cardIDs(cards []*models.VideoCard) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(cards))
	for _, c := range cards {
		ids = append(ids, c.ID)
	}
	return ids
}

func equalIDs(t *testing.T, got, want []uuid.UUID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d ids %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("id[%d] = %s, want %s (got %v want %v)", i, got[i], want[i], got, want)
		}
	}
}
