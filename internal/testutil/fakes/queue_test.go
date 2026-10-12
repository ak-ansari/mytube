package fakes

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/google/uuid"
)

func TestQueueDelayed(t *testing.T) {
	ctx := context.Background()
	q := NewQueue()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q.Now = func() time.Time { return now }

	_ = q.EnqueueAfter(ctx, "j", []byte("c"), 3*time.Second)
	_ = q.EnqueueAfter(ctx, "j", []byte("a"), time.Second)
	_ = q.EnqueueAfter(ctx, "j", []byte("b"), 2*time.Second)
	_ = q.EnqueueAfter(ctx, "j", []byte("b2"), 2*time.Second) // same due: insertion order
	_ = q.EnqueueAfter(ctx, "j", []byte("now"), 0)            // straight onto the list
	if got := q.Messages("j"); len(got) != 1 || string(got[0]) != "now" {
		t.Fatalf("list = %q", got)
	}
	if d := q.Delayed("j"); len(d) != 4 || !d[0].Due.Equal(now.Add(3*time.Second)) || d[0].Delay != 3*time.Second {
		t.Fatalf("delayed = %+v", d)
	}
	if n, _ := q.PromoteDue(ctx, "j", now.Add(time.Second-1), 10); n != 0 {
		t.Fatalf("promoted %d before due", n)
	}
	if n, _ := q.PromoteDue(ctx, "j", now.Add(2*time.Second), 2); n != 2 {
		t.Fatalf("limited promote = %d", n)
	}
	if n, _ := q.PromoteDue(ctx, "j", now.Add(time.Hour), 0); n != 2 {
		t.Fatalf("rest = %d", n)
	}
	var got []string
	for _, m := range q.Messages("j") {
		got = append(got, string(m))
	}
	if strings.Join(got, ",") != "now,a,b,b2,c" {
		t.Fatalf("order = %v", got)
	}

	q.Errs.Set("PromoteDue", ErrCacheMiss)
	if _, err := q.PromoteDue(ctx, "j", now, 1); err == nil {
		t.Error("injected error ignored")
	}
}

func TestDeadLetterRepoNewestFirst(t *testing.T) {
	ctx := context.Background()
	r := NewDeadLetterRepo()
	vid := uuid.New()
	for _, e := range []string{"first", "second"} {
		if err := r.Insert(ctx, &models.DeadLetterJob{VideoID: &vid, Error: e}); err != nil {
			t.Fatal(err)
		}
	}
	_ = r.Insert(ctx, &models.DeadLetterJob{Error: "orphan"})
	list, _ := r.ListByVideo(ctx, vid)
	if len(list) != 2 || list[0].Error != "second" || !list[0].CreatedAt.After(list[1].CreatedAt) {
		t.Fatalf("list = %+v", list)
	}
	if len(r.All()) != 3 || r.All()[0].ID == uuid.Nil {
		t.Fatalf("all = %+v", r.All())
	}
}
