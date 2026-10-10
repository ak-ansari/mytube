//go:build integration

// Package integration holds shared setup for the integration tests that run against the real
// dev Docker stack (Postgres, Redis, MinIO via nginx, Elasticsearch).
//
// Run them with:
//
//	go test -tags integration -count=1 -p 1 ./...
//
// Every service is configured from IT_* environment variables with defaults that match
// deployments/docker-compose.yml. A test whose service is unreachable is skipped, not failed.
// Nothing here touches dev data: Postgres uses a throwaway database (mytube_test), Redis uses
// DB 15 plus a unique key prefix per test, MinIO uses the mytube-test bucket with a unique
// prefix per test, and Elasticsearch tests use a uniquely named throwaway index.
package integration

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/db"
	"github.com/ak-ansari/mytube/pkg/logger"
	es_v9 "github.com/elastic/go-elasticsearch/v9"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
)

// ErrUnavailable means a backing service could not be reached; callers should skip.
var ErrUnavailable = errors.New("integration service unavailable")

// Env returns the environment variable key, or def when it is unset or empty.
func Env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func reachable(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrUnavailable, addr, err)
	}
	return conn.Close()
}

// Logger returns a logger.Logger that discards everything.
func Logger() logger.Logger { return nopLogger{} }

type nopLogger struct{}

func (nopLogger) Debug(string, ...logger.Field)   {}
func (nopLogger) Info(string, ...logger.Field)    {}
func (nopLogger) Success(string, ...logger.Field) {}
func (nopLogger) Warn(string, ...logger.Field)    {}
func (nopLogger) Error(string, ...logger.Field)   {}
func (nopLogger) Fatal(string, ...logger.Field)   {}
func (nopLogger) Flush()                          {}

// RepoRoot walks up from the working directory to the directory holding go.mod.
func RepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found above working directory")
		}
		dir = parent
	}
}

// ---------------------------------------------------------------- Postgres

// PGConfig builds an app config pointing at dbName on the test Postgres server.
//
//	IT_PG_HOST (localhost) IT_PG_PORT (5432) IT_PG_USER (mytube) IT_PG_PASS (mytube)
func PGConfig(dbName string) *config.Config {
	return &config.Config{DB: config.DB{
		PgHost:   Env("IT_PG_HOST", "localhost"),
		PgPort:   Env("IT_PG_PORT", "5432"),
		PgUser:   Env("IT_PG_USER", "mytube"),
		PgPASS:   Env("IT_PG_PASS", "mytube"),
		PgDBName: dbName,
	}}
}

// TestDBName is the throwaway database used by repository tests (IT_PG_DB, default mytube_test).
func TestDBName() string { return Env("IT_PG_DB", "mytube_test") }

func pgURL(c *config.Config) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.DB.PgUser, c.DB.PgPASS),
		Host:     net.JoinHostPort(c.DB.PgHost, c.DB.PgPort),
		Path:     "/" + c.DB.PgDBName,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

// RecreateDatabase drops (forcefully) and creates dbName, connecting through the maintenance
// database IT_PG_ADMIN_DB (default "postgres"). It refuses to touch the dev database.
func RecreateDatabase(ctx context.Context, dbName string) error {
	if dbName == "" || !strings.Contains(dbName, "test") {
		return fmt.Errorf("refusing to recreate database %q: name must contain \"test\"", dbName)
	}
	admin := PGConfig(Env("IT_PG_ADMIN_DB", "postgres"))
	if err := reachable(net.JoinHostPort(admin.DB.PgHost, admin.DB.PgPort)); err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, pgURL(admin))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer conn.Close(ctx)
	ident := pgx.Identifier{dbName}.Sanitize()
	if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+ident+" WITH (FORCE)"); err != nil {
		return err
	}
	_, err = conn.Exec(ctx, "CREATE DATABASE "+ident)
	return err
}

// DropDatabase drops dbName (best effort, used at the end of TestMain).
func DropDatabase(ctx context.Context, dbName string) error {
	if !strings.Contains(dbName, "test") {
		return fmt.Errorf("refusing to drop database %q", dbName)
	}
	conn, err := pgx.Connect(ctx, pgURL(PGConfig(Env("IT_PG_ADMIN_DB", "postgres"))))
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	return err
}

// MigratedPool recreates dbName and opens a pool through the app's own db.NewPool, which
// applies ./migrations with goose. db.NewPool resolves ./migrations against the working
// directory, so this temporarily switches to the repo root.
func MigratedPool(ctx context.Context, dbName string) (*pgxpool.Pool, error) {
	if err := RecreateDatabase(ctx, dbName); err != nil {
		return nil, err
	}
	root, err := RepoRoot()
	if err != nil {
		return nil, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err := os.Chdir(root); err != nil {
		return nil, err
	}
	defer os.Chdir(wd) //nolint:errcheck
	return db.NewPool(PGConfig(dbName), Logger())
}

// PGPool is a lazily initialised migrated pool for one package's TestMain.
type PGPool struct {
	Pool   *pgxpool.Pool
	DBName string
	Err    error
}

// SetupPG is meant for TestMain: it recreates the test database and applies migrations.
// The error is kept so each test can skip (unreachable) or fail (anything else).
func SetupPG(dbName string) *PGPool {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	p, err := MigratedPool(ctx, dbName)
	return &PGPool{Pool: p, DBName: dbName, Err: err}
}

// Require returns the pool, skipping the test when Postgres is unreachable.
func (p *PGPool) Require(t testing.TB) *pgxpool.Pool {
	t.Helper()
	if p == nil {
		t.Fatal("PGPool not initialised; is TestMain missing?")
	}
	if errors.Is(p.Err, ErrUnavailable) {
		t.Skipf("postgres not reachable: %v", p.Err)
	}
	if p.Err != nil {
		t.Fatalf("postgres setup failed: %v", p.Err)
	}
	return p.Pool
}

// Close closes the pool and drops the database unless IT_PG_KEEP=1.
func (p *PGPool) Close() {
	if p == nil || p.Pool == nil {
		return
	}
	p.Pool.Close()
	if Env("IT_PG_KEEP", "") == "1" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = DropDatabase(ctx, p.DBName)
}

// AppTables are the tables created by ./migrations (goose's own table excluded).
var AppTables = []string{"subscriptions", "comments", "video_reactions", "video_metadata", "videos", "users"}

// Truncate empties every app table.
func Truncate(t testing.TB, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "TRUNCATE "+strings.Join(AppTables, ", ")+" CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// ---------------------------------------------------------------- Redis

// Redis returns a client on IT_REDIS_DB (default 15) plus a unique key prefix for this test.
// Every key starting with the prefix is deleted when the test ends.
//
//	IT_REDIS_ADDR (127.0.0.1:6379) IT_REDIS_DB (15)
func Redis(t testing.TB) (*redis.Client, string) {
	t.Helper()
	addr := Env("IT_REDIS_ADDR", "127.0.0.1:6379")
	dbNum, err := strconv.Atoi(Env("IT_REDIS_DB", "15"))
	if err != nil {
		t.Fatalf("IT_REDIS_DB: %v", err)
	}
	if err := reachable(addr); err != nil {
		t.Skipf("redis not reachable: %v", err)
	}
	c := redis.NewClient(&redis.Options{Addr: addr, DB: dbNum})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		t.Skipf("redis ping failed: %v", err)
	}
	prefix := "it:" + uuid.NewString() + ":"
	t.Cleanup(func() {
		ctx := context.Background()
		iter := c.Scan(ctx, 0, prefix+"*", 100).Iterator()
		for iter.Next(ctx) {
			c.Del(ctx, iter.Val())
		}
		_ = c.Close()
	})
	return c, prefix
}

// ---------------------------------------------------------------- MinIO

// MinioSettings are the S3 connection values for the test bucket.
//
//	IT_MINIO_ENDPOINT (localhost:9002) IT_MINIO_ACCESS_KEY (mytube)
//	IT_MINIO_SECRET_KEY (mytube123) IT_MINIO_BUCKET (mytube-test)
type MinioSettings struct {
	Endpoint, AccessKey, SecretKey, Bucket string
}

func MinioFromEnv() MinioSettings {
	return MinioSettings{
		Endpoint:  Env("IT_MINIO_ENDPOINT", "localhost:9002"),
		AccessKey: Env("IT_MINIO_ACCESS_KEY", "mytube"),
		SecretKey: Env("IT_MINIO_SECRET_KEY", "mytube123"),
		Bucket:    Env("IT_MINIO_BUCKET", "mytube-test"),
	}
}

// Minio returns a client, the test bucket (created if missing) and a unique key prefix
// "it-<uuid>/". Objects under the prefix are removed when the test ends, and the bucket is
// removed too if it is then empty.
func Minio(t testing.TB) (*minio.Client, MinioSettings, string) {
	t.Helper()
	s := MinioFromEnv()
	if s.Bucket == "mytube" {
		t.Fatal("IT_MINIO_BUCKET must not be the dev bucket")
	}
	if err := reachable(s.Endpoint); err != nil {
		t.Skipf("minio not reachable: %v", err)
	}
	c, err := minio.New(s.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(s.AccessKey, s.SecretKey, ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("minio client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exists, err := c.BucketExists(ctx, s.Bucket)
	if err != nil {
		t.Skipf("minio not usable: %v", err)
	}
	if !exists {
		if err := c.MakeBucket(ctx, s.Bucket, minio.MakeBucketOptions{Region: "us-east-1"}); err != nil {
			t.Fatalf("make bucket %s: %v", s.Bucket, err)
		}
	}
	prefix := "it-" + uuid.NewString() + "/"
	t.Cleanup(func() {
		ctx := context.Background()
		objs := c.ListObjects(ctx, s.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
		for res := range c.RemoveObjects(ctx, s.Bucket, objs, minio.RemoveObjectsOptions{}) {
			if res.Err != nil {
				t.Errorf("cleanup %s: %v", res.ObjectName, res.Err)
			}
		}
		_ = c.RemoveBucket(ctx, s.Bucket) // only succeeds once empty
	})
	return c, s, prefix
}

// ---------------------------------------------------------------- Elasticsearch

// Elastic returns an ES client for IT_ES_URL (default http://localhost:9200).
func Elastic(t testing.TB) *es_v9.Client {
	t.Helper()
	raw := Env("IT_ES_URL", "http://localhost:9200")
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("IT_ES_URL: %v", err)
	}
	if err := reachable(u.Host); err != nil {
		t.Skipf("elasticsearch not reachable: %v", err)
	}
	c, err := es_v9.NewClient(es_v9.Config{Addresses: []string{raw}})
	if err != nil {
		t.Fatalf("es client: %v", err)
	}
	res, err := c.Ping()
	if err != nil {
		t.Skipf("elasticsearch ping failed: %v", err)
	}
	res.Body.Close()
	if res.IsError() {
		t.Skipf("elasticsearch ping: %s", res.Status())
	}
	return c
}

// Eventually polls cond until it returns true or the timeout elapses.
func Eventually(t testing.TB, timeout time.Duration, cond func() bool, msg string, args ...any) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("condition not met within %s: "+msg, append([]any{timeout}, args...)...)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
