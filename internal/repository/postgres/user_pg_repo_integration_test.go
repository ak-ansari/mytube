//go:build integration

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestUserRepo_Upsert_CreatesUser(t *testing.T) {
	pool := setup(t)
	repo := postgres.NewUserRepo(pool)
	id := uuid.New()
	u, err := repo.Upsert(ctxT(t), &models.User{ID: id, UserName: "alice", ChannelName: "Alice TV", Email: "a@x.test"})
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != id || u.UserName != "alice" || u.ChannelName != "Alice TV" || u.Email != "a@x.test" {
		t.Fatalf("unexpected user %+v", u)
	}
	if u.Status != models.Active || u.Avatar != "" || u.Description != "" || u.Subscribers != 0 {
		t.Fatalf("unexpected defaults %+v", u)
	}
}

func TestUserRepo_Upsert_SameIdentityIsNoOp(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewUserRepo(pool)
	in := &models.User{ID: uuid.New(), UserName: "alice", ChannelName: "Alice TV", Email: "a@x.test"}
	if _, err := repo.Upsert(ctx, in); err != nil {
		t.Fatal(err)
	}
	exec(t, pool, `UPDATE users SET updated_at = $2 WHERE id = $1`, in.ID, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := repo.UpdateProfile(ctx, in.ID, ptr("Renamed"), ptr("bio"), ptr("avatars/a.png")); err != nil {
		t.Fatal(err)
	}
	exec(t, pool, `UPDATE users SET updated_at = $2 WHERE id = $1`, in.ID, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	// same Keycloak identity (channel name from the token is ignored on conflict)
	u, err := repo.Upsert(ctx, &models.User{ID: in.ID, UserName: "alice", ChannelName: "from token", Email: "a@x.test"})
	if err != nil {
		t.Fatal(err)
	}
	if u.ChannelName != "Renamed" || u.Description != "bio" || u.Avatar != "avatars/a.png" {
		t.Fatalf("profile fields not preserved: %+v", u)
	}
	var updated time.Time
	_ = pool.QueryRow(ctx, `SELECT updated_at FROM users WHERE id=$1`, in.ID).Scan(&updated)
	if updated.Year() != 2020 {
		t.Fatalf("unchanged identity should not write the row, updated_at = %v", updated)
	}
}

func TestUserRepo_Upsert_IdentityChangeUpdatesOnlyIdentity(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewUserRepo(pool)
	id := uuid.New()
	if _, err := repo.Upsert(ctx, &models.User{ID: id, UserName: "alice", ChannelName: "Alice TV", Email: "a@x.test"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateProfile(ctx, id, nil, ptr("bio"), nil); err != nil {
		t.Fatal(err)
	}
	u, err := repo.Upsert(ctx, &models.User{ID: id, UserName: "alice2", ChannelName: "ignored", Email: "new@x.test"})
	if err != nil {
		t.Fatal(err)
	}
	if u.UserName != "alice2" || u.Email != "new@x.test" {
		t.Fatalf("identity not updated: %+v", u)
	}
	if u.ChannelName != "Alice TV" || u.Description != "bio" {
		t.Fatalf("profile fields changed: %+v", u)
	}
	if n := count(t, pool, `SELECT COUNT(*) FROM users`); n != 1 {
		t.Fatalf("users = %d, want 1", n)
	}
}

func TestUserRepo_ProfileAndUpdate(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewUserRepo(pool)
	alice := mkUser(t, pool, "alice")
	bob := mkUser(t, pool, "bob")
	mkVideo(t, pool, alice, videoOpts{})                                           // counted
	mkVideo(t, pool, alice, videoOpts{Visibility: ptr(models.VisibilityPrivate)})  // not counted
	mkVideo(t, pool, alice, videoOpts{Status: models.StatusTranscoded})            // not counted
	mkVideo(t, pool, alice, videoOpts{Visibility: ptr(models.VisibilityUnlisted)}) // not counted
	if err := repo.Subscribe(ctx, bob, alice); err != nil {
		t.Fatal(err)
	}

	p, err := repo.GetProfile(ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != alice || p.UserName != "alice" || p.ChannelName == nil || *p.ChannelName != "alice channel" {
		t.Fatalf("unexpected profile %+v", p)
	}
	if p.Email == nil || *p.Email != "alice@example.test" {
		t.Fatalf("email = %v", p.Email)
	}
	if p.Subscribers != 1 || p.VideoCount != 1 || p.IsSubscribed {
		t.Fatalf("subscribers/videos/isSubscribed = %d/%d/%v, want 1/1/false", p.Subscribers, p.VideoCount, p.IsSubscribed)
	}

	// nil fields stay unchanged
	if err := repo.UpdateProfile(ctx, alice, nil, ptr("about me"), nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateProfile(ctx, alice, ptr("New name"), nil, ptr("avatars/a/1.png")); err != nil {
		t.Fatal(err)
	}
	p, _ = repo.GetProfile(ctx, alice)
	if *p.ChannelName != "New name" || p.Description == nil || *p.Description != "about me" || p.Avatar == nil || *p.Avatar != "avatars/a/1.png" {
		t.Fatalf("UpdateProfile not applied: %+v", p.Channel)
	}
}

func TestUserRepo_GetProfile_NotFound(t *testing.T) {
	pool := setup(t)
	repo := postgres.NewUserRepo(pool)
	if _, err := repo.GetProfile(ctxT(t), uuid.New()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("err = %v, want pgx.ErrNoRows", err)
	}
	if _, err := repo.GetChannel(ctxT(t), uuid.New(), nil); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("err = %v, want pgx.ErrNoRows", err)
	}
}

func TestUserRepo_Subscriptions(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewUserRepo(pool)
	alice := mkUser(t, pool, "alice")
	bob := mkUser(t, pool, "bob")
	carol := mkUser(t, pool, "carol")

	// idempotent
	for i := 0; i < 2; i++ {
		if err := repo.Subscribe(ctx, alice, bob); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.Subscribe(ctx, alice, carol); err != nil {
		t.Fatal(err)
	}
	if n := count(t, pool, `SELECT COUNT(*) FROM subscriptions`); n != 2 {
		t.Fatalf("subscriptions = %d, want 2", n)
	}
	exec(t, pool, `UPDATE subscriptions SET created_at = NOW() - interval '1 hour' WHERE channel_id = $1`, bob)

	if err := repo.Subscribe(ctx, alice, alice); err == nil {
		t.Fatal("self-subscription must violate the check constraint")
	}
	if err := repo.Subscribe(ctx, alice, uuid.New()); err == nil {
		t.Fatal("subscribing to an unknown channel must violate the foreign key")
	}

	ch, err := repo.GetChannel(ctx, bob, &alice)
	if err != nil {
		t.Fatal(err)
	}
	if !ch.IsSubscribed || ch.Subscribers != 1 {
		t.Fatalf("bob seen by alice: subscribed=%v subs=%d", ch.IsSubscribed, ch.Subscribers)
	}
	ch, _ = repo.GetChannel(ctx, bob, nil)
	if ch.IsSubscribed {
		t.Fatal("anonymous viewer cannot be subscribed")
	}
	ch, _ = repo.GetChannel(ctx, bob, &carol)
	if ch.IsSubscribed {
		t.Fatal("carol is not subscribed to bob")
	}

	list, err := repo.ListSubscriptions(ctx, alice)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != carol || list[1].ID != bob {
		t.Fatalf("subscriptions not newest-first: %v", list)
	}
	for _, c := range list {
		if !c.IsSubscribed || c.Subscribers != 1 {
			t.Fatalf("channel %s: subscribed=%v subs=%d", c.UserName, c.IsSubscribed, c.Subscribers)
		}
	}

	if err := repo.Unsubscribe(ctx, alice, bob); err != nil {
		t.Fatal(err)
	}
	if err := repo.Unsubscribe(ctx, alice, bob); err != nil { // not subscribed: no error
		t.Fatal(err)
	}
	list, _ = repo.ListSubscriptions(ctx, alice)
	if len(list) != 1 || list[0].ID != carol {
		t.Fatalf("after unsubscribe: %v", list)
	}
	list, err = repo.ListSubscriptions(ctx, bob)
	if err != nil || len(list) != 0 {
		t.Fatalf("bob subscriptions = %v, %v; want empty", list, err)
	}
}

func TestUserRepo_DeleteUserCascades(t *testing.T) {
	pool := setup(t)
	ctx := ctxT(t)
	repo := postgres.NewUserRepo(pool)
	alice := mkUser(t, pool, "alice")
	bob := mkUser(t, pool, "bob")
	v := mkVideo(t, pool, alice, videoOpts{})
	_ = repo.Subscribe(ctx, bob, alice)
	if err := postgres.NewVideoCatalogRepo(pool).SetReaction(ctx, v, bob, 1); err != nil {
		t.Fatal(err)
	}

	exec(t, pool, `DELETE FROM users WHERE id=$1`, alice)
	for table, want := range map[string]int{"videos": 0, "video_metadata": 0, "video_reactions": 0, "subscriptions": 0, "users": 1} {
		if n := count(t, pool, `SELECT COUNT(*) FROM `+table); n != want {
			t.Fatalf("%s = %d after deleting the user, want %d", table, n, want)
		}
	}
}
