package routes_test

import (
	"net/http"
	"testing"

	"github.com/ak-ansari/mytube/internal/testutil/apptest"
	"github.com/google/uuid"
)

// Every write/studio route must reject anonymous callers with 401 before touching services.
func TestProtectedRoutesRequireLogin(t *testing.T) {
	e := apptest.New(t)
	id, cid := uuid.NewString(), uuid.NewString()
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/videos/mine"},
		{http.MethodGet, "/api/videos/liked"},
		{http.MethodGet, "/api/videos/subscriptions"},
		{http.MethodPut, "/api/videos/" + id},
		{http.MethodDelete, "/api/videos/" + id},
		{http.MethodPut, "/api/videos/" + id + "/reaction"},
		{http.MethodPost, "/api/videos/" + id + "/comments"},
		{http.MethodDelete, "/api/videos/" + id + "/comments/" + cid},
		{http.MethodPost, "/api/videos/upload/presign"},
		{http.MethodPost, "/api/videos/" + id + "/confirm"},
		{http.MethodGet, "/api/users/me"},
		{http.MethodPut, "/api/users/me"},
		{http.MethodGet, "/api/users/me/subscriptions"},
		{http.MethodPost, "/api/channels/" + id + "/subscribe"},
		{http.MethodDelete, "/api/channels/" + id + "/subscribe"},
		{http.MethodGet, "/api/files/presign?key=k&size=1"},
	}
	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			w, env := e.Do(t, r.method, r.path, "", map[string]string{})
			if w.Code != http.StatusUnauthorized || env.Message != "login required" || env.Success {
				t.Fatalf("got %d %+v", w.Code, env)
			}
		})
	}
}

// Read routes stay public (anonymous requests are not rejected by auth).
func TestPublicRoutes(t *testing.T) {
	e := apptest.New(t)
	owner, _ := e.User(t, "owner")
	vid := e.SeedVideo(owner, apptest.VideoOpts{})
	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/videos/browse"},
		{http.MethodGet, "/api/videos/" + vid.String() + "/details"},
		{http.MethodGet, "/api/videos/" + vid.String() + "/related"},
		{http.MethodPost, "/api/videos/" + vid.String() + "/view"},
		{http.MethodGet, "/api/videos/" + vid.String() + "/comments"},
		{http.MethodGet, "/api/channels/" + owner.String()},
		{http.MethodGet, "/api/channels/" + owner.String() + "/videos"},
		{http.MethodGet, "/api/videos/search?search=x"},
		{http.MethodGet, "/api/videos/suggest?search=x"},
		{http.MethodGet, "/api/videos/url?key=thumbnails/" + vid.String() + "/" + vid.String() + ".jpeg"},
		{http.MethodGet, "/api/files/url?key=avatars/" + owner.String() + "/a.png"},
	}
	for _, r := range routes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			w, env := e.Do(t, r.method, r.path, "", nil)
			if w.Code != http.StatusOK {
				t.Fatalf("got %d %+v", w.Code, env)
			}
		})
	}
}

func TestUnknownRouteIs404(t *testing.T) {
	e := apptest.New(t)
	w, _ := e.Do(t, http.MethodGet, "/api/nope", "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d", w.Code)
	}
}
