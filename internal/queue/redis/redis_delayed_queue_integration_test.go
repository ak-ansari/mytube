//go:build integration

package redisQueue_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/queue"
	redisQueue "github.com/ak-ansari/mytube/internal/queue/redis"
	"github.com/ak-ansari/mytube/internal/testutil/integration"
	"github.com/redis/go-redis/v9"
)

var _ queue.DelayedQueue = redisQueue.NewRedisQ(nil)

func drain(t *testing.T, c *redis.Client, list string) []string {
	t.Helper()
	out, err := c.LRange(context.Background(), list, 0, -1).Result()
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDelayedQueue_ZeroDelayGoesStraightToTheList(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	name := p + "jobs"
	for _, d := range []time.Duration{0, -time.Second} {
		if err := q.EnqueueAfter(ctx, name, []byte("now"), d); err != nil {
			t.Fatal(err)
		}
	}
	if got := drain(t, client, name); len(got) != 2 || got[0] != "now" {
		t.Fatalf("list = %v", got)
	}
	if n := client.ZCard(ctx, name+":delayed").Val(); n != 0 {
		t.Fatalf("delayed set = %d, want 0", n)
	}
}

func TestDelayedQueue_NotPromotedBeforeDue(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	name := p + "jobs"

	enqueuedAt := time.Now()
	if err := q.EnqueueAfter(ctx, name, []byte(`{"video_id":"a","stage":4,"attempt":1}`), 10*time.Second); err != nil {
		t.Fatal(err)
	}
	// stored in the ZSET "<queue>:delayed", not on the list
	if n := client.LLen(ctx, name).Val(); n != 0 {
		t.Fatalf("list length = %d before due", n)
	}
	members, err := client.ZRangeWithScores(ctx, name+":delayed", 0, -1).Result()
	if err != nil || len(members) != 1 {
		t.Fatalf("delayed set = %v, %v", members, err)
	}
	due := time.UnixMilli(int64(members[0].Score))
	if d := due.Sub(enqueuedAt); d < 10*time.Second-50*time.Millisecond || d > 10*time.Second+2*time.Second {
		t.Errorf("score is %v after enqueue, want ~10s", d)
	}
	if m := members[0].Member.(string); !strings.HasSuffix(m, `|{"video_id":"a","stage":4,"attempt":1}`) {
		t.Errorf("member = %q, want <uuid>|<payload>", m)
	}

	for _, at := range []time.Time{time.Now(), due.Add(-time.Millisecond)} {
		n, err := q.PromoteDue(ctx, name, at, 100)
		if err != nil || n != 0 {
			t.Fatalf("PromoteDue(%v) = %d, %v; want 0 before due", at, n, err)
		}
	}
	if n := client.LLen(ctx, name).Val(); n != 0 {
		t.Fatal("promoted before due")
	}

	// due exactly at the score (inclusive)
	n, err := q.PromoteDue(ctx, name, due, 100)
	if err != nil || n != 1 {
		t.Fatalf("PromoteDue at due = %d, %v", n, err)
	}
	got, err := q.Dequeue(ctx, name)
	if err != nil || string(got) != `{"video_id":"a","stage":4,"attempt":1}` {
		t.Fatalf("dequeued %q, %v (uuid prefix must be stripped)", got, err)
	}
	if n := client.ZCard(ctx, name+":delayed").Val(); n != 0 {
		t.Errorf("delayed set not emptied: %d", n)
	}
}

func TestDelayedQueue_PromotesInDueOrderAndRespectsLimit(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	name := p + "jobs"

	// enqueue out of order; payloads contain the separator to check only the first '|' is split on
	delays := map[string]time.Duration{"c|3": 3 * time.Second, "a|1": time.Second, "e|5": 5 * time.Second, "b|2": 2 * time.Second, "d|4": 4 * time.Second}
	for payload, d := range delays {
		if err := q.EnqueueAfter(ctx, name, []byte(payload), d); err != nil {
			t.Fatal(err)
		}
	}
	later := time.Now().Add(time.Minute)

	n, err := q.PromoteDue(ctx, name, later, 2)
	if err != nil || n != 2 {
		t.Fatalf("limited promote = %d, %v", n, err)
	}
	if got := drain(t, client, name); strings.Join(got, ",") != "a|1,b|2" {
		t.Fatalf("after limit 2: list = %v, want earliest first", got)
	}
	// limit <= 0 falls back to 100
	n, err = q.PromoteDue(ctx, name, later, 0)
	if err != nil || n != 3 {
		t.Fatalf("promote rest = %d, %v", n, err)
	}
	if got := drain(t, client, name); strings.Join(got, ",") != "a|1,b|2,c|3,d|4,e|5" {
		t.Fatalf("list = %v", got)
	}
	// nothing left
	if n, err := q.PromoteDue(ctx, name, later, 100); err != nil || n != 0 {
		t.Fatalf("empty promote = %d, %v", n, err)
	}
	// promoted messages go behind what is already on the list
	_ = q.Enqueue(ctx, p+"q2", []byte("existing"))
	_ = q.EnqueueAfter(ctx, p+"q2", []byte("retry"), time.Millisecond)
	if _, err := q.PromoteDue(ctx, p+"q2", later, 10); err != nil {
		t.Fatal(err)
	}
	if got := drain(t, client, p+"q2"); strings.Join(got, ",") != "existing,retry" {
		t.Fatalf("q2 = %v", got)
	}
}

func TestDelayedQueue_PartialPromotionOnlyMovesDueMessages(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	name := p + "jobs"
	_ = q.EnqueueAfter(ctx, name, []byte("soon"), 50*time.Millisecond)
	_ = q.EnqueueAfter(ctx, name, []byte("later"), time.Hour)
	time.Sleep(100 * time.Millisecond)
	n, err := q.PromoteDue(ctx, name, time.Now(), 100)
	if err != nil || n != 1 {
		t.Fatalf("promote = %d, %v", n, err)
	}
	if got := drain(t, client, name); len(got) != 1 || got[0] != "soon" {
		t.Fatalf("list = %v", got)
	}
	if n := client.ZCard(ctx, name+":delayed").Val(); n != 1 {
		t.Fatalf("delayed left = %d, want the hour-long one", n)
	}
}

func TestDelayedQueue_IdenticalPayloadsAreKeptSeparately(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	name := p + "jobs"
	payload := []byte(`{"video_id":"same","stage":4,"attempt":1}`)
	for i := 0; i < 3; i++ {
		if err := q.EnqueueAfter(ctx, name, payload, time.Second); err != nil {
			t.Fatal(err)
		}
	}
	if n := client.ZCard(ctx, name+":delayed").Val(); n != 3 {
		t.Fatalf("delayed set = %d, want 3 distinct members", n)
	}
	n, err := q.PromoteDue(ctx, name, time.Now().Add(time.Minute), 100)
	if err != nil || n != 3 {
		t.Fatalf("promote = %d, %v", n, err)
	}
	got := drain(t, client, name)
	if len(got) != 3 {
		t.Fatalf("list = %v", got)
	}
	for _, g := range got {
		if g != string(payload) {
			t.Fatalf("payload = %q", g)
		}
	}
}

func TestDelayedQueue_BinaryPayload(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	in := []byte{0, '|', 1, 255, '\n', 0}
	if err := q.EnqueueAfter(ctx, p+"bin", in, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if _, err := q.PromoteDue(ctx, p+"bin", time.Now().Add(time.Second), 10); err != nil {
		t.Fatal(err)
	}
	if out, err := q.Dequeue(ctx, p+"bin"); err != nil || string(out) != string(in) {
		t.Fatalf("binary round trip = %v, %v", out, err)
	}
}

// Several workers promote concurrently (every runner iteration calls PromoteDue): each message
// lands on the list exactly once.
func TestDelayedQueue_ConcurrentPromoteMovesEachOnce(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	name := p + "jobs"
	const msgs = 300
	for i := 0; i < msgs; i++ {
		if err := q.EnqueueAfter(ctx, name, []byte(fmt.Sprint(i)), time.Duration(1+i%7)*time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Add(time.Second)

	const workers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	start := make(chan struct{})
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for {
				n, err := q.PromoteDue(ctx, name, now, 7)
				if err != nil {
					t.Error(err)
					return
				}
				mu.Lock()
				total += n
				mu.Unlock()
				if n == 0 {
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()

	if total != msgs {
		t.Errorf("promote counts sum to %d, want %d", total, msgs)
	}
	got := drain(t, client, name)
	if len(got) != msgs {
		t.Fatalf("list has %d messages, want %d", len(got), msgs)
	}
	sort.Strings(got)
	seen := map[string]bool{}
	for _, g := range got {
		if seen[g] {
			t.Fatalf("message %s promoted twice", g)
		}
		seen[g] = true
	}
	if n := client.ZCard(ctx, name+":delayed").Val(); n != 0 {
		t.Errorf("delayed left = %d", n)
	}
}
