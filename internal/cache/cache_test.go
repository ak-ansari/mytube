package cache_test

import (
	"testing"

	"github.com/ak-ansari/mytube/internal/cache"
)

func TestGetKey(t *testing.T) {
	tests := []struct{ prefix, id, want string }{
		{cache.KEY, "originals/abc/original.mp4", "key:originals/abc/original.mp4"},
		{cache.URL, "x", "url:x"},
		{cache.VIDEO_INFO, "42", "video_info:42"},
		{"view", "vid:1.2.3.4", "view:vid:1.2.3.4"},
		{"", "", ":"},
	}
	for _, tt := range tests {
		if got := cache.GetKey(tt.prefix, tt.id); got != tt.want {
			t.Errorf("GetKey(%q, %q) = %q, want %q", tt.prefix, tt.id, got, tt.want)
		}
	}
}
