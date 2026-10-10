//go:build integration

package redisQueue_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/queue"
	redisQueue "github.com/ak-ansari/mytube/internal/queue/redis"
	"github.com/ak-ansari/mytube/internal/testutil/integration"
)

var _ queue.Queue = redisQueue.NewRedisQ(nil)

func TestRedisQueue_FIFO(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	name := p + "video_queue"

	payloads := []string{`{"job":"validate","id":"1"}`, `{"job":"thumbnail","id":"1"}`, `{"job":"transcode","id":"2"}`}
	for _, pl := range payloads {
		if err := q.Enqueue(ctx, name, []byte(pl)); err != nil {
			t.Fatal(err)
		}
	}
	if n := client.LLen(ctx, name).Val(); n != 3 {
		t.Fatalf("queue length = %d", n)
	}
	for i, want := range payloads {
		got, err := q.Dequeue(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Fatalf("dequeue %d = %s, want %s", i, got, want)
		}
	}
	if n := client.LLen(ctx, name).Val(); n != 0 {
		t.Fatalf("queue not drained: %d", n)
	}
}

func TestRedisQueue_BinaryPayload(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	in := []byte{0, 1, 2, 255, '\n', 0}
	_ = q.Enqueue(ctx, p+"bin", in)
	out, err := q.Dequeue(ctx, p+"bin")
	if err != nil || string(out) != string(in) {
		t.Fatalf("binary round trip = %v, %v", out, err)
	}
}

func TestRedisQueue_QueuesAreIsolated(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	_ = q.Enqueue(ctx, p+"a", []byte("for-a"))
	_ = q.Enqueue(ctx, p+"b", []byte("for-b"))
	if got, _ := q.Dequeue(ctx, p+"b"); string(got) != "for-b" {
		t.Fatalf("queue b = %s", got)
	}
	if got, _ := q.Dequeue(ctx, p+"a"); string(got) != "for-a" {
		t.Fatalf("queue a = %s", got)
	}
}

// Dequeue blocks up to 5s, then reports "no message" as (nil, nil).
func TestRedisQueue_DequeueEmptyTimesOut(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	start := time.Now()
	got, err := q.Dequeue(context.Background(), p+"empty")
	if err != nil || got != nil {
		t.Fatalf("empty dequeue = %v, %v; want nil, nil", got, err)
	}
	if el := time.Since(start); el < 4*time.Second {
		t.Fatalf("returned after %v; expected to block ~5s", el)
	}
}

func TestRedisQueue_BlockedDequeueWakesOnEnqueue(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	got := make(chan []byte, 1)
	go func() {
		b, _ := q.Dequeue(ctx, p+"wake")
		got <- b
	}()
	time.Sleep(200 * time.Millisecond)
	_ = q.Enqueue(ctx, p+"wake", []byte("hello"))
	select {
	case b := <-got:
		if string(b) != "hello" {
			t.Fatalf("got %q", b)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("blocked consumer was not woken by Enqueue")
	}
}

// Several workers share one queue: every job is delivered exactly once.
func TestRedisQueue_CompetingConsumersDeliverOnce(t *testing.T) {
	client, p := integration.Redis(t)
	q := redisQueue.NewRedisQ(client)
	ctx := context.Background()
	name := p + "jobs"
	const jobs = 100
	const workers = 4
	for i := 0; i < jobs; i++ {
		_ = q.Enqueue(ctx, name, []byte(fmt.Sprint(i)))
	}
	for w := 0; w < workers; w++ { // one stop sentinel per worker, queued after the jobs
		_ = q.Enqueue(ctx, name, []byte("STOP"))
	}
	var mu sync.Mutex
	seen := map[string]int{}
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				b, err := q.Dequeue(ctx, name)
				if err != nil || b == nil || string(b) == "STOP" {
					return
				}
				mu.Lock()
				seen[string(b)]++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(seen) != jobs {
		t.Fatalf("delivered %d distinct jobs, want %d", len(seen), jobs)
	}
	for k, n := range seen {
		if n != 1 {
			t.Fatalf("job %s delivered %d times", k, n)
		}
	}
}
