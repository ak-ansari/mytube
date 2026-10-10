//go:build integration

package db_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/db"
	"github.com/ak-ansari/mytube/internal/testutil/integration"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

// a database of its own, so this package never races the repository tests
func migrationsDBName() string { return integration.TestDBName() + "_migrations" }

func migrationFiles(t *testing.T) []string {
	t.Helper()
	root, err := integration.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(root, "migrations", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	return files
}

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := integration.MigratedPool(ctx, migrationsDBName())
	if errors.Is(err, integration.ErrUnavailable) {
		t.Skipf("postgres not reachable: %v", err)
	}
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		_ = integration.DropDatabase(context.Background(), migrationsDBName())
	})
	return pool
}

func scalar[T any](t *testing.T, pool *pgxpool.Pool, q string, args ...any) T {
	t.Helper()
	var v T
	if err := pool.QueryRow(context.Background(), q, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return v
}

func TestNewPool_AppliesAllMigrations(t *testing.T) {
	pool := newPool(t)
	files := migrationFiles(t)

	version := scalar[int64](t, pool, `SELECT MAX(version_id) FROM goose_db_version WHERE is_applied`)
	if int(version) != len(files) {
		t.Fatalf("goose version = %d, want %d (one per file in ./migrations)", version, len(files))
	}

	for _, table := range integration.AppTables {
		if !scalar[bool](t, pool, `SELECT to_regclass($1) IS NOT NULL`, "public."+table) {
			t.Errorf("table %s missing", table)
		}
	}

	columns := map[string][]string{
		"users":            {"email", "description"},                                                                         // 004
		"videos":           {"views", "stage", "status"},                                                                     // 002, 005
		"video_metadata":   {"available_qualities"},                                                                          // 003
		"dead_letter_jobs": {"id", "source", "video_id", "stage", "attempts", "retryable", "error", "payload", "created_at"}, // 007
	}
	for table, cols := range columns {
		for _, col := range cols {
			if !scalar[bool](t, pool, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name=$1 AND column_name=$2)`, table, col) {
				t.Errorf("column %s.%s missing", table, col)
			}
		}
	}

	// 006: free-text btree indexes dropped, unique metadata-per-video index added
	for _, idx := range []string{"idx_videos_title", "idx_videos_description"} {
		if scalar[bool](t, pool, `SELECT to_regclass($1) IS NOT NULL`, "public."+idx) {
			t.Errorf("index %s should have been dropped by 006", idx)
		}
	}
	if !scalar[bool](t, pool, `SELECT indisunique FROM pg_index WHERE indexrelid = 'public.idx_video_metadata_video_id'::regclass`) {
		t.Error("idx_video_metadata_video_id should be unique")
	}

	// 007: dead-letter indexes, and no foreign key so entries outlive their video
	for _, idx := range []string{"idx_dead_letter_jobs_video_id", "idx_dead_letter_jobs_created_at"} {
		if !scalar[bool](t, pool, `SELECT to_regclass($1) IS NOT NULL`, "public."+idx) {
			t.Errorf("index %s missing", idx)
		}
	}
	if n := scalar[int64](t, pool, `SELECT COUNT(*) FROM pg_constraint WHERE conrelid = 'public.dead_letter_jobs'::regclass AND contype = 'f'`); n != 0 {
		t.Errorf("dead_letter_jobs has %d foreign keys, want none", n)
	}

	// 008: updated_at trigger on videos
	if !scalar[bool](t, pool, `SELECT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_videos_touch_updated_at' AND tgrelid = 'public.videos'::regclass AND NOT tgisinternal)`) {
		t.Error("trigger trg_videos_touch_updated_at missing")
	}

	// 006 lifted the btree size limit on titles: a > 2.7KB title must insert fine
	long := make([]byte, 10_000)
	for i := range long {
		long[i] = byte('a' + i%26)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO users (id, user_name) VALUES ('00000000-0000-0000-0000-000000000001', 'u')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), `INSERT INTO videos (id, user_id, file_key, title, description) VALUES
		('00000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000001', 'k', $1, $1)`, string(long)); err != nil {
		t.Fatalf("long title insert: %v", err)
	}
}

func TestNewPool_IsIdempotent(t *testing.T) {
	pool := newPool(t)
	before := scalar[int64](t, pool, `SELECT COUNT(*) FROM goose_db_version`)

	// a second startup against the already-migrated database must be a no-op
	root, _ := integration.RepoRoot()
	wd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd) //nolint:errcheck
	again, err := db.NewPool(integration.PGConfig(migrationsDBName()), integration.Logger())
	if err != nil {
		t.Fatalf("second NewPool: %v", err)
	}
	defer again.Close()
	if err := again.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if after := scalar[int64](t, pool, `SELECT COUNT(*) FROM goose_db_version`); after != before {
		t.Fatalf("goose_db_version rows %d -> %d; re-run applied migrations again", before, after)
	}
}

// Every migration's Down section must undo its Up, so the chain can be rolled back and re-applied.
func TestMigrations_DownAndUpAgain(t *testing.T) {
	pool := newPool(t)
	pool.Close()

	root, _ := integration.RepoRoot()
	conf := integration.PGConfig(migrationsDBName())
	sqlDB, err := sql.Open("pgx", "postgres://"+conf.DB.PgUser+":"+conf.DB.PgPASS+"@"+conf.DB.PgHost+":"+conf.DB.PgPort+"/"+conf.DB.PgDBName+"?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	ctx := context.Background()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "migrations")
	if err := goose.DownToContext(ctx, sqlDB, dir, 0); err != nil {
		t.Fatalf("down to 0: %v", err)
	}
	for _, table := range integration.AppTables {
		var exists bool
		_ = sqlDB.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists)
		if exists {
			t.Errorf("table %s still exists after rolling everything back", table)
		}
	}
	if err := goose.UpContext(ctx, sqlDB, dir); err != nil {
		t.Fatalf("up again: %v", err)
	}
	v, err := goose.GetDBVersionContext(ctx, sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if int(v) != len(migrationFiles(t)) {
		t.Fatalf("version after re-up = %d", v)
	}
}
