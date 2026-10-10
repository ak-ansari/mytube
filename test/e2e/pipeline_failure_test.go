//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestShortClipPublishes uploads a 3s clip. The thumbnail stage used to seek to 00:03, past the end
// of the clip, so ffmpeg wrote no frame; it now seeks to min(3s, duration/2) = 1s, so the clip gets
// an auto thumbnail and goes all the way to published.
func TestShortClipPublishes(t *testing.T) {
	clip := makeClipOf(t, 3)
	r := call(t, owner, http.MethodPost, "/videos/upload/presign", map[string]any{"file_name": "e2e-short.mp4", "size": fileSize(t, clip)})
	expect(t, r, http.StatusCreated, "presign")
	var up struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	r.decode(t, &up)
	t.Cleanup(func() {
		if r := call(t, owner, http.MethodDelete, "/videos/"+up.ID, nil); r.Code != http.StatusOK {
			t.Errorf("cleanup: DELETE = %d", r.Code)
		}
		if r, _ := details(t, owner, up.ID); r.Code != http.StatusNotFound {
			t.Errorf("cleanup: details after delete = %d, want 404", r.Code)
		}
	})
	putFile(t, up.URL, clip)

	poll(t, phase1Timeout, time.Second, "short clip valid with thumbnail", func() (bool, string) {
		_, d := details(t, owner, up.ID)
		if strings.HasSuffix(d.Status, "_failed") {
			t.Fatalf("short clip failed in phase 1: %s (thumbnail seek should stay inside the clip)", d.Status)
		}
		return d.Status == "valid" && d.Thumbnail != nil && *d.Thumbnail != "",
			fmt.Sprintf("status=%s thumbnail=%s", d.Status, deref(d.Thumbnail))
	})
	d := mustDetails(t, owner, up.ID)
	if want := "thumbnails/" + up.ID + "/" + up.ID + ".jpeg"; deref(d.Thumbnail) != want {
		t.Errorf("auto thumbnail = %s, want %s", deref(d.Thumbnail), want)
	}
	if code, _ := rawGet(t, d.ThumbnailURL); code != http.StatusOK {
		t.Errorf("GET thumbnail_url = %d", code)
	}

	r = call(t, owner, http.MethodPost, "/videos/"+up.ID+"/confirm", map[string]any{"title": "short clip " + randWord(6), "visibility": "unlisted"})
	expect(t, r, http.StatusOK, "confirm short clip")
	poll(t, publishTimeout, 2*time.Second, "short clip published", func() (bool, string) {
		_, d := details(t, owner, up.ID)
		if strings.HasSuffix(d.Status, "_failed") {
			t.Fatalf("short clip failed in phase 2: %s", d.Status)
		}
		return d.Status == "published" && d.ManifestURL != "", "status=" + d.Status
	})
	d = mustDetails(t, nil, up.ID) // unlisted: watchable by link
	if d.ManifestURL == "" || len(d.AvailableQualities) == 0 {
		t.Errorf("published short clip: manifest_url=%q qualities=%v", d.ManifestURL, d.AvailableQualities)
	} else if code, body := rawGet(t, d.ManifestURL); code != http.StatusOK || !strings.HasPrefix(string(body), "#EXTM3U") {
		t.Errorf("GET manifest = %d", code)
	}
}
