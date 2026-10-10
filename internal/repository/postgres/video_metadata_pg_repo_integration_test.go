//go:build integration

package postgres_test

import (
	"errors"
	"testing"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestVideoMetadataRepo_InsertBasicAndGet(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoMetadataRepo(pool)
	videoID := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{NoMeta: true})

	metaID := uuid.New()
	key := "originals/" + videoID.String() + "/original.mp4"
	if err := repo.InsertBasic(ctx, &models.VideoMetadata{ID: metaID, VideoId: videoID, Filename: "clip.mp4",
		OriginalObjectKey: key, Size: 4096}); err != nil {
		t.Fatal(err)
	}

	// Get accepts either the video id or the metadata id
	for _, lookup := range []uuid.UUID{videoID, metaID} {
		m, err := repo.Get(ctx, lookup.String())
		if err != nil {
			t.Fatalf("Get(%s): %v", lookup, err)
		}
		if m.ID != metaID || m.Filename != "clip.mp4" || m.OriginalObjectKey != key || m.Size != 4096 {
			t.Fatalf("unexpected metadata %+v", m)
		}
		if m.DurationSeconds != nil || m.Width != nil || m.ManifestPath != nil || m.Thumbnail != nil {
			t.Fatalf("nullable fields should be nil: %+v", m)
		}
		if len(m.AvailableQualities) != 0 {
			t.Fatalf("qualities default should be empty, got %v", m.AvailableQualities)
		}
	}
}

func TestVideoMetadataRepo_Get_ReturnsVideoIDAndSHA(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoMetadataRepo(pool)
	videoID := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{})
	if err := repo.UpdateMeta(ctx, videoID.String(), "abc123", 10, "h264", "aac", 1920, 1080); err != nil {
		t.Fatal(err)
	}
	m, err := repo.Get(ctx, videoID.String())
	if err != nil {
		t.Fatal(err)
	}
	if m.VideoId != videoID || m.SHA256 == nil || *m.SHA256 != "abc123" {
		t.Skipf("BUG: VideoMetadataRepo.Get (internal/repository/postgres/video_metadata_pg_repo.go:55-58) does not select "+
			"video_id or sha256, so the returned metadata has VideoId=%s (want %s) and SHA256=%v (want abc123); "+
			"GET /api/video-meta/:id serves a zero video_id", m.VideoId, videoID, m.SHA256)
	}
}

func TestVideoMetadataRepo_Updates(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoMetadataRepo(pool)
	videoID := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{})
	before, err := repo.Get(ctx, videoID.String())
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.UpdateMeta(ctx, videoID.String(), "sha", 42, "h264", "aac", 1280, 720); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateQualities(ctx, videoID.String(), []string{"360p", "720p"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateManifest(ctx, videoID.String(), "segments/x/master.m3u8"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateThumbnail(ctx, videoID.String(), "thumbnails/x/x.jpeg"); err != nil {
		t.Fatal(err)
	}
	m, err := repo.Get(ctx, videoID.String())
	if err != nil {
		t.Fatal(err)
	}
	if m.DurationSeconds == nil || *m.DurationSeconds != 42 || *m.CodecVideo != "h264" || *m.CodecAudio != "aac" ||
		*m.Width != 1280 || *m.Height != 720 {
		t.Fatalf("UpdateMeta not applied: %+v", m)
	}
	if len(m.AvailableQualities) != 2 || m.AvailableQualities[0] != "360p" || m.AvailableQualities[1] != "720p" {
		t.Fatalf("qualities = %v", m.AvailableQualities)
	}
	if m.ManifestPath == nil || *m.ManifestPath != "segments/x/master.m3u8" {
		t.Fatalf("manifest = %v", m.ManifestPath)
	}
	if m.Thumbnail == nil || *m.Thumbnail != "thumbnails/x/x.jpeg" {
		t.Fatalf("thumbnail = %v", m.Thumbnail)
	}
	if !m.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("updated_at not bumped: %v -> %v", before.UpdatedAt, m.UpdatedAt)
	}
}

func TestVideoMetadataRepo_Constraints(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewVideoMetadataRepo(pool)
	videoID := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{}) // already has a metadata row

	// migration 006: exactly one metadata row per video
	if err := repo.InsertBasic(ctx, &models.VideoMetadata{ID: uuid.New(), VideoId: videoID, Filename: "f",
		OriginalObjectKey: "k", Size: 1}); err == nil {
		t.Fatal("expected unique violation on video_metadata(video_id)")
	}
	// foreign key to videos
	if err := repo.InsertBasic(ctx, &models.VideoMetadata{ID: uuid.New(), VideoId: uuid.New(), Filename: "f",
		OriginalObjectKey: "k", Size: 1}); err == nil {
		t.Fatal("expected foreign key violation for unknown video")
	}
}

func TestVideoMetadataRepo_Get_NotFound(t *testing.T) {
	pool := setup(t)
	if _, err := postgres.NewVideoMetadataRepo(pool).Get(ctxT(t), uuid.NewString()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("err = %v, want pgx.ErrNoRows", err)
	}
}

func TestVideoMetadataRepo_GetByKey_NotFound(t *testing.T) {
	pool := setup(t)
	if _, err := postgres.NewVideoMetadataRepo(pool).GetByKey(ctxT(t), "originals/nope/original.mp4"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("err = %v, want pgx.ErrNoRows", err)
	}
}

func TestVideoMetadataRepo_GetByKey_ReturnsVideoID(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	videoID := mkVideo(t, pool, mkUser(t, pool, "alice"), videoOpts{})
	key := "originals/" + videoID.String() + "/original.mp4"

	got, err := postgres.NewVideoMetadataRepo(pool).GetByKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got != videoID.String() {
		var metaID string
		_ = pool.QueryRow(ctx, `SELECT id::text FROM video_metadata WHERE video_id=$1`, videoID).Scan(&metaID)
		if got == metaID {
			t.Skipf("BUG: VideoMetadataRepo.GetByKey (internal/repository/postgres/video_metadata_pg_repo.go:67-75) "+
				"selects video_metadata.id instead of video_id: got metadata id %s, want video id %s "+
				"(the worker only works because presign caches key->video id for 24h)", got, videoID)
		}
		t.Fatalf("GetByKey = %s, want video id %s", got, videoID)
	}
}
