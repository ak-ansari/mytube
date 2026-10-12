// Package stream mints playback URLs for the nginx HLS gateway (deployments/nginx/templates).
package stream

import (
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ak-ansari/mytube/internal/config"
)

// Signer builds /videos/<token>/<expires>/<videoId>/<file> URLs that nginx's secure_link module verifies.
// The token covers a whole video, so relative playlist/segment URLs inherit it.
type Signer struct {
	baseURL string
	secret  string
	ttl     time.Duration
	now     func() time.Time
}

func NewSigner(conf config.Stream) (*Signer, error) {
	if conf.TokenSecret == "" || conf.BaseUrl == "" {
		return nil, errors.New("STREAM.BASE_URL and STREAM.TOKEN_SECRET must be configured")
	}
	ttl := 6 * time.Hour
	if conf.TokenTTL != "" {
		d, err := time.ParseDuration(conf.TokenTTL)
		if err != nil {
			return nil, fmt.Errorf("invalid STREAM.TOKEN_TTL: %w", err)
		}
		if d <= 0 {
			return nil, fmt.Errorf("STREAM.TOKEN_TTL must be positive, got %s", conf.TokenTTL)
		}
		ttl = d
	}
	return &Signer{baseURL: strings.TrimRight(conf.BaseUrl, "/"), secret: conf.TokenSecret, ttl: ttl, now: time.Now}, nil
}

// PlaybackURL returns a signed URL for one file (e.g. master.m3u8) of a video's HLS folder.
func (s *Signer) PlaybackURL(videoID, file string) string {
	// round the expiry up to the hour so the URL is stable (and cacheable) for an hour at a time
	expires := s.now().Add(s.ttl).Truncate(time.Hour).Add(time.Hour).Unix()
	return fmt.Sprintf("%s/videos/%s/%d/%s/%s", s.baseURL, s.token(videoID, expires), expires, videoID, file)
}

// token must match the nginx side: secure_link_md5 "$secure_link_expires:$vid:<secret>".
func (s *Signer) token(videoID string, expires int64) string {
	sum := md5.Sum([]byte(fmt.Sprintf("%d:%s:%s", expires, videoID, s.secret)))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
