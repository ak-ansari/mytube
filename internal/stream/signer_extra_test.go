package stream

import (
	"crypto/md5"
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/config"
)

func TestNewSignerConfig(t *testing.T) {
	tests := []struct {
		name    string
		conf    config.Stream
		wantErr bool
		ttl     time.Duration
		base    string
	}{
		{"default ttl", config.Stream{BaseUrl: "http://gw", TokenSecret: "s"}, false, 6 * time.Hour, "http://gw"},
		{"custom ttl", config.Stream{BaseUrl: "http://gw", TokenSecret: "s", TokenTTL: "90m"}, false, 90 * time.Minute, "http://gw"},
		{"trailing slashes trimmed", config.Stream{BaseUrl: "http://gw///", TokenSecret: "s"}, false, 6 * time.Hour, "http://gw"},
		{"bad ttl", config.Stream{BaseUrl: "http://gw", TokenSecret: "s", TokenTTL: "six hours"}, true, 0, ""},
		{"negative ttl", config.Stream{BaseUrl: "http://gw", TokenSecret: "s", TokenTTL: "-1h"}, true, 0, ""},
		{"zero ttl", config.Stream{BaseUrl: "http://gw", TokenSecret: "s", TokenTTL: "0s"}, true, 0, ""},
		{"bare zero ttl", config.Stream{BaseUrl: "http://gw", TokenSecret: "s", TokenTTL: "0"}, true, 0, ""},
		{"smallest positive ttl", config.Stream{BaseUrl: "http://gw", TokenSecret: "s", TokenTTL: "1ns"}, false, time.Nanosecond, "http://gw"},
		{"missing base url", config.Stream{TokenSecret: "s"}, true, 0, ""},
		{"missing section entirely", config.Stream{}, true, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSigner(tt.conf)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v", err)
			}
			if tt.wantErr {
				return
			}
			if s.ttl != tt.ttl || s.baseURL != tt.base {
				t.Errorf("signer ttl=%v base=%q", s.ttl, s.baseURL)
			}
		})
	}
}

func TestPlaybackURLRounding(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		now  time.Time
		ttl  string
		want time.Time
	}{
		{"mid-hour rounds up", base.Add(10*time.Hour + 20*time.Minute), "2h", base.Add(13 * time.Hour)},
		{"exactly on the hour still adds an hour", base.Add(10 * time.Hour), "2h", base.Add(13 * time.Hour)},
		{"one second past", base.Add(10*time.Hour + time.Second), "2h", base.Add(13 * time.Hour)},
		{"last second of the hour", base.Add(10*time.Hour + 59*time.Minute + 59*time.Second), "2h", base.Add(13 * time.Hour)},
		{"minute ttl", base.Add(10*time.Hour + 30*time.Minute), "45m", base.Add(12 * time.Hour)},
		{"crosses midnight", base.Add(23*time.Hour + 30*time.Minute), "1h", base.Add(25 * time.Hour)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewSigner(config.Stream{BaseUrl: "http://gw", TokenSecret: "s", TokenTTL: tt.ttl})
			if err != nil {
				t.Fatal(err)
			}
			s.now = func() time.Time { return tt.now }
			parts := strings.Split(s.PlaybackURL("vid", "master.m3u8"), "/")
			exp, err := strconv.ParseInt(parts[5], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			if exp != tt.want.Unix() {
				t.Errorf("expires = %s, want %s", time.Unix(exp, 0).UTC(), tt.want)
			}
			if !time.Unix(exp, 0).After(tt.now) {
				t.Error("url already expired when issued")
			}
		})
	}
}

func TestPlaybackURLIsStableWithinTheHour(t *testing.T) {
	s, _ := NewSigner(config.Stream{BaseUrl: "http://gw", TokenSecret: "s", TokenTTL: "1h"})
	t0 := time.Date(2026, 1, 1, 10, 1, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	a := s.PlaybackURL("vid", "master.m3u8")
	s.now = func() time.Time { return t0.Add(58 * time.Minute) }
	if b := s.PlaybackURL("vid", "master.m3u8"); a != b {
		t.Errorf("url changed within the hour:\n%s\n%s", a, b)
	}
	s.now = func() time.Time { return t0.Add(time.Hour) }
	if c := s.PlaybackURL("vid", "master.m3u8"); a == c {
		t.Error("url should change in the next hour")
	}
}

func TestTokenBindsVideoExpiryAndSecret(t *testing.T) {
	s1, _ := NewSigner(config.Stream{BaseUrl: "http://gw", TokenSecret: "one"})
	s2, _ := NewSigner(config.Stream{BaseUrl: "http://gw", TokenSecret: "two"})
	base := s1.token("vid", 100)
	if base != s1.token("vid", 100) {
		t.Fatal("token not deterministic")
	}
	for name, other := range map[string]string{
		"other video":  s1.token("vid2", 100),
		"other expiry": s1.token("vid", 101),
		"other secret": s2.token("vid", 100),
	} {
		if other == base {
			t.Errorf("%s produced the same token", name)
		}
	}
	// nginx: secure_link_md5 "$secure_link_expires:$vid:<secret>" -> base64url(md5), no padding
	sum := md5.Sum([]byte("1700000000:abc:one"))
	if got, want := s1.token("abc", 1700000000), base64.RawURLEncoding.EncodeToString(sum[:]); got != want {
		t.Errorf("token = %q, want %q", got, want)
	}
}

func TestPlaybackURLExpiredTokenScenario(t *testing.T) {
	// a URL minted with a past clock carries an expiry nginx will reject (410); the signer never mints those for "now"
	s, _ := NewSigner(config.Stream{BaseUrl: "http://gw", TokenSecret: "s", TokenTTL: "1h"})
	past := time.Now().Add(-48 * time.Hour)
	s.now = func() time.Time { return past }
	exp, _ := strconv.ParseInt(strings.Split(s.PlaybackURL("v", "a.ts"), "/")[5], 10, 64)
	if time.Unix(exp, 0).After(time.Now()) {
		t.Fatal("expected an expired timestamp")
	}
	s.now = time.Now
	exp, _ = strconv.ParseInt(strings.Split(s.PlaybackURL("v", "a.ts"), "/")[5], 10, 64)
	if !time.Unix(exp, 0).After(time.Now().Add(time.Hour)) {
		t.Fatal("fresh url should outlive the ttl")
	}
}
