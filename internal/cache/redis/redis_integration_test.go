//go:build integration

package redisCache_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/cache"
	redisCache "github.com/ak-ansari/mytube/internal/cache/redis"
	"github.com/ak-ansari/mytube/internal/testutil/integration"
	"github.com/redis/go-redis/v9"
)

type videoInfo struct {
	ID    string   `json:"id"`
	Views int      `json:"views"`
	Tags  []string `json:"tags"`
}

// compile-time check: the Redis impl satisfies the app's Cache interface
var _ cache.Cache = redisCache.NewRedisCache(nil)

func newCache(t *testing.T) (cache.Cache, *redis.Client, string) {
	t.Helper()
	client, prefix := integration.Redis(t)
	return redisCache.NewRedisCache(client), client, prefix
}

func TestRedisCache_SetGetRoundTrip(t *testing.T) {
	c, raw, p := newCache(t)
	ctx := context.Background()
	key := cache.GetKey(p+cache.VIDEO_INFO, "v1")

	in := videoInfo{ID: "v1", Views: 3, Tags: []string{"a", "b"}}
	if err := c.Set(ctx, key, in, time.Minute); err != nil {
		t.Fatal(err)
	}
	var out videoInfo
	if err := c.Get(ctx, key, &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != "v1" || out.Views != 3 || len(out.Tags) != 2 || out.Tags[1] != "b" {
		t.Fatalf("round trip = %+v", out)
	}
	// stored as JSON with the requested TTL
	if s, _ := raw.Get(ctx, key).Result(); s != `{"id":"v1","views":3,"tags":["a","b"]}` {
		t.Fatalf("raw value = %s", s)
	}
	if ttl := raw.TTL(ctx, key).Val(); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("ttl = %v", ttl)
	}

	// plain strings are JSON encoded too
	if err := c.Set(ctx, p+"str", "originals/x.mp4", 0); err != nil {
		t.Fatal(err)
	}
	var s string
	if err := c.Get(ctx, p+"str", &s); err != nil || s != "originals/x.mp4" {
		t.Fatalf("string = %q, %v", s, err)
	}
	if ttl := raw.TTL(ctx, p+"str").Val(); ttl != -1 {
		t.Fatalf("expiration 0 should mean no TTL, got %v", ttl)
	}
}

func TestRedisCache_GetMissingLeavesTargetAndReturnsNil(t *testing.T) {
	c, _, p := newCache(t)
	out := videoInfo{ID: "untouched"}
	if err := c.Get(context.Background(), p+"missing", &out); err != nil {
		t.Fatalf("missing key err = %v, want nil", err)
	}
	if out.ID != "untouched" {
		t.Fatalf("target modified: %+v", out)
	}
}

func TestRedisCache_GetNonJSONFails(t *testing.T) {
	c, raw, p := newCache(t)
	raw.Set(context.Background(), p+"bad", "not json", 0)
	var out videoInfo
	if err := c.Get(context.Background(), p+"bad", &out); err == nil {
		t.Fatal("expected json error")
	}
}

func TestRedisCache_Expiry(t *testing.T) {
	c, _, p := newCache(t)
	ctx := context.Background()
	if err := c.Set(ctx, p+"short", 1, 150*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if ok, _ := c.Exists(ctx, p+"short"); !ok {
		t.Fatal("key should exist right after Set")
	}
	integration.Eventually(t, 3*time.Second, func() bool {
		ok, _ := c.Exists(ctx, p+"short")
		return !ok
	}, "key did not expire")
}

func TestRedisCache_DeleteAndExists(t *testing.T) {
	c, _, p := newCache(t)
	ctx := context.Background()
	if ok, err := c.Exists(ctx, p+"k"); err != nil || ok {
		t.Fatalf("exists before set = %v, %v", ok, err)
	}
	_ = c.Set(ctx, p+"k", true, time.Minute)
	if ok, _ := c.Exists(ctx, p+"k"); !ok {
		t.Fatal("exists after set = false")
	}
	if err := c.Delete(ctx, p+"k"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := c.Exists(ctx, p+"k"); ok {
		t.Fatal("exists after delete = true")
	}
	if err := c.Delete(ctx, p+"k"); err != nil { // deleting a missing key is fine
		t.Fatal(err)
	}
}

// The worker reads MinIO bucket events from a Redis hash (namespace format).
func TestRedisCache_HashOps(t *testing.T) {
	c, raw, p := newCache(t)
	ctx := context.Background()
	hash := p + "minio-events"

	all, err := c.GetAllFromHash(ctx, hash)
	if err != nil || all == nil || len(all) != 0 {
		t.Fatalf("missing hash = %#v, %v; want empty map", all, err)
	}
	if v, err := c.GetFromHash(ctx, hash, "nope"); err != nil || v != "" {
		t.Fatalf("missing field = %q, %v", v, err)
	}

	raw.HSet(ctx, hash, "mytube/originals/a/original.mp4", `{"Records":[1]}`, "mytube/originals/b/original.mp4", `{"Records":[2]}`)
	all, err = c.GetAllFromHash(ctx, hash)
	if err != nil || len(all) != 2 || all["mytube/originals/a/original.mp4"] != `{"Records":[1]}` {
		t.Fatalf("hash = %v, %v", all, err)
	}
	if v, _ := c.GetFromHash(ctx, hash, "mytube/originals/b/original.mp4"); v != `{"Records":[2]}` {
		t.Fatalf("field b = %q", v)
	}

	if err := c.DeleteFromHash(ctx, hash, "mytube/originals/a/original.mp4"); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteFromHash(ctx, hash, "mytube/originals/a/original.mp4"); err != nil { // already gone
		t.Fatal(err)
	}
	all, _ = c.GetAllFromHash(ctx, hash)
	if len(all) != 1 {
		t.Fatalf("after delete = %v", all)
	}

	// reading a non-hash key as a hash surfaces the WRONGTYPE error
	raw.Set(ctx, p+"plain", "x", 0)
	if _, err := c.GetAllFromHash(ctx, p+"plain"); err == nil {
		t.Fatal("expected WRONGTYPE error")
	}
	if _, err := c.GetFromHash(ctx, p+"plain", "f"); err == nil {
		t.Fatal("expected WRONGTYPE error")
	}
}

func TestRedisCache_SetNXLock(t *testing.T) {
	c, raw, p := newCache(t)
	ctx := context.Background()
	lock := p + "lock:video:1"

	ok, err := c.SetNX(ctx, lock, "worker-1", time.Minute)
	if err != nil || !ok {
		t.Fatalf("first SetNX = %v, %v", ok, err)
	}
	ok, err = c.SetNX(ctx, lock, "worker-2", time.Minute)
	if err != nil || ok {
		t.Fatalf("second SetNX = %v, %v; want false", ok, err)
	}
	var holder string
	_ = c.Get(ctx, lock, &holder)
	if holder != "worker-1" {
		t.Fatalf("holder = %q", holder)
	}
	if ttl := raw.TTL(ctx, lock).Val(); ttl <= 0 {
		t.Fatalf("lock ttl = %v", ttl)
	}
	if err := c.DeleteLock(ctx, lock); err != nil {
		t.Fatal(err)
	}
	if ok, _ := c.SetNX(ctx, lock, "worker-2", time.Minute); !ok {
		t.Fatal("SetNX after DeleteLock should succeed")
	}

	// lock with TTL frees itself
	_, _ = c.SetNX(ctx, p+"ttl-lock", 1, 150*time.Millisecond)
	integration.Eventually(t, 3*time.Second, func() bool {
		ok, _ := c.SetNX(ctx, p+"ttl-lock", 2, time.Minute)
		return ok
	}, "expired lock was not re-acquirable")
}

// View dedupe and stage locks rely on SETNX: concurrent callers get exactly one winner.
func TestRedisCache_SetNXConcurrentExactlyOneWins(t *testing.T) {
	c, _, p := newCache(t)
	ctx := context.Background()
	for round := 0; round < 5; round++ {
		key := p + "race:" + string(rune('a'+round))
		var wins, errs atomic.Int32
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < 32; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				ok, err := c.SetNX(ctx, key, i, 30*time.Minute)
				if err != nil {
					errs.Add(1)
				} else if ok {
					wins.Add(1)
				}
			}(i)
		}
		close(start)
		wg.Wait()
		if errs.Load() != 0 || wins.Load() != 1 {
			t.Fatalf("round %d: wins=%d errs=%d, want exactly one winner", round, wins.Load(), errs.Load())
		}
	}
}

func TestRedisCache_SetUnmarshalableValueFails(t *testing.T) {
	c, _, p := newCache(t)
	if err := c.Set(context.Background(), p+"ch", make(chan int), time.Minute); err == nil {
		t.Fatal("expected json marshal error")
	}
	if _, err := c.SetNX(context.Background(), p+"ch", make(chan int), time.Minute); err == nil {
		t.Fatal("expected json marshal error")
	}
}
