package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ak-ansari/mytube/internal/index"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/services"
	"github.com/ak-ansari/mytube/internal/testutil/apptest"
	"github.com/ak-ansari/mytube/internal/testutil/fakes"
	"github.com/google/uuid"
)

var errBoom = errors.New("boom")

type page struct {
	Items   []json.RawMessage `json:"items"`
	Page    int               `json:"page"`
	Size    int               `json:"size"`
	HasMore bool              `json:"has_more"`
}

func expect(t *testing.T, code int, env apptest.Envelope, status int, msg string) {
	t.Helper()
	if code != status {
		t.Fatalf("status = %d, want %d (%+v)", code, status, env)
	}
	if env.Status != status || env.Success != (status < 400) {
		t.Errorf("envelope status/success = %d/%v", env.Status, env.Success)
	}
	if msg != "" && env.Message != msg {
		t.Errorf("message = %q, want %q", env.Message, msg)
	}
}

func TestBrowseEndpoint(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.User(t, "owner")
	for i := 0; i < 3; i++ {
		e.SeedVideo(owner, apptest.VideoOpts{})
	}
	e.SeedVideo(owner, apptest.VideoOpts{Visibility: models.VisibilityPrivate})

	w, env := e.Do(t, http.MethodGet, "/api/videos/browse?page=1&size=2", "", nil)
	expect(t, w.Code, env, 200, "videos fetched successfully")
	var p page
	apptest.DecodeData(t, env, &p)
	if len(p.Items) != 2 || !p.HasMore || p.Page != 1 || p.Size != 2 {
		t.Errorf("page = %+v", p)
	}
	var card models.VideoCard
	_ = json.Unmarshal(p.Items[0], &card)
	if !strings.HasPrefix(card.ThumbnailURL, "https://get.test/thumbnails/") {
		t.Errorf("thumbnail_url = %q", card.ThumbnailURL)
	}

	w, env = e.Do(t, http.MethodGet, "/api/videos/browse?size=500", "", nil)
	apptest.DecodeData(t, env, &p)
	if p.Size != 50 || len(p.Items) != 3 || p.HasMore {
		t.Errorf("capped page = %+v", p)
	}

	// empty result is [] not null
	w, env = e.Do(t, http.MethodGet, "/api/videos/browse?search=zzz", "", nil)
	expect(t, w.Code, env, 200, "")
	if !strings.Contains(string(env.Data), `"items":[]`) {
		t.Errorf("data = %s", env.Data)
	}

	e.Index.Errs.Set("SearchVideo", errBoom)
	w, env = e.Do(t, http.MethodGet, "/api/videos/browse?search=x", "", nil)
	expect(t, w.Code, env, 500, "something went wrong")
}

func TestDetailsEndpoint(t *testing.T) {
	e := apptest.New(t)
	owner, ownerTok := e.User(t, "owner")
	_, otherTok := e.User(t, "other")
	pub := e.SeedVideo(owner, apptest.VideoOpts{Manifest: "segments/x/master.m3u8"})
	priv := e.SeedVideo(owner, apptest.VideoOpts{Visibility: models.VisibilityPrivate})

	tests := []struct {
		name   string
		path   string
		token  string
		status int
		msg    string
	}{
		{"public anon", "/api/videos/" + pub.String() + "/details", "", 200, "video fetched successfully"},
		{"private anon", "/api/videos/" + priv.String() + "/details", "", 404, "not found"},
		{"private other", "/api/videos/" + priv.String() + "/details", otherTok, 404, "not found"},
		{"private owner", "/api/videos/" + priv.String() + "/details", ownerTok, 200, ""},
		{"missing", "/api/videos/" + uuid.NewString() + "/details", "", 404, "not found"},
		{"bad uuid", "/api/videos/nope/details", "", 400, "invalid id"},
		{"bad token", "/api/videos/" + pub.String() + "/details", "garbage", 401, "invalid or expired token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, env := e.Do(t, http.MethodGet, tt.path, tt.token, nil)
			expect(t, w.Code, env, tt.status, tt.msg)
		})
	}

	_, env := e.Do(t, http.MethodGet, "/api/videos/"+pub.String()+"/details", "", nil)
	var d models.VideoDetails
	apptest.DecodeData(t, env, &d)
	if !strings.HasPrefix(d.ManifestURL, "http://gateway.test/videos/") || !strings.HasSuffix(d.ManifestURL, "/"+pub.String()+"/master.m3u8") {
		t.Errorf("manifest_url = %q", d.ManifestURL)
	}
}

func TestRelatedAndChannelVideosEndpoints(t *testing.T) {
	e := apptest.New(t)
	alice, _ := e.User(t, "alice")
	bob, _ := e.User(t, "bob")
	a := e.SeedVideo(alice, apptest.VideoOpts{})
	e.SeedVideo(alice, apptest.VideoOpts{})
	e.SeedVideo(bob, apptest.VideoOpts{})

	w, env := e.Do(t, http.MethodGet, "/api/videos/"+a.String()+"/related", "", nil)
	expect(t, w.Code, env, 200, "")
	var p page
	apptest.DecodeData(t, env, &p)
	if len(p.Items) != 2 {
		t.Errorf("related = %d items", len(p.Items))
	}
	w, env = e.Do(t, http.MethodGet, "/api/videos/x/related", "", nil)
	expect(t, w.Code, env, 400, "invalid id")

	w, env = e.Do(t, http.MethodGet, "/api/channels/"+bob.String()+"/videos", "", nil)
	expect(t, w.Code, env, 200, "channel videos fetched successfully")
	apptest.DecodeData(t, env, &p)
	if len(p.Items) != 1 {
		t.Errorf("channel videos = %d items", len(p.Items))
	}
	w, env = e.Do(t, http.MethodGet, "/api/channels/x/videos", "", nil)
	expect(t, w.Code, env, 400, "invalid id")
	e.Catalog.Errs.Set("ListPublished", errBoom)
	for _, path := range []string{"/api/channels/" + bob.String() + "/videos", "/api/videos/" + a.String() + "/related"} {
		w, env = e.Do(t, http.MethodGet, path, "", nil)
		expect(t, w.Code, env, 500, "something went wrong")
	}
}

func TestStudioLists(t *testing.T) {
	e := apptest.New(t)
	me, tok := e.User(t, "me")
	creator, _ := e.User(t, "creator")
	e.SeedVideo(me, apptest.VideoOpts{Status: models.StatusUploadPending, Title: new(string)})
	liked := e.SeedVideo(creator, apptest.VideoOpts{})
	_ = e.Catalog.SetReaction(context.Background(), liked, me, 1)
	_ = e.Users.Subscribe(context.Background(), me, creator)

	for _, path := range []string{"/api/videos/mine", "/api/videos/liked", "/api/videos/subscriptions"} {
		t.Run(path, func(t *testing.T) {
			w, env := e.Do(t, http.MethodGet, path, tok, nil)
			expect(t, w.Code, env, 200, "videos fetched successfully")
			var p page
			apptest.DecodeData(t, env, &p)
			if len(p.Items) != 1 {
				t.Errorf("%s = %d items", path, len(p.Items))
			}
			w, env = e.Do(t, http.MethodGet, path, "", nil)
			expect(t, w.Code, env, 401, "login required")
		})
	}
	e.Catalog.Errs.Set("ListByOwner", errBoom)
	e.Catalog.Errs.Set("ListPublished", errBoom)
	for _, path := range []string{"/api/videos/mine", "/api/videos/liked", "/api/videos/subscriptions"} {
		w, env := e.Do(t, http.MethodGet, path, tok, nil)
		expect(t, w.Code, env, 500, "")
	}
}

func TestUpdateAndDeleteVideoEndpoints(t *testing.T) {
	e := apptest.New(t)
	owner, ownerTok := e.User(t, "owner")
	_, otherTok := e.User(t, "other")
	id := e.SeedVideo(owner, apptest.VideoOpts{})
	path := "/api/videos/" + id.String()

	tests := []struct {
		name   string
		token  string
		path   string
		body   any
		status int
		msg    string
	}{
		{"anonymous", "", path, map[string]string{"title": "New title"}, 401, "login required"},
		{"not owner", otherTok, path, map[string]string{"title": "New title"}, 403, "you are not allowed to do that"},
		{"bad id", ownerTok, "/api/videos/nope", map[string]string{}, 400, "invalid id"},
		{"bad json", ownerTok, path, "{", 400, "invalid request body"},
		{"short title", ownerTok, path, map[string]string{"title": "ab"}, 400, "title must be 3-100 characters"},
		{"bad visibility", ownerTok, path, map[string]string{"visibility": "secret"}, 400, "visibility must be public, private or unlisted"},
		{"missing", ownerTok, "/api/videos/" + uuid.NewString(), map[string]string{}, 404, "not found"},
		{"ok", ownerTok, path, map[string]string{"title": "New title", "visibility": "unlisted"}, 200, "video updated successfully"},
	}
	for _, tt := range tests {
		t.Run("PUT "+tt.name, func(t *testing.T) {
			w, env := e.Do(t, http.MethodPut, tt.path, tt.token, tt.body)
			expect(t, w.Code, env, tt.status, tt.msg)
		})
	}
	d, _ := e.Catalog.Video(id)
	if *d.Title != "New title" || *d.Visibility != models.VisibilityUnlisted {
		t.Errorf("video after update = %+v", d.VideoCard)
	}

	for _, tt := range []struct {
		name   string
		token  string
		path   string
		status int
	}{
		{"anonymous", "", path, 401},
		{"not owner", otherTok, path, 403},
		{"bad id", ownerTok, "/api/videos/nope", 400},
		{"owner", ownerTok, path, 200},
		{"already gone", ownerTok, path, 404},
	} {
		t.Run("DELETE "+tt.name, func(t *testing.T) {
			w, env := e.Do(t, http.MethodDelete, tt.path, tt.token, nil)
			expect(t, w.Code, env, tt.status, "")
		})
	}
	if len(e.Store.DeletedPrefixes) != 4 {
		t.Errorf("deleted prefixes = %v", e.Store.DeletedPrefixes)
	}
}

func TestRecordViewEndpoint(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.User(t, "owner")
	viewer, tok := e.User(t, "viewer")
	id := e.SeedVideo(owner, apptest.VideoOpts{})
	path := "/api/videos/" + id.String() + "/view"

	for i := 0; i < 2; i++ {
		w, env := e.Do(t, http.MethodPost, path, "", nil)
		expect(t, w.Code, env, 200, "view recorded")
	}
	// X-Forwarded-For is not trusted: same socket peer -> still deduped
	w, env := e.Do(t, http.MethodPost, path, "", nil)
	expect(t, w.Code, env, 200, "")
	w, env = e.Do(t, http.MethodPost, path, tok, nil)
	expect(t, w.Code, env, 200, "")
	d, _ := e.Catalog.Video(id)
	if d.Views != 2 {
		t.Errorf("views = %d, want 2 (one anonymous IP + one user)", d.Views)
	}
	if !e.Cache.Has("view:" + id.String() + ":" + viewer.String()) {
		t.Error("logged-in views should be keyed by user id")
	}
	w, env = e.Do(t, http.MethodPost, "/api/videos/"+uuid.NewString()+"/view", "", nil)
	expect(t, w.Code, env, 404, "")
	w, env = e.Do(t, http.MethodPost, "/api/videos/x/view", "", nil)
	expect(t, w.Code, env, 400, "invalid id")
}

func TestXForwardedForIgnored(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.User(t, "owner")
	id := e.SeedVideo(owner, apptest.VideoOpts{})
	for _, ip := range []string{"1.1.1.1", "2.2.2.2"} {
		req, _ := http.NewRequest(http.MethodPost, "/api/videos/"+id.String()+"/view", nil)
		req.Header.Set("X-Forwarded-For", ip)
		req.RemoteAddr = "10.0.0.1:1234"
		w := newRecorder()
		e.Router.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("status %d", w.Code)
		}
	}
	if d, _ := e.Catalog.Video(id); d.Views != 1 {
		t.Errorf("views = %d: spoofed X-Forwarded-For bypassed dedup", d.Views)
	}
}

func TestReactionEndpoint(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.User(t, "owner")
	_, tok := e.User(t, "fan")
	id := e.SeedVideo(owner, apptest.VideoOpts{})
	path := "/api/videos/" + id.String() + "/reaction"

	tests := []struct {
		name   string
		token  string
		body   any
		status int
		msg    string
	}{
		{"anonymous", "", map[string]int{"value": 1}, 401, "login required"},
		{"missing value", tok, map[string]any{}, 400, "value is required (1, -1 or 0)"},
		{"bad json", tok, "nope", 400, "value is required (1, -1 or 0)"},
		{"out of range", tok, map[string]int{"value": 5}, 400, "value must be 1, -1 or 0"},
		{"like", tok, map[string]int{"value": 1}, 200, "reaction saved"},
		{"clear", tok, map[string]int{"value": 0}, 200, "reaction saved"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, env := e.Do(t, http.MethodPut, path, tt.token, tt.body)
			expect(t, w.Code, env, tt.status, tt.msg)
		})
	}
	_, env := e.Do(t, http.MethodPut, path, tok, map[string]int{"value": -1})
	var d models.VideoDetails
	apptest.DecodeData(t, env, &d)
	if d.MyReaction != -1 || d.Dislikes != 1 {
		t.Errorf("details = my %d dislikes %d", d.MyReaction, d.Dislikes)
	}
	w, env := e.Do(t, http.MethodPut, "/api/videos/x/reaction", tok, map[string]int{"value": 1})
	expect(t, w.Code, env, 400, "invalid id")
}

func TestCommentEndpoints(t *testing.T) {
	e := apptest.New(t)
	owner, ownerTok := e.User(t, "owner")
	_, aliceTok := e.User(t, "alice")
	_, bobTok := e.User(t, "bob")
	id := e.SeedVideo(owner, apptest.VideoOpts{})
	base := "/api/videos/" + id.String() + "/comments"

	w, env := e.Do(t, http.MethodPost, base, "", map[string]string{"content": "hi"})
	expect(t, w.Code, env, 401, "login required")
	w, env = e.Do(t, http.MethodPost, base, aliceTok, "{")
	expect(t, w.Code, env, 400, "invalid request body")
	w, env = e.Do(t, http.MethodPost, base, aliceTok, map[string]string{"content": "  "})
	expect(t, w.Code, env, 400, "comment must be 1-2000 characters")
	w, env = e.Do(t, http.MethodPost, "/api/videos/x/comments", aliceTok, map[string]string{"content": "hi"})
	expect(t, w.Code, env, 400, "invalid id")

	var ids []string
	for _, tok := range []string{aliceTok, aliceTok, bobTok} {
		w, env = e.Do(t, http.MethodPost, base, tok, map[string]string{"content": "hello"})
		expect(t, w.Code, env, 200, "comment added")
		var c models.Comment
		apptest.DecodeData(t, env, &c)
		ids = append(ids, c.ID.String())
	}

	w, env = e.Do(t, http.MethodGet, base+"?size=2", "", nil)
	expect(t, w.Code, env, 200, "comments fetched successfully")
	var p page
	apptest.DecodeData(t, env, &p)
	if len(p.Items) != 2 || !p.HasMore {
		t.Errorf("comments page = %+v", p)
	}
	w, env = e.Do(t, http.MethodGet, "/api/videos/"+uuid.NewString()+"/comments", "", nil)
	expect(t, w.Code, env, 404, "")
	w, env = e.Do(t, http.MethodGet, "/api/videos/x/comments", "", nil)
	expect(t, w.Code, env, 400, "invalid id")

	del := func(cid string) string { return base + "/" + cid }
	tests := []struct {
		name   string
		path   string
		token  string
		status int
	}{
		{"anonymous", del(ids[0]), "", 401},
		{"bad comment id", del("x"), aliceTok, 400},
		{"bad video id", "/api/videos/x/comments/" + ids[0], aliceTok, 400},
		{"stranger", del(ids[0]), bobTok, 403},
		{"author", del(ids[0]), aliceTok, 200},
		{"video owner", del(ids[2]), ownerTok, 200},
		{"gone", del(ids[0]), aliceTok, 404},
		{"wrong video", "/api/videos/" + uuid.NewString() + "/comments/" + ids[1], aliceTok, 404},
	}
	for _, tt := range tests {
		t.Run("DELETE "+tt.name, func(t *testing.T) {
			w, env := e.Do(t, http.MethodDelete, tt.path, tt.token, nil)
			expect(t, w.Code, env, tt.status, "")
		})
	}
}

func TestUserEndpoints(t *testing.T) {
	e := apptest.New(t)
	me, tok := e.User(t, "me")
	creator, _ := e.User(t, "creator")

	w, env := e.Do(t, http.MethodGet, "/api/users/me", "", nil)
	expect(t, w.Code, env, 401, "login required")
	w, env = e.Do(t, http.MethodGet, "/api/users/me", tok, nil)
	expect(t, w.Code, env, 200, "profile fetched successfully")
	var prof models.Profile
	apptest.DecodeData(t, env, &prof)
	if prof.ID != me {
		t.Errorf("profile id = %s", prof.ID)
	}
	// the token sync refreshed the identity fields
	if prof.UserName != "demo" || prof.Email == nil || *prof.Email != "demo@example.com" {
		t.Errorf("profile = %+v", prof)
	}

	w, env = e.Do(t, http.MethodPut, "/api/users/me", tok, "{")
	expect(t, w.Code, env, 400, "invalid request body")
	w, env = e.Do(t, http.MethodPut, "/api/users/me", tok, map[string]string{"channel_name": "x"})
	expect(t, w.Code, env, 400, "channel name must be 3-60 characters")
	w, env = e.Do(t, http.MethodPut, "/api/users/me", tok, map[string]string{"channel_name": "Brand New", "avatar": "avatars/a.png"})
	expect(t, w.Code, env, 200, "profile updated successfully")
	apptest.DecodeData(t, env, &prof)
	if *prof.ChannelName != "Brand New" || prof.AvatarURL != "https://get.test/avatars/a.png" {
		t.Errorf("updated profile = %+v", prof)
	}

	chPath := "/api/channels/" + creator.String()
	w, env = e.Do(t, http.MethodGet, chPath, "", nil)
	expect(t, w.Code, env, 200, "channel fetched successfully")
	w, env = e.Do(t, http.MethodGet, "/api/channels/"+uuid.NewString(), "", nil)
	expect(t, w.Code, env, 404, "not found")
	w, env = e.Do(t, http.MethodGet, "/api/channels/x", "", nil)
	expect(t, w.Code, env, 400, "invalid id")

	w, env = e.Do(t, http.MethodPost, chPath+"/subscribe", "", nil)
	expect(t, w.Code, env, 401, "login required")
	w, env = e.Do(t, http.MethodPost, "/api/channels/"+me.String()+"/subscribe", tok, nil)
	expect(t, w.Code, env, 400, "you cannot subscribe to your own channel")
	w, env = e.Do(t, http.MethodPost, "/api/channels/x/subscribe", tok, nil)
	expect(t, w.Code, env, 400, "invalid id")
	w, env = e.Do(t, http.MethodPost, chPath+"/subscribe", tok, nil)
	expect(t, w.Code, env, 200, "subscribed")
	var ch models.Channel
	apptest.DecodeData(t, env, &ch)
	if !ch.IsSubscribed || ch.Subscribers != 1 {
		t.Errorf("channel after subscribe = %+v", ch)
	}
	w, env = e.Do(t, http.MethodGet, chPath, tok, nil)
	apptest.DecodeData(t, env, &ch)
	if !ch.IsSubscribed {
		t.Error("GET channel should report is_subscribed for the viewer")
	}

	w, env = e.Do(t, http.MethodGet, "/api/users/me/subscriptions", tok, nil)
	expect(t, w.Code, env, 200, "subscriptions fetched successfully")
	var subs []models.Channel
	apptest.DecodeData(t, env, &subs)
	if len(subs) != 1 || subs[0].ID != creator {
		t.Errorf("subscriptions = %+v", subs)
	}

	w, env = e.Do(t, http.MethodDelete, chPath+"/subscribe", tok, nil)
	expect(t, w.Code, env, 200, "unsubscribed")
	apptest.DecodeData(t, env, &ch)
	if ch.IsSubscribed || ch.Subscribers != 0 {
		t.Errorf("channel after unsubscribe = %+v", ch)
	}
	w, env = e.Do(t, http.MethodDelete, "/api/channels/x/subscribe", tok, nil)
	expect(t, w.Code, env, 400, "invalid id")

	e.Users.Errs.Set("ListSubscriptions", errBoom)
	w, env = e.Do(t, http.MethodGet, "/api/users/me/subscriptions", tok, nil)
	expect(t, w.Code, env, 500, "something went wrong")
	e.Users.Errs.Set("Unsubscribe", errBoom)
	w, env = e.Do(t, http.MethodDelete, chPath+"/subscribe", tok, nil)
	expect(t, w.Code, env, 500, "")
	e.Users.Errs.Set("GetProfile", errBoom)
	w, env = e.Do(t, http.MethodGet, "/api/users/me", tok, nil)
	expect(t, w.Code, env, 500, "")
}

func TestUploadFlowEndpoints(t *testing.T) {
	e := apptest.New(t)
	owner, tok := e.User(t, "owner")
	_, otherTok := e.User(t, "other")

	for _, tt := range []struct {
		name   string
		token  string
		body   any
		status int
	}{
		{"anonymous", "", map[string]any{"file_name": "a.mp4", "size": 10}, 401},
		{"missing size", tok, map[string]any{"file_name": "a.mp4"}, 400},
		{"too big", tok, map[string]any{"file_name": "a.mp4", "size": int64(10737418241)}, 400},
		{"short name", tok, map[string]any{"file_name": "a", "size": 10}, 400},
		{"no extension", tok, map[string]any{"file_name": "video", "size": 10}, 400},
	} {
		t.Run("presign "+tt.name, func(t *testing.T) {
			w, env := e.Do(t, http.MethodPost, "/api/videos/upload/presign", tt.token, tt.body)
			expect(t, w.Code, env, tt.status, "")
		})
	}

	w, env := e.Do(t, http.MethodPost, "/api/videos/upload/presign", tok, map[string]any{"file_name": "clip.mp4", "size": 10})
	expect(t, w.Code, env, 201, "file uploaded successfully")
	var res services.UploadResult
	apptest.DecodeData(t, env, &res)
	if res.ID == "" || res.Key != "originals/"+res.ID+"/original.mp4" || !strings.HasPrefix(res.Url, "https://put.test/") {
		t.Fatalf("upload result = %+v", res)
	}
	v, _ := e.Videos.Get(context.Background(), res.ID)
	if v.UserID != owner {
		t.Errorf("uploader = %s", v.UserID)
	}
	// the catalog (RequireVideoOwner) reads from the catalog repo; mirror the row there
	vid := uuid.MustParse(res.ID)
	e.Catalog.Put(&models.VideoDetails{VideoCard: models.VideoCard{ID: vid, Status: models.StatusUploadPending, Channel: models.ChannelSummary{ID: owner}}})

	confirm := "/api/videos/" + res.ID + "/confirm"
	good := map[string]string{"title": "My upload", "visibility": "public", "description": "d"}
	for _, tt := range []struct {
		name   string
		path   string
		token  string
		body   any
		status int
	}{
		{"anonymous", confirm, "", good, 401},
		{"not owner", confirm, otherTok, good, 403},
		{"bad id", "/api/videos/x/confirm", tok, good, 400},
		{"missing video", "/api/videos/" + uuid.NewString() + "/confirm", tok, good, 404},
		{"bad visibility", confirm, tok, map[string]string{"title": "My upload", "visibility": "everyone"}, 400},
		{"missing title", confirm, tok, map[string]string{"visibility": "public"}, 400},
		{"foreign thumbnail", confirm, tok, map[string]string{"title": "My upload", "visibility": "public", "thumbnail": "avatars/x.png"}, 400},
		{"whitespace title", confirm, tok, map[string]string{"title": "      ", "visibility": "public"}, 400},
		{"ok", confirm, tok, good, 200},
	} {
		t.Run("confirm "+tt.name, func(t *testing.T) {
			w, env := e.Do(t, http.MethodPost, tt.path, tt.token, tt.body)
			expect(t, w.Code, env, tt.status, "")
		})
	}
	v, _ = e.Videos.Get(context.Background(), res.ID)
	if v.Title == nil || *v.Title != "My upload" {
		t.Errorf("video after confirm = %+v", v)
	}
	// past phase 1 -> 400
	_ = e.Videos.UpdateState(context.Background(), res.ID, 5, models.StatusTranscoded)
	w, env = e.Do(t, http.MethodPost, confirm, tok, good)
	expect(t, w.Code, env, 400, "video already confirmed")
}

func TestLegacyVideoEndpoints(t *testing.T) {
	e := apptest.New(t)
	ctx := context.Background()
	id := uuid.New()
	e.Videos.Put(&models.Video{ID: id, Title: fakes.Ptr("dogs"), Status: models.StatusPublished, Visibility: fakes.Ptr(models.VisibilityPublic)})
	e.Metas.Put(&models.VideoMetadata{VideoId: id, OriginalObjectKey: "originals/" + id.String() + "/original.mp4"})
	_ = e.Index.InsertVideo(ctx, &models.Video{ID: id, Title: fakes.Ptr("dogs")})
	e.Index.Suggestions = []index.Suggestion{{ID: id.String(), Text: "dogs"}}

	tests := []struct {
		name   string
		path   string
		status int
		data   string
	}{
		{"GET /videos/:id", "/api/videos/" + id.String(), 200, `"originals/` + id.String() + `/original.mp4"`},
		{"GET /videos/:id missing", "/api/videos/" + uuid.NewString(), 500, "null"},
		{"GET /videos/url", "/api/videos/url?key=a/b", 200, `"https://get.test/a/b"`},
		{"GET /videos/suggest", "/api/videos/suggest?search=do", 200, `["dogs"]`},
		{"GET /video-meta/:id", "/api/video-meta/" + id.String(), 200, ""},
		{"GET /files/url", "/api/files/url?key=thumbnails/x.png", 200, `"https://get.test/thumbnails/x.png"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, env := e.Do(t, http.MethodGet, tt.path, "", nil)
			expect(t, w.Code, env, tt.status, "")
			if tt.data != "" && string(env.Data) != tt.data {
				t.Errorf("data = %s, want %s", env.Data, tt.data)
			}
		})
	}

	w, env := e.Do(t, http.MethodGet, "/api/videos/search?search=dogs", "", nil)
	expect(t, w.Code, env, 200, "videos retrieved successfully")
	var vids []models.Video
	apptest.DecodeData(t, env, &vids)
	if len(vids) != 1 || vids[0].ID != id {
		t.Errorf("search = %+v", vids)
	}

	e.Index.Errs.Set("SearchVideo", errBoom)
	e.Index.Errs.Set("SuggestVideos", errBoom)
	e.Store.Errs.Set("GetUrl", errBoom)
	for _, p := range []string{"/api/videos/search?search=x", "/api/videos/suggest?search=x", "/api/videos/url?key=k", "/api/files/url?key=k"} {
		w, env := e.Do(t, http.MethodGet, p, "", nil)
		expect(t, w.Code, env, 500, "something went wrong")
	}
	e.Metas.Errs.Set("Get", errBoom)
	w, env = e.Do(t, http.MethodGet, "/api/video-meta/"+id.String(), "", nil)
	expect(t, w.Code, env, 500, "")
}

func TestFilePresignEndpoint(t *testing.T) {
	e := apptest.New(t)
	_, tok := e.User(t, "me")
	w, env := e.Do(t, http.MethodGet, "/api/files/presign?key=thumbnails/x.png&size=10", "", nil)
	expect(t, w.Code, env, 401, "login required")
	w, env = e.Do(t, http.MethodGet, "/api/files/presign?key=thumbnails/x.png&size=abc", tok, nil)
	expect(t, w.Code, env, 400, "Size is not a valid number")
	w, env = e.Do(t, http.MethodGet, "/api/files/presign?key=thumbnails/x.png&size=10", tok, nil)
	expect(t, w.Code, env, 200, "")
	var out map[string]string
	apptest.DecodeData(t, env, &out)
	if out["key"] != "thumbnails/x.png" || out["url"] != "https://put.test/thumbnails/x.png?size=10" {
		t.Errorf("presign = %v", out)
	}
	e.Store.Errs.Set("GerPreSignedPutUrl", errBoom)
	w, env = e.Do(t, http.MethodGet, "/api/files/presign?key=k&size=1", tok, nil)
	expect(t, w.Code, env, 500, "")
}

func newRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }
