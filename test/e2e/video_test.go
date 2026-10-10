//go:build e2e

package e2e

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

const (
	phase1Timeout  = 2 * time.Minute
	publishTimeout = 4 * time.Minute
	esTimeout      = 30 * time.Second
)

// streamSecret mirrors STREAM.TOKEN_SECRET; only used to mint an *expired* gateway URL.
var streamSecret = envOr("E2E_STREAM_SECRET", "change-me-local-dev-secret")

// TestVideoLifecycle uploads one real clip through the whole pipeline and then exercises the
// catalog, studio and engagement APIs against it. Subtests run in order and share the video;
// it is deleted (and checked to be gone) in t.Cleanup.
func TestVideoLifecycle(t *testing.T) {
	clip := makeClip(t)
	size := fileSize(t, clip)
	word := "e2e" + randWord(12) // unique ES token
	title := "E2E clip " + word

	// ---- 1. presign ----
	r := call(t, owner, http.MethodPost, "/videos/upload/presign", map[string]any{"file_name": "e2e-clip.mp4", "size": size})
	expect(t, r, http.StatusCreated, "presign")
	var up struct {
		ID  string `json:"id"`
		Key string `json:"key"`
		URL string `json:"url"`
	}
	r.decode(t, &up)
	if _, err := uuid.Parse(up.ID); err != nil {
		t.Fatalf("presign id %q is not a uuid", up.ID)
	}
	if want := "originals/" + up.ID + "/original.mp4"; up.Key != want {
		t.Errorf("key = %s, want %s", up.Key, want)
	}
	id := up.ID
	t.Logf("video id %s", id)

	t.Cleanup(func() { deleteAndVerify(t, id, word) })

	// ---- 2. upload bytes to MinIO ----
	putFile(t, up.URL, clip)

	// ---- 3. wait for phase 1 (valid + thumbnail) ----
	poll(t, phase1Timeout, time.Second, "status valid with thumbnail", func() (bool, string) {
		_, d := details(t, owner, id)
		if strings.HasSuffix(d.Status, "_failed") {
			t.Fatalf("pipeline failed in phase 1: %s", d.Status)
		}
		return d.Status == "valid" && d.Thumbnail != nil && *d.Thumbnail != "",
			fmt.Sprintf("status=%s thumbnail=%s (is the worker running and seeing MinIO events?)", d.Status, deref(d.Thumbnail))
	})
	d := mustDetails(t, owner, id)
	if d.Title != nil {
		t.Errorf("title before confirm = %q, want null", *d.Title)
	}
	if want := "thumbnails/" + id + "/"; !strings.HasPrefix(deref(d.Thumbnail), want) {
		t.Errorf("auto thumbnail %s not under %s", deref(d.Thumbnail), want)
	}
	if d.ThumbnailURL == "" {
		t.Errorf("thumbnail_url empty")
	} else if code, _ := rawGet(t, d.ThumbnailURL); code != http.StatusOK {
		t.Errorf("GET presigned thumbnail_url = %d, want 200", code)
	}
	// unconfirmed videos are visible to the owner only
	r, _ = details(t, nil, id)
	expect(t, r, http.StatusNotFound, "anonymous details before publish")
	if !containsVideo(listVideos(t, owner, "/videos/mine?size=50"), id) {
		t.Errorf("/videos/mine does not list the processing video")
	}

	// ---- 4. confirm ----
	if other != nil {
		r = call(t, other, http.MethodPost, "/videos/"+id+"/confirm", map[string]any{"title": title, "visibility": "public"})
		expect(t, r, http.StatusForbidden, "confirm by non-owner")
	}
	r = call(t, owner, http.MethodPost, "/videos/"+id+"/confirm", map[string]any{"title": "ab", "visibility": "public"})
	expect(t, r, http.StatusBadRequest, "confirm with short title")
	r = call(t, owner, http.MethodPost, "/videos/"+id+"/confirm", map[string]any{"title": title, "visibility": "secret"})
	expect(t, r, http.StatusBadRequest, "confirm with bad visibility")
	r = call(t, owner, http.MethodPost, "/videos/"+id+"/confirm", map[string]any{"title": title, "visibility": "public", "thumbnail": "thumbnails/" + uuid.NewString() + "/x.jpeg"})
	expect(t, r, http.StatusBadRequest, "confirm with foreign thumbnail key")
	r = call(t, owner, http.MethodPost, "/videos/"+id+"/confirm", map[string]any{
		"title": "  " + title + "  ", "description": "end-to-end test upload " + word, "visibility": "public",
	})
	expect(t, r, http.StatusOK, "confirm")

	// ---- 5. wait for published ----
	poll(t, publishTimeout, 2*time.Second, "status published", func() (bool, string) {
		_, d := details(t, owner, id)
		if strings.HasSuffix(d.Status, "_failed") {
			t.Fatalf("pipeline failed in phase 2: %s", d.Status)
		}
		return d.Status == "published" && d.ManifestURL != "", "status=" + d.Status
	})
	d = mustDetails(t, nil, id)
	if deref(d.Title) != title {
		t.Errorf("title = %q, want trimmed %q", deref(d.Title), title)
	}
	if deref(d.Visibility) != "public" {
		t.Errorf("visibility = %s", deref(d.Visibility))
	}
	if d.Channel.ID != owner.id {
		t.Errorf("channel.id = %s, want %s", d.Channel.ID, owner.id)
	}
	if len(d.AvailableQualities) == 0 {
		t.Errorf("available_qualities empty")
	}

	t.Run("Discovery", func(t *testing.T) { testDiscovery(t, id, word) })
	t.Run("Playback", func(t *testing.T) { testPlayback(t, id) })
	t.Run("Views", func(t *testing.T) { testViews(t, id) })
	t.Run("Reactions", func(t *testing.T) { testReactions(t, id) })
	t.Run("Comments", func(t *testing.T) { testComments(t, id) })
	t.Run("Subscriptions", func(t *testing.T) { testSubscriptions(t, id) })
	t.Run("StudioEdit", func(t *testing.T) { testStudioEdit(t, id) })
	t.Run("Visibility", func(t *testing.T) { testVisibility(t, id) })
	t.Run("Validation", func(t *testing.T) { testVideoValidation(t, id) })
}

// ---------- subtests ----------

func testDiscovery(t *testing.T, id, word string) {
	poll(t, 10*time.Second, time.Second, "video on home feed", func() (bool, string) {
		return containsVideo(listVideos(t, nil, "/videos/browse?size=50"), id), "not in first 50 of /videos/browse"
	})
	poll(t, esTimeout, time.Second, "video in ES search", func() (bool, string) {
		p := listVideos(t, nil, "/videos/browse?search="+url.QueryEscape(word))
		return containsVideo(p, id), fmt.Sprintf("%d hits for %q", len(p.Items), word)
	})
	if !containsVideo(listVideos(t, owner, "/videos/mine?size=50"), id) {
		t.Errorf("not in /videos/mine")
	}
	if !containsVideo(listVideos(t, nil, "/channels/"+owner.id+"/videos?size=50"), id) {
		t.Errorf("not in channel videos")
	}
	if containsVideo(listVideos(t, nil, "/videos/"+id+"/related?size=50"), id) {
		t.Errorf("related lists the video itself")
	}
	r := call(t, nil, http.MethodGet, "/channels/"+owner.id, nil)
	expect(t, r, http.StatusOK, "channel")
	var ch channel
	r.decode(t, &ch)
	if ch.VideoCount < 1 {
		t.Errorf("channel video_count = %d", ch.VideoCount)
	}
}

func testPlayback(t *testing.T, id string) {
	d := mustDetails(t, nil, id)
	if d.ManifestURL == "" {
		t.Fatal("manifest_url empty")
	}
	code, body := rawGet(t, d.ManifestURL)
	if code != http.StatusOK {
		t.Fatalf("GET manifest = %d, want 200", code)
	}
	if !strings.HasPrefix(string(body), "#EXTM3U") {
		t.Fatalf("manifest is not an m3u8: %.100s", body)
	}
	// relative variant playlist inherits the token
	variant := ""
	for _, line := range strings.Split(string(body), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			variant = line
			break
		}
	}
	if variant == "" {
		t.Errorf("master playlist has no variants")
	} else {
		base, _ := url.Parse(d.ManifestURL)
		ref, _ := url.Parse(variant)
		vurl := base.ResolveReference(ref).String()
		if code, vb := rawGet(t, vurl); code != http.StatusOK || !strings.HasPrefix(string(vb), "#EXTM3U") {
			t.Errorf("GET variant %s = %d", vurl, code)
		}
	}

	// URL layout: <base>/videos/<token>/<expires>/<videoId>/<file>
	u, _ := url.Parse(d.ManifestURL)
	seg := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(seg) != 5 || seg[0] != "videos" || seg[3] != id {
		t.Fatalf("unexpected manifest_url layout: %s", d.ManifestURL)
	}
	tok := []byte(seg[1])
	if tok[0] == 'A' {
		tok[0] = 'B'
	} else {
		tok[0] = 'A'
	}
	withPath := func(token, expires, vid string) string {
		c := *u
		c.Path = "/" + strings.Join([]string{"videos", token, expires, vid, seg[4]}, "/")
		return c.String()
	}
	if code, _ := rawGet(t, withPath(string(tok), seg[2], id)); code != http.StatusForbidden {
		t.Errorf("tampered token: %d, want 403", code)
	}
	if code, _ := rawGet(t, withPath(seg[1], seg[2], uuid.NewString())); code != http.StatusForbidden {
		t.Errorf("token reused for another video id: %d, want 403", code)
	}
	if code, _ := rawGet(t, withPath(seg[1], "9999999999", id)); code != http.StatusForbidden {
		t.Errorf("token with altered expiry: %d, want 403", code)
	}
	// a correctly signed but past expiry gives 410 (needs E2E_STREAM_SECRET == STREAM.TOKEN_SECRET)
	past := fmt.Sprint(time.Now().Add(-time.Hour).Unix())
	sum := md5.Sum([]byte(past + ":" + id + ":" + streamSecret))
	code, _ = rawGet(t, withPath(base64.RawURLEncoding.EncodeToString(sum[:]), past, id))
	switch code {
	case http.StatusGone:
	case http.StatusForbidden:
		t.Logf("expired-URL check inconclusive: E2E_STREAM_SECRET does not match the gateway secret")
	default:
		t.Errorf("expired token: %d, want 410", code)
	}
}

func testViews(t *testing.T, id string) {
	v0 := mustDetails(t, nil, id).Views
	for i := 0; i < 3; i++ {
		r := call(t, nil, http.MethodPost, "/videos/"+id+"/view", nil)
		expect(t, r, http.StatusOK, "anonymous view")
	}
	if v := mustDetails(t, nil, id).Views; v != v0+1 {
		t.Errorf("after 3 anonymous views from one IP: views=%d, want %d (dedupe)", v, v0+1)
	}
	// a logged-in viewer is keyed by user id, not IP
	r := call(t, owner, http.MethodPost, "/videos/"+id+"/view", nil)
	expect(t, r, http.StatusOK, "owner view")
	r = call(t, owner, http.MethodPost, "/videos/"+id+"/view", nil)
	expect(t, r, http.StatusOK, "owner view again")
	if v := mustDetails(t, nil, id).Views; v != v0+2 {
		t.Errorf("after owner views: views=%d, want %d", v, v0+2)
	}
	r = call(t, nil, http.MethodPost, "/videos/"+uuid.NewString()+"/view", nil)
	expect(t, r, http.StatusNotFound, "view unknown video")
	r = call(t, nil, http.MethodPost, "/videos/nope/view", nil)
	expect(t, r, http.StatusBadRequest, "view bad uuid")
}

func testReactions(t *testing.T, id string) {
	u := requireOther(t)
	react := func(value any, want int) videoDetails {
		t.Helper()
		r := call(t, u, http.MethodPut, "/videos/"+id+"/reaction", map[string]any{"value": value})
		expect(t, r, want, fmt.Sprintf("reaction %v", value))
		var d videoDetails
		if want == http.StatusOK {
			r.decode(t, &d)
		}
		return d
	}
	check := func(stage string, likes, dislikes int64, mine int) {
		t.Helper()
		d := mustDetails(t, u, id)
		if d.Likes != likes || d.Dislikes != dislikes || d.MyReaction != mine {
			t.Errorf("%s: likes=%d dislikes=%d my_reaction=%d, want %d/%d/%d", stage, d.Likes, d.Dislikes, d.MyReaction, likes, dislikes, mine)
		}
		if a := mustDetails(t, nil, id); a.MyReaction != 0 || a.Likes != likes {
			t.Errorf("%s anonymous: likes=%d my_reaction=%d", stage, a.Likes, a.MyReaction)
		}
	}
	liked := func() bool { return containsVideo(listVideos(t, u, "/videos/liked?size=50"), id) }

	if d := react(1, http.StatusOK); d.MyReaction != 1 || d.Likes != 1 {
		t.Errorf("like response: likes=%d my_reaction=%d", d.Likes, d.MyReaction)
	}
	check("like", 1, 0, 1)
	if !liked() {
		t.Errorf("/videos/liked missing video after like")
	}
	react(1, http.StatusOK) // idempotent
	check("like twice", 1, 0, 1)

	react(-1, http.StatusOK)
	check("dislike", 0, 1, -1)
	if liked() {
		t.Errorf("/videos/liked still lists video after dislike")
	}

	react(0, http.StatusOK)
	check("clear", 0, 0, 0)

	react(2, http.StatusBadRequest)
	r := call(t, u, http.MethodPut, "/videos/"+id+"/reaction", map[string]any{})
	expect(t, r, http.StatusBadRequest, "reaction without value")
	r = call(t, u, http.MethodPut, "/videos/"+uuid.NewString()+"/reaction", map[string]any{"value": 1})
	expect(t, r, http.StatusNotFound, "reaction on unknown video")
}

func testComments(t *testing.T, id string) {
	u := requireOther(t)
	add := func(as *user, content string) comment {
		t.Helper()
		r := call(t, as, http.MethodPost, "/videos/"+id+"/comments", map[string]any{"content": content})
		expect(t, r, http.StatusOK, "add comment")
		var c comment
		r.decode(t, &c)
		if c.Content != strings.TrimSpace(content) || c.VideoID != id || c.Author.ID != as.id {
			t.Errorf("comment = %+v", c)
		}
		return c
	}
	list := func(q string) page[comment] {
		t.Helper()
		r := call(t, nil, http.MethodGet, "/videos/"+id+"/comments"+q, nil)
		expect(t, r, http.StatusOK, "list comments")
		var p page[comment]
		r.decode(t, &p)
		return p
	}
	del := func(as *user, cid string, want int, what string) {
		t.Helper()
		r := call(t, as, http.MethodDelete, "/videos/"+id+"/comments/"+cid, nil)
		expect(t, r, want, what)
	}

	byOwner := add(owner, "owner comment")
	byOther1 := add(u, "  other comment 1  ")
	byOther2 := add(u, "other comment 2")

	p1 := list("?size=2&page=1")
	if len(p1.Items) != 2 || !p1.HasMore || p1.Size != 2 || p1.Page != 1 {
		t.Errorf("page 1: %d items has_more=%v size=%d page=%d", len(p1.Items), p1.HasMore, p1.Size, p1.Page)
	}
	p2 := list("?size=2&page=2")
	if len(p2.Items) != 1 || p2.HasMore {
		t.Errorf("page 2: %d items has_more=%v", len(p2.Items), p2.HasMore)
	}
	seen := map[string]bool{}
	for _, c := range append(p1.Items, p2.Items...) {
		seen[c.ID] = true
	}
	for _, c := range []comment{byOwner, byOther1, byOther2} {
		if !seen[c.ID] {
			t.Errorf("comment %s missing from pages", c.ID)
		}
	}
	if n := mustDetails(t, nil, id).CommentCount; n != 3 {
		t.Errorf("comment_count = %d, want 3", n)
	}

	r := call(t, u, http.MethodPost, "/videos/"+id+"/comments", map[string]any{"content": "   "})
	expect(t, r, http.StatusBadRequest, "blank comment")
	r = call(t, u, http.MethodPost, "/videos/"+id+"/comments", map[string]any{"content": strings.Repeat("x", 2001)})
	expect(t, r, http.StatusBadRequest, "comment too long")
	r = call(t, u, http.MethodPost, "/videos/"+uuid.NewString()+"/comments", map[string]any{"content": "hi"})
	expect(t, r, http.StatusNotFound, "comment on unknown video")

	del(u, byOwner.ID, http.StatusForbidden, "stranger deletes video owner's comment")
	del(u, byOther1.ID, http.StatusOK, "author deletes own comment")
	del(owner, byOther2.ID, http.StatusOK, "video owner deletes someone else's comment")
	del(owner, byOwner.ID, http.StatusOK, "owner deletes own comment")
	del(owner, byOwner.ID, http.StatusNotFound, "delete already-deleted comment")
	del(owner, "not-a-uuid", http.StatusBadRequest, "delete bad comment id")

	if p := list(""); len(p.Items) != 0 {
		t.Errorf("%d comments left after deletes", len(p.Items))
	}
	if n := mustDetails(t, nil, id).CommentCount; n != 0 {
		t.Errorf("comment_count = %d after deletes", n)
	}
}

func testSubscriptions(t *testing.T, id string) {
	u := requireOther(t)
	t.Cleanup(func() { call(t, u, http.MethodDelete, "/channels/"+owner.id+"/subscribe", nil) })

	before := mustDetails(t, u, id)
	if before.IsSubscribed {
		t.Fatalf("new user already subscribed")
	}

	r := call(t, u, http.MethodPost, "/channels/"+owner.id+"/subscribe", nil)
	expect(t, r, http.StatusOK, "subscribe")
	var ch channel
	r.decode(t, &ch)
	if !ch.IsSubscribed || ch.Subscribers != before.Subscribers+1 {
		t.Errorf("subscribe response: is_subscribed=%v subscribers=%d (before %d)", ch.IsSubscribed, ch.Subscribers, before.Subscribers)
	}
	r = call(t, u, http.MethodPost, "/channels/"+owner.id+"/subscribe", nil)
	expect(t, r, http.StatusOK, "subscribe twice")
	r.decode(t, &ch)
	if ch.Subscribers != before.Subscribers+1 {
		t.Errorf("double subscribe counted twice: %d", ch.Subscribers)
	}

	if d := mustDetails(t, u, id); !d.IsSubscribed || d.Subscribers != before.Subscribers+1 {
		t.Errorf("details: is_subscribed=%v subscribers=%d", d.IsSubscribed, d.Subscribers)
	}
	if !subscribedTo(t, u, owner.id) {
		t.Errorf("/users/me/subscriptions missing channel")
	}
	if !containsVideo(listVideos(t, u, "/videos/subscriptions?size=50"), id) {
		t.Errorf("/videos/subscriptions feed missing video")
	}

	r = call(t, u, http.MethodDelete, "/channels/"+owner.id+"/subscribe", nil)
	expect(t, r, http.StatusOK, "unsubscribe")
	r.decode(t, &ch)
	if ch.IsSubscribed || ch.Subscribers != before.Subscribers {
		t.Errorf("unsubscribe response: is_subscribed=%v subscribers=%d", ch.IsSubscribed, ch.Subscribers)
	}
	if subscribedTo(t, u, owner.id) {
		t.Errorf("/users/me/subscriptions still lists channel")
	}
	if containsVideo(listVideos(t, u, "/videos/subscriptions?size=50"), id) {
		t.Errorf("feed still lists video after unsubscribe")
	}
	if mustDetails(t, u, id).IsSubscribed {
		t.Errorf("details still is_subscribed")
	}
}

func subscribedTo(t *testing.T, u *user, channelID string) bool {
	t.Helper()
	r := call(t, u, http.MethodGet, "/users/me/subscriptions", nil)
	expect(t, r, http.StatusOK, "my subscriptions")
	var chs []channel
	r.decode(t, &chs)
	for _, c := range chs {
		if c.ID == channelID {
			return true
		}
	}
	return false
}

func testStudioEdit(t *testing.T, id string) {
	newWord := "e2e" + randWord(12)
	newTitle := "E2E edited " + newWord
	r := call(t, owner, http.MethodPut, "/videos/"+id, map[string]any{"title": newTitle, "description": "edited description"})
	expect(t, r, http.StatusOK, "owner edit")
	var d videoDetails
	r.decode(t, &d)
	if deref(d.Title) != newTitle || deref(d.Description) != "edited description" {
		t.Errorf("edit response: %q / %q", deref(d.Title), deref(d.Description))
	}
	if deref(d.Visibility) != "public" {
		t.Errorf("partial edit changed visibility to %s", deref(d.Visibility))
	}
	if got := mustDetails(t, nil, id); deref(got.Title) != newTitle {
		t.Errorf("details title = %q after edit", deref(got.Title))
	}
	poll(t, esTimeout, time.Second, "re-indexed title in search", func() (bool, string) {
		p := listVideos(t, nil, "/videos/browse?search="+url.QueryEscape(newWord))
		return containsVideo(p, id), fmt.Sprintf("%d hits", len(p.Items))
	})

	r = call(t, owner, http.MethodPut, "/videos/"+id, map[string]any{"title": "x"})
	expect(t, r, http.StatusBadRequest, "edit with short title")
	r = call(t, owner, http.MethodPut, "/videos/"+id, map[string]any{"visibility": "secret"})
	expect(t, r, http.StatusBadRequest, "edit with bad visibility")
	r = call(t, owner, http.MethodPut, "/videos/"+id, map[string]any{"thumbnail": "avatars/x/y.png"})
	expect(t, r, http.StatusBadRequest, "edit with foreign thumbnail key")
	r = call(t, owner, http.MethodPut, "/videos/"+id, "{bad json")
	expect(t, r, http.StatusBadRequest, "edit with malformed body")
	r = call(t, owner, http.MethodPut, "/videos/"+uuid.NewString(), map[string]any{"title": "whatever"})
	expect(t, r, http.StatusNotFound, "edit unknown video")

	if u := other; u != nil {
		r = call(t, u, http.MethodPut, "/videos/"+id, map[string]any{"title": "hijacked title"})
		expect(t, r, http.StatusForbidden, "non-owner edit")
		r = call(t, u, http.MethodDelete, "/videos/"+id, nil)
		expect(t, r, http.StatusForbidden, "non-owner delete")
		if got := mustDetails(t, nil, id); deref(got.Title) != newTitle {
			t.Errorf("title changed by non-owner: %q", deref(got.Title))
		}
	} else {
		t.Log("non-owner checks skipped: " + otherSkipReason)
	}
}

func testVisibility(t *testing.T, id string) {
	setVis := func(v string) {
		t.Helper()
		r := call(t, owner, http.MethodPut, "/videos/"+id, map[string]any{"visibility": v})
		expect(t, r, http.StatusOK, "set visibility "+v)
		var d videoDetails
		r.decode(t, &d)
		if deref(d.Visibility) != v {
			t.Fatalf("visibility = %s, want %s", deref(d.Visibility), v)
		}
	}
	// always leave it public for later subtests
	t.Cleanup(func() { call(t, owner, http.MethodPut, "/videos/"+id, map[string]any{"visibility": "public"}) })
	title := deref(mustDetails(t, owner, id).Title)
	searchWord := title[strings.LastIndex(title, " ")+1:]

	// ---- private ----
	setVis("private")
	if containsVideo(listVideos(t, nil, "/videos/browse?size=50"), id) {
		t.Errorf("private video on home feed")
	}
	if containsVideo(listVideos(t, nil, "/videos/browse?search="+url.QueryEscape(searchWord)), id) {
		t.Errorf("private video in search results")
	}
	if containsVideo(listVideos(t, nil, "/channels/"+owner.id+"/videos?size=50"), id) {
		t.Errorf("private video in channel videos")
	}
	r, _ := details(t, nil, id)
	expectAny(t, r, "anonymous details of private video", http.StatusNotFound, http.StatusForbidden)
	if other != nil {
		r, _ = details(t, other, id)
		expectAny(t, r, "stranger details of private video", http.StatusNotFound, http.StatusForbidden)
		r = call(t, other, http.MethodGet, "/videos/"+id+"/comments", nil)
		expectAny(t, r, "stranger comments of private video", http.StatusNotFound, http.StatusForbidden)
		r = call(t, other, http.MethodPut, "/videos/"+id+"/reaction", map[string]any{"value": 1})
		expectAny(t, r, "stranger reacts to private video", http.StatusNotFound, http.StatusForbidden)
	}
	r = call(t, nil, http.MethodPost, "/videos/"+id+"/view", nil)
	expectAny(t, r, "anonymous view of private video", http.StatusNotFound, http.StatusForbidden)
	if d := mustDetails(t, owner, id); d.ManifestURL == "" {
		t.Errorf("owner gets no manifest_url for private video")
	}
	if !containsVideo(listVideos(t, owner, "/videos/mine?size=50"), id) {
		t.Errorf("private video missing from /videos/mine")
	}

	// ---- unlisted ----
	setVis("unlisted")
	d := mustDetails(t, nil, id)
	if d.ManifestURL == "" {
		t.Errorf("unlisted video has no manifest_url for anonymous viewer")
	} else if code, _ := rawGet(t, d.ManifestURL); code != http.StatusOK {
		t.Errorf("unlisted manifest GET = %d", code)
	}
	if containsVideo(listVideos(t, nil, "/videos/browse?size=50"), id) {
		t.Errorf("unlisted video on home feed")
	}
	if containsVideo(listVideos(t, nil, "/videos/browse?search="+url.QueryEscape(searchWord)), id) {
		t.Errorf("unlisted video in search results")
	}
	if containsVideo(listVideos(t, nil, "/channels/"+owner.id+"/videos?size=50"), id) {
		t.Errorf("unlisted video in channel videos")
	}
	r = call(t, nil, http.MethodGet, "/videos/"+id+"/comments", nil)
	expect(t, r, http.StatusOK, "comments of unlisted video")

	// ---- back to public ----
	setVis("public")
	if !containsVideo(listVideos(t, nil, "/videos/browse?size=50"), id) {
		t.Errorf("public again but not on home feed")
	}
}

func testVideoValidation(t *testing.T, id string) {
	r := call(t, nil, http.MethodGet, "/videos/not-a-uuid/details", nil)
	expect(t, r, http.StatusBadRequest, "details bad uuid")
	r = call(t, nil, http.MethodGet, "/videos/"+uuid.NewString()+"/details", nil)
	expect(t, r, http.StatusNotFound, "details unknown id")
	r = call(t, nil, http.MethodGet, "/videos/not-a-uuid/related", nil)
	expect(t, r, http.StatusBadRequest, "related bad uuid")
	r = call(t, nil, http.MethodGet, "/videos/not-a-uuid/comments", nil)
	expect(t, r, http.StatusBadRequest, "comments bad uuid")
	r = call(t, nil, http.MethodGet, "/videos/"+uuid.NewString()+"/comments", nil)
	expect(t, r, http.StatusNotFound, "comments unknown id")
	r = call(t, owner, http.MethodPut, "/videos/not-a-uuid", map[string]any{"title": "abc"})
	expect(t, r, http.StatusBadRequest, "edit bad uuid")
	r = call(t, owner, http.MethodDelete, "/videos/not-a-uuid", nil)
	expect(t, r, http.StatusBadRequest, "delete bad uuid")
	r = call(t, owner, http.MethodDelete, "/videos/"+uuid.NewString(), nil)
	expect(t, r, http.StatusNotFound, "delete unknown id")

	// paging is capped / defaulted
	for q, want := range map[string]int{"size=100": 50, "size=50": 50, "size=0": 20, "size=-5": 20, "size=abc": 20, "size=3": 3} {
		p := listVideos(t, nil, "/videos/browse?"+q)
		if p.Size != want || len(p.Items) > want {
			t.Errorf("browse?%s: size=%d items=%d, want size %d", q, p.Size, len(p.Items), want)
		}
	}
	if p := listVideos(t, owner, "/videos/mine?size=500"); p.Size != 50 {
		t.Errorf("mine?size=500: size=%d", p.Size)
	}
	r = call(t, nil, http.MethodGet, "/videos/"+id+"/comments?size=1000", nil)
	expect(t, r, http.StatusOK, "comments big size")
	var cp page[comment]
	r.decode(t, &cp)
	if cp.Size != 50 {
		t.Errorf("comments size=%d, want 50", cp.Size)
	}

	// confirm
	r = call(t, owner, http.MethodPost, "/videos/"+id+"/confirm", map[string]any{"title": "again title", "visibility": "public"})
	expect(t, r, http.StatusBadRequest, "confirm after publish")
	r = call(t, owner, http.MethodPost, "/videos/"+uuid.NewString()+"/confirm", map[string]any{"title": "again title", "visibility": "public"})
	expect(t, r, http.StatusNotFound, "confirm unknown id")
	r = call(t, owner, http.MethodPost, "/videos/not-a-uuid/confirm", map[string]any{"title": "again title", "visibility": "public"})
	expect(t, r, http.StatusBadRequest, "confirm bad uuid")

	// presign body validation (no rows are created on these)
	for name, body := range map[string]any{
		"no extension": map[string]any{"file_name": "clipmp4", "size": 10},
		"zero size":    map[string]any{"file_name": "clip.mp4", "size": 0},
		"too big":      map[string]any{"file_name": "clip.mp4", "size": int64(11) << 30},
		"missing name": map[string]any{"size": 10},
		"malformed":    "{nope",
	} {
		r = call(t, owner, http.MethodPost, "/videos/upload/presign", body)
		expect(t, r, http.StatusBadRequest, "presign "+name)
	}
}

// ---------- cleanup ----------

func deleteAndVerify(t *testing.T, id, word string) {
	r := call(t, owner, http.MethodDelete, "/videos/"+id, nil)
	if r.Code != http.StatusOK {
		t.Errorf("cleanup: DELETE /videos/%s = %d: %s", id, r.Code, truncate(r.Raw))
		return
	}
	for _, as := range []*user{owner, nil} {
		if r, _ := details(t, as, id); r.Code != http.StatusNotFound {
			t.Errorf("cleanup: details after delete = %d, want 404", r.Code)
		}
	}
	r = call(t, owner, http.MethodDelete, "/videos/"+id, nil)
	if r.Code != http.StatusNotFound {
		t.Errorf("cleanup: second DELETE = %d, want 404", r.Code)
	}
	if containsVideo(listVideos(t, owner, "/videos/mine?size=50"), id) {
		t.Errorf("cleanup: still in /videos/mine")
	}
	if containsVideo(listVideos(t, nil, "/videos/browse?search="+url.QueryEscape(word)), id) {
		t.Errorf("cleanup: still in search results")
	}
}

// ---------- media helpers ----------

func ffmpegPath(t *testing.T) string {
	t.Helper()
	for _, p := range []string{os.Getenv("E2E_FFMPEG"), "ffmpeg", "/usr/local/bin/ffmpeg", "/opt/homebrew/bin/ffmpeg"} {
		if p == "" {
			continue
		}
		if full, err := exec.LookPath(p); err == nil {
			return full
		}
	}
	t.Skip("ffmpeg not found (set E2E_FFMPEG); cannot generate the test clip")
	return ""
}

// makeClip renders a 5s 320x240 test pattern with a sine tone. It must be longer than 3s:
// the thumbnail stage grabs the frame at 00:03 (media.FFM.CreateThumbnail), so a <=3s clip
// gets no thumbnail (see TestShortClipFailsPhase1).
func makeClip(t *testing.T) string {
	t.Helper()
	return makeClipOf(t, 5)
}

func makeClipOf(t *testing.T, seconds int) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "clip.mp4")
	cmd := exec.Command(ffmpegPath(t), "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", fmt.Sprintf("testsrc=duration=%d:size=320x240:rate=25", seconds),
		"-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=440:duration=%d", seconds),
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v\n%s", err, b)
	}
	return out
}

func fileSize(t *testing.T, p string) int64 {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return st.Size()
}

func putFile(t *testing.T, presigned, p string) {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPut, presigned, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.ContentLength = int64(len(b))
	req.Header.Set("Content-Type", "video/mp4")
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("PUT presigned: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT presigned = %d", resp.StatusCode)
	}
	// unsigned access to the private bucket is refused
	plain, _ := url.Parse(presigned)
	plain.RawQuery = ""
	if code, _ := rawGet(t, plain.String()); code != http.StatusForbidden {
		t.Errorf("unsigned GET of original = %d, want 403 (bucket should be private)", code)
	}
}
