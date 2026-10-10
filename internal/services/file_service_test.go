package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ak-ansari/mytube/internal/services"
	"github.com/google/uuid"
)

func TestFileServicePresignImageUpload(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	files := services.NewFileService(e.store, e.svc)
	me, other := uuid.New(), uuid.New()
	mine := e.video(me)
	theirs := e.video(other)
	thumbDir := "thumbnails/" + mine.String() + "/"
	avatarDir := "avatars/" + me.String() + "/"

	tests := []struct {
		name string
		key  string
		size int64
		want error // nil = accepted
	}{
		{"own thumbnail png", thumbDir + "custom.png", 1, nil},
		{"own thumbnail jpg", thumbDir + "c.jpg", 1024, nil},
		{"own thumbnail jpeg", thumbDir + mine.String() + ".jpeg", 1024, nil},
		{"own thumbnail webp", thumbDir + "c.webp", 1024, nil},
		{"own thumbnail gif", thumbDir + "c.gif", 1024, nil},
		{"own thumbnail uppercase ext", thumbDir + "C.PNG", 1024, nil},
		{"max size", thumbDir + "c.png", services.MaxImageUploadSize, nil},
		{"own avatar", avatarDir + "a.png", 2048, nil},

		{"zero size", thumbDir + "c.png", 0, services.ErrBadRequest},
		{"negative size", thumbDir + "c.png", -1, services.ErrBadRequest},
		{"one byte over max", thumbDir + "c.png", services.MaxImageUploadSize + 1, services.ErrBadRequest},
		{"someone else's video", "thumbnails/" + theirs.String() + "/c.png", 10, services.ErrForbidden},
		{"unknown video", "thumbnails/" + uuid.NewString() + "/c.png", 10, services.ErrNotFound},
		{"someone else's avatar", "avatars/" + other.String() + "/a.png", 10, services.ErrBadRequest},
		{"avatar without user folder", "avatars/a.png", 10, services.ErrBadRequest},
		{"thumbnail without video folder", "thumbnails/c.png", 10, services.ErrBadRequest},
		{"thumbnail folder only", thumbDir, 10, services.ErrBadRequest},
		{"bad extension", thumbDir + "c.mp4", 10, services.ErrBadRequest},
		{"svg is not accepted", thumbDir + "c.svg", 10, services.ErrBadRequest},
		{"no extension", thumbDir + "c", 10, services.ErrBadRequest},
		{"nested thumbnail", thumbDir + "x/c.png", 10, services.ErrBadRequest},
		{"nested avatar", avatarDir + "x/a.png", 10, services.ErrBadRequest},
		{"dot-dot thumbnail", thumbDir + "../" + theirs.String() + "/c.png", 10, services.ErrBadRequest},
		{"dot-dot avatar", avatarDir + "..png", 10, services.ErrBadRequest},
		{"non-canonical uuid (upper case)", "thumbnails/" + strings.ToUpper(mine.String()) + "/c.png", 10, services.ErrBadRequest},
		{"non-canonical uuid (braces)", "thumbnails/{" + mine.String() + "}/c.png", 10, services.ErrBadRequest},
		{"non-canonical uuid (no dashes)", "thumbnails/" + strings.ReplaceAll(mine.String(), "-", "") + "/c.png", 10, services.ErrBadRequest},
		{"original", "originals/" + mine.String() + "/original.mp4", 10, services.ErrBadRequest},
		{"segments", "segments/" + mine.String() + "/master.m3u8", 10, services.ErrBadRequest},
		{"transcoded", "transcoded/" + mine.String() + "/720p.mp4", 10, services.ErrBadRequest},
		{"arbitrary key", "k", 10, services.ErrBadRequest},
		{"empty key", "", 10, services.ErrBadRequest},
		{"leading slash", "/" + thumbDir + "c.png", 10, services.ErrBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := e.store.Calls.Count("GerPreSignedPutUrl")
			url, err := files.PresignImageUpload(ctx, me, tt.key, tt.size)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				if !strings.HasPrefix(url, "https://put.test/"+tt.key+"?size=") {
					t.Errorf("url = %q", url)
				}
				return
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if e.store.Calls.Count("GerPreSignedPutUrl") != before {
				t.Error("a rejected key was signed")
			}
		})
	}

	t.Run("owner lookup error passes through", func(t *testing.T) {
		e.repo.Errs.Set("GetOwnerID", errBoom)
		defer e.repo.Errs.Set("GetOwnerID", nil)
		if _, err := files.PresignImageUpload(ctx, me, thumbDir+"c.png", 10); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("signing error passes through", func(t *testing.T) {
		e.store.Errs.Set("GerPreSignedPutUrl", errBoom)
		defer e.store.Errs.Set("GerPreSignedPutUrl", nil)
		if _, err := files.PresignImageUpload(ctx, me, avatarDir+"a.png", 10); !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestFileServiceDownloadURL(t *testing.T) {
	ctx := context.Background()
	e := newCatalogEnv(t)
	files := services.NewFileService(e.store, e.svc)
	id := uuid.NewString()

	for _, key := range []string{
		"thumbnails/" + id + "/" + id + ".jpeg",
		"thumbnails/" + id + "/c.webp",
		"avatars/" + id + "/a.png",
		"avatars/" + id + "/a.GIF",
	} {
		if !services.IsPublicImageKey(key) {
			t.Errorf("IsPublicImageKey(%q) = false", key)
		}
		if url, err := files.DownloadURL(ctx, key); err != nil || url != "https://get.test/"+key {
			t.Errorf("DownloadURL(%q) = %q, %v", key, url, err)
		}
	}
	for _, key := range []string{
		"", "k", "a/b",
		"thumbnails/x.png",
		"thumbnails/" + id + "/",
		"thumbnails/" + id + "/c.mp4",
		"thumbnails/" + id + "/x/c.png",
		"thumbnails/" + id + "/../c.png",
		"thumbnails/" + strings.ToUpper(id) + "/c.png",
		"thumbnails/not-a-uuid/c.png",
		"originals/" + id + "/original.mp4",
		"originals/" + id + "/original.png",
		"transcoded/" + id + "/720p.mp4",
		"segments/" + id + "/master.m3u8",
		"/avatars/" + id + "/a.png",
	} {
		if services.IsPublicImageKey(key) {
			t.Errorf("IsPublicImageKey(%q) = true", key)
		}
		if _, err := files.DownloadURL(ctx, key); !errors.Is(err, services.ErrBadRequest) {
			t.Errorf("DownloadURL(%q) err = %v, want ErrBadRequest", key, err)
		}
	}
	if e.store.Calls.Count("GetUrl") != 4 {
		t.Errorf("GetUrl calls = %d, want only the 4 accepted keys", e.store.Calls.Count("GetUrl"))
	}

	e.store.Errs.Set("GetUrl", errBoom)
	if _, err := files.DownloadURL(ctx, "avatars/"+id+"/a.png"); !errors.Is(err, errBoom) {
		t.Errorf("store error = %v", err)
	}
}

func TestAvatarPrefix(t *testing.T) {
	id := uuid.New()
	if got := services.AvatarPrefix(id); got != "avatars/"+id.String()+"/" {
		t.Errorf("AvatarPrefix = %q", got)
	}
}
