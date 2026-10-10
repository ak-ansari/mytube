package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ak-ansari/mytube/internal/auth"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/testutil/fakes"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func newUserEnv() (*fakes.UserRepo, *fakes.ObjectStore, *services.UserService) {
	repo, store := fakes.NewUserRepo(), fakes.NewObjectStore()
	return repo, store, services.NewUserService(repo, store)
}

func claims(sub, user, email, name string) *auth.Claims {
	return &auth.Claims{RegisteredClaims: jwt.RegisteredClaims{Subject: sub}, PreferredUsername: user, Email: email, Name: name}
}

func TestSyncFromToken(t *testing.T) {
	ctx := context.Background()
	id := uuid.NewString()
	tests := []struct {
		name                     string
		c                        *auth.Claims
		wantErr                  bool
		wantUser, wantCh, wantEm string
	}{
		{"all claims", claims(id, "demo", "d@x.io", "Demo Channel"), false, "demo", "Demo Channel", "d@x.io"},
		{"no name -> username", claims(id, "demo", "d@x.io", ""), false, "demo", "demo", "d@x.io"},
		{"no username -> email", claims(id, "", "d@x.io", ""), false, "d@x.io", "d@x.io", "d@x.io"},
		{"sub not a uuid", claims("service-account", "demo", "", ""), true, "", "", ""},
		{"empty sub", claims("", "demo", "", ""), true, "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, svc := newUserEnv()
			u, err := svc.SyncFromToken(ctx, tt.c)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if tt.wantErr {
				return
			}
			if u.ID.String() != id || u.UserName != tt.wantUser || u.ChannelName != tt.wantCh || u.Email != tt.wantEm {
				t.Errorf("user = %+v", u)
			}
		})
	}
}

func TestSyncFromTokenKeepsProfileFields(t *testing.T) {
	ctx := context.Background()
	repo, _, svc := newUserEnv()
	id := uuid.New()
	if _, err := svc.SyncFromToken(ctx, claims(id.String(), "demo", "a@x.io", "First")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateProfile(ctx, id, services.UpdateProfileInput{ChannelName: fakes.Ptr("Renamed")}); err != nil {
		t.Fatal(err)
	}
	u, err := svc.SyncFromToken(ctx, claims(id.String(), "demo2", "b@x.io", "Other"))
	if err != nil {
		t.Fatal(err)
	}
	if u.ChannelName != "Renamed" || u.UserName != "demo2" || u.Email != "b@x.io" {
		t.Errorf("user after re-login = %+v", u)
	}
	repo.Errs.Set("Upsert", errBoom)
	if _, err := svc.SyncFromToken(ctx, claims(id.String(), "demo", "", "")); !errors.Is(err, errBoom) {
		t.Errorf("err = %v", err)
	}
}

func TestGetProfile(t *testing.T) {
	ctx := context.Background()
	repo, store, svc := newUserEnv()
	id := uuid.New()
	repo.Put(models.User{ID: id, UserName: "demo", ChannelName: "Demo", Avatar: "avatars/" + id.String() + "/a.png", Email: "d@x.io"})

	p, err := svc.GetProfile(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if p.AvatarURL != "https://get.test/avatars/"+id.String()+"/a.png" || *p.Email != "d@x.io" || *p.ChannelName != "Demo" {
		t.Errorf("profile = %+v", p)
	}
	if _, err := svc.GetProfile(ctx, uuid.New()); !errors.Is(err, services.ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
	repo.Errs.Set("GetProfile", errBoom)
	if _, err := svc.GetProfile(ctx, id); !errors.Is(err, errBoom) {
		t.Errorf("repo error: %v", err)
	}
	repo.Errs.Set("GetProfile", nil)
	store.Errs.Set("GetUrl", errBoom)
	if p, err := svc.GetProfile(ctx, id); err != nil || p.AvatarURL != "" {
		t.Errorf("presign failure: %+v %v", p, err)
	}
}

func TestUpdateProfile(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		in      services.UpdateProfileInput
		wantErr error
		check   func(t *testing.T, p *models.Profile)
	}{
		{"name too short", services.UpdateProfileInput{ChannelName: fakes.Ptr("  ab ")}, services.ErrBadRequest, nil},
		{"name too long", services.UpdateProfileInput{ChannelName: fakes.Ptr(strings.Repeat("n", 61))}, services.ErrBadRequest, nil},
		{"description too long", services.UpdateProfileInput{Description: fakes.Ptr(strings.Repeat("d", 1001))}, services.ErrBadRequest, nil},
		{"name trimmed", services.UpdateProfileInput{ChannelName: fakes.Ptr("  My Channel  ")}, nil, func(t *testing.T, p *models.Profile) {
			if *p.ChannelName != "My Channel" {
				t.Errorf("channel name = %q", *p.ChannelName)
			}
		}},
		{"boundaries", services.UpdateProfileInput{ChannelName: fakes.Ptr(strings.Repeat("n", 60)), Description: fakes.Ptr(strings.Repeat("d", 1000))}, nil, nil},
		{"avatar and description", services.UpdateProfileInput{Description: fakes.Ptr("hello"), Avatar: fakes.Ptr("avatars/x/a.png")}, nil, func(t *testing.T, p *models.Profile) {
			if *p.Description != "hello" || p.AvatarURL != "https://get.test/avatars/x/a.png" || *p.ChannelName != "Orig" {
				t.Errorf("profile = %+v", p)
			}
		}},
		{"empty update", services.UpdateProfileInput{}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, _, svc := newUserEnv()
			id := uuid.New()
			repo.Put(models.User{ID: id, UserName: "u", ChannelName: "Orig"})
			p, err := svc.UpdateProfile(ctx, id, tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil && repo.Calls.Count("UpdateProfile") != 0 {
				t.Error("invalid input reached the repo")
			}
			if tt.check != nil {
				tt.check(t, p)
			}
		})
	}
	t.Run("repo error", func(t *testing.T) {
		repo, _, svc := newUserEnv()
		repo.Errs.Set("UpdateProfile", errBoom)
		if _, err := svc.UpdateProfile(ctx, uuid.New(), services.UpdateProfileInput{}); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestChannelsAndSubscriptions(t *testing.T) {
	ctx := context.Background()
	repo, store, svc := newUserEnv()
	me, ch, ch2 := uuid.New(), uuid.New(), uuid.New()
	repo.Put(models.User{ID: me, UserName: "me"})
	repo.Put(models.User{ID: ch, UserName: "creator", ChannelName: "Creator", Avatar: "avatars/c.png"})
	repo.Put(models.User{ID: ch2, UserName: "creator2"})

	if _, err := svc.GetChannel(ctx, uuid.New(), nil); !errors.Is(err, services.ErrNotFound) {
		t.Errorf("missing channel: %v", err)
	}
	if _, err := svc.Subscribe(ctx, me, me); !errors.Is(err, services.ErrBadRequest) {
		t.Errorf("self subscribe: %v", err)
	}
	if _, err := svc.Subscribe(ctx, me, uuid.New()); !errors.Is(err, services.ErrNotFound) {
		t.Errorf("subscribe to missing: %v", err)
	}
	if repo.Calls.Count("Subscribe") != 0 {
		t.Error("invalid subscribe reached the repo")
	}

	c, err := svc.Subscribe(ctx, me, ch)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsSubscribed || c.Subscribers != 1 || c.AvatarURL != "https://get.test/avatars/c.png" {
		t.Errorf("after subscribe: %+v", c)
	}
	// idempotent
	if c, _ = svc.Subscribe(ctx, me, ch); c.Subscribers != 1 {
		t.Errorf("double subscribe counted twice: %d", c.Subscribers)
	}
	if _, err := svc.Subscribe(ctx, me, ch2); err != nil {
		t.Fatal(err)
	}

	anon, err := svc.GetChannel(ctx, ch, nil)
	if err != nil || anon.IsSubscribed || anon.Subscribers != 1 {
		t.Errorf("anonymous view: %+v %v", anon, err)
	}
	mine, err := svc.GetChannel(ctx, ch, &me)
	if err != nil || !mine.IsSubscribed {
		t.Errorf("viewer view: %+v %v", mine, err)
	}

	subs, err := svc.ListSubscriptions(ctx, me)
	if err != nil || len(subs) != 2 {
		t.Fatalf("subscriptions = %v, %v", subs, err)
	}
	for _, s := range subs {
		if s.ID == ch && s.AvatarURL == "" {
			t.Error("subscription avatar not signed")
		}
	}

	c, err = svc.Unsubscribe(ctx, me, ch)
	if err != nil || c.IsSubscribed || c.Subscribers != 0 {
		t.Errorf("after unsubscribe: %+v %v", c, err)
	}
	if _, err := svc.Unsubscribe(ctx, me, uuid.New()); !errors.Is(err, services.ErrNotFound) {
		t.Errorf("unsubscribe from missing channel: %v", err)
	}

	empty, err := svc.ListSubscriptions(ctx, uuid.New())
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("no subscriptions = %v, %v", empty, err)
	}

	for _, m := range []string{"Subscribe", "Unsubscribe", "ListSubscriptions"} {
		repo.Errs.Set(m, errBoom)
	}
	if _, err := svc.Subscribe(ctx, me, ch); !errors.Is(err, errBoom) {
		t.Errorf("subscribe repo error: %v", err)
	}
	if _, err := svc.Unsubscribe(ctx, me, ch); !errors.Is(err, errBoom) {
		t.Errorf("unsubscribe repo error: %v", err)
	}
	if _, err := svc.ListSubscriptions(ctx, me); !errors.Is(err, errBoom) {
		t.Errorf("list repo error: %v", err)
	}
	_ = store
}
