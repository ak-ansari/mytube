package stream

import (
	"strings"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/config"
)

func TestPlaybackURL(t *testing.T) {
	s, err := NewSigner(config.Stream{BaseUrl: "http://localhost/", TokenSecret: "secret", TokenTTL: "2h"})
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 1, 1, 10, 20, 0, 0, time.UTC) }

	got := s.PlaybackURL("65ae26a1-c831-4010-8a2e-82a2e35b0292", "master.m3u8")
	// 10:20 + 2h = 12:20, rounded up to 13:00
	want := "http://localhost/videos/" + s.token("65ae26a1-c831-4010-8a2e-82a2e35b0292", 1767272400) +
		"/1767272400/65ae26a1-c831-4010-8a2e-82a2e35b0292/master.m3u8"
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
	// nginx's location regex expects a 22-char unpadded base64url md5
	tok := strings.Split(got, "/")[4]
	if len(tok) != 22 || strings.ContainsAny(tok, "+/=") {
		t.Fatalf("token %q is not unpadded base64url md5", tok)
	}
}

func TestNewSignerRequiresSecret(t *testing.T) {
	if _, err := NewSigner(config.Stream{BaseUrl: "http://localhost"}); err == nil {
		t.Fatal("expected an error without TOKEN_SECRET")
	}
}
