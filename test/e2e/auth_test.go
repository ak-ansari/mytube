//go:build e2e

package e2e

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAnonymousReadAllowed(t *testing.T) {
	r := call(t, nil, http.MethodGet, "/videos/browse", nil)
	expect(t, r, http.StatusOK, "anonymous browse")
	var p page[videoCard]
	r.decode(t, &p)
	if p.Page != 1 || p.Size != 20 {
		t.Errorf("default paging = page %d size %d, want 1/20", p.Page, p.Size)
	}
	if p.Items == nil {
		t.Errorf("items should be [] not null")
	}
	for _, v := range p.Items {
		if v.Status != "published" || v.Visibility == nil || *v.Visibility != "public" {
			t.Errorf("browse returned non-public video %s (status %s, visibility %s)", v.ID, v.Status, deref(v.Visibility))
		}
	}

	r = call(t, nil, http.MethodGet, "/channels/"+owner.id, nil)
	expect(t, r, http.StatusOK, "anonymous channel")
	r = call(t, nil, http.MethodGet, "/channels/"+owner.id+"/videos", nil)
	expect(t, r, http.StatusOK, "anonymous channel videos")
}

func TestProtectedRoutesRequireLogin(t *testing.T) {
	id := uuid.NewString()
	routes := []struct{ method, path string }{
		{http.MethodGet, "/users/me"},
		{http.MethodPut, "/users/me"},
		{http.MethodGet, "/users/me/subscriptions"},
		{http.MethodGet, "/videos/mine"},
		{http.MethodGet, "/videos/liked"},
		{http.MethodGet, "/videos/subscriptions"},
		{http.MethodPost, "/videos/upload/presign"},
		{http.MethodPost, "/videos/" + id + "/confirm"},
		{http.MethodPut, "/videos/" + id},
		{http.MethodDelete, "/videos/" + id},
		{http.MethodPut, "/videos/" + id + "/reaction"},
		{http.MethodPost, "/videos/" + id + "/comments"},
		{http.MethodDelete, "/videos/" + id + "/comments/" + uuid.NewString()},
		{http.MethodPost, "/channels/" + id + "/subscribe"},
		{http.MethodDelete, "/channels/" + id + "/subscribe"},
		{http.MethodGet, "/files/presign?key=x&size=1"},
	}
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			r := call(t, nil, rt.method, rt.path, map[string]any{})
			expect(t, r, http.StatusUnauthorized, "anonymous "+rt.method+" "+rt.path)
		})
	}
}

func TestInvalidTokenRejected(t *testing.T) {
	good, err := owner.Token()
	if err != nil {
		t.Fatal(err)
	}
	// flip a character in the signature
	parts := strings.Split(good, ".")
	sig := []byte(parts[2])
	if sig[0] == 'A' {
		sig[0] = 'B'
	} else {
		sig[0] = 'A'
	}
	tampered := parts[0] + "." + parts[1] + "." + string(sig)

	cases := map[string]string{
		"garbage bearer":   "Bearer not-a-jwt",
		"tampered sig":     "Bearer " + tampered,
		"wrong scheme":     "Basic " + good,
		"missing token":    "Bearer",
		"bearer only junk": "Token abc",
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			// even public routes reject a bad token so the client knows to re-login
			r := callAuth(t, header, http.MethodGet, "/videos/browse", nil)
			expect(t, r, http.StatusUnauthorized, "browse with "+name)
			r = callAuth(t, header, http.MethodGet, "/users/me", nil)
			expect(t, r, http.StatusUnauthorized, "/users/me with "+name)
		})
	}
}

func TestMeReturnsSyncedProfile(t *testing.T) {
	tok, _ := owner.Token()
	claims, err := jwtClaims(tok)
	if err != nil {
		t.Fatal(err)
	}
	r := call(t, owner, http.MethodGet, "/users/me", nil)
	expect(t, r, http.StatusOK, "GET /users/me")
	var p profile
	r.decode(t, &p)
	if p.ID != claims["sub"] {
		t.Errorf("id = %s, want token sub %v", p.ID, claims["sub"])
	}
	if p.UserName != claims["preferred_username"] {
		t.Errorf("user_name = %s, want %v", p.UserName, claims["preferred_username"])
	}
	if email, ok := claims["email"].(string); ok && email != "" && deref(p.Email) != email {
		t.Errorf("email = %s, want %s", deref(p.Email), email)
	}
	if p.ChannelName == nil || *p.ChannelName == "" {
		t.Errorf("channel_name is empty")
	}

	if other != nil {
		r := call(t, other, http.MethodGet, "/users/me", nil)
		expect(t, r, http.StatusOK, "GET /users/me (new user)")
		var np profile
		r.decode(t, &np)
		if np.ID != other.id || np.UserName != other.username {
			t.Errorf("new user synced as %s/%s, want %s/%s", np.ID, np.UserName, other.id, other.username)
		}
		// realm user has firstName/lastName, so channel_name comes from the "name" claim
		if deref(np.ChannelName) != "E2E Tester" {
			t.Errorf("new user channel_name = %s, want %q", deref(np.ChannelName), "E2E Tester")
		}
		if np.Subscribers != 0 || np.VideoCount != 0 {
			t.Errorf("new user has subscribers=%d video_count=%d, want 0/0", np.Subscribers, np.VideoCount)
		}
	}
}

func TestUpdateMe(t *testing.T) {
	// prefer the throwaway user so the demo profile is untouched; else restore demo afterwards
	u := other
	if u == nil {
		u = owner
		r := call(t, u, http.MethodGet, "/users/me", nil)
		expect(t, r, http.StatusOK, "GET /users/me")
		var orig profile
		r.decode(t, &orig)
		t.Cleanup(func() {
			body := map[string]any{"description": orig.Description}
			if orig.ChannelName != nil {
				body["channel_name"] = *orig.ChannelName
			}
			if orig.Description == nil {
				body["description"] = ""
			}
			call(t, u, http.MethodPut, "/users/me", body)
		})
	}

	name := "E2E Channel " + randWord(5)
	desc := "e2e description " + randWord(8)
	r := call(t, u, http.MethodPut, "/users/me", map[string]any{"channel_name": "  " + name + "  ", "description": desc})
	expect(t, r, http.StatusOK, "PUT /users/me")
	var p profile
	r.decode(t, &p)
	if deref(p.ChannelName) != name {
		t.Errorf("channel_name = %q, want %q (trimmed)", deref(p.ChannelName), name)
	}
	if deref(p.Description) != desc {
		t.Errorf("description = %q, want %q", deref(p.Description), desc)
	}

	// persisted
	r = call(t, u, http.MethodGet, "/users/me", nil)
	expect(t, r, http.StatusOK, "GET /users/me")
	r.decode(t, &p)
	if deref(p.ChannelName) != name || deref(p.Description) != desc {
		t.Errorf("after GET: %q/%q, want %q/%q", deref(p.ChannelName), deref(p.Description), name, desc)
	}
	// and visible on the public channel page
	r = call(t, nil, http.MethodGet, "/channels/"+u.id, nil)
	expect(t, r, http.StatusOK, "GET channel")
	var ch channel
	r.decode(t, &ch)
	if deref(ch.ChannelName) != name {
		t.Errorf("channel page name = %q, want %q", deref(ch.ChannelName), name)
	}

	// partial update keeps other fields
	r = call(t, u, http.MethodPut, "/users/me", map[string]any{"description": desc + "!"})
	expect(t, r, http.StatusOK, "PUT /users/me partial")
	r.decode(t, &p)
	if deref(p.ChannelName) != name || deref(p.Description) != desc+"!" {
		t.Errorf("partial update: %q/%q", deref(p.ChannelName), deref(p.Description))
	}

	// validation
	r = call(t, u, http.MethodPut, "/users/me", map[string]any{"channel_name": "ab"})
	expect(t, r, http.StatusBadRequest, "channel_name too short")
	r = call(t, u, http.MethodPut, "/users/me", map[string]any{"description": strings.Repeat("x", 1001)})
	expect(t, r, http.StatusBadRequest, "description too long")
	r = call(t, u, http.MethodPut, "/users/me", "{not json")
	expect(t, r, http.StatusBadRequest, "malformed body")
}

func TestChannelValidation(t *testing.T) {
	r := call(t, nil, http.MethodGet, "/channels/not-a-uuid", nil)
	expect(t, r, http.StatusBadRequest, "channel bad uuid")
	r = call(t, nil, http.MethodGet, "/channels/"+uuid.NewString(), nil)
	expect(t, r, http.StatusNotFound, "unknown channel")
	r = call(t, owner, http.MethodPost, "/channels/"+uuid.NewString()+"/subscribe", nil)
	expect(t, r, http.StatusNotFound, "subscribe to unknown channel")
	r = call(t, owner, http.MethodPost, "/channels/"+owner.id+"/subscribe", nil)
	expect(t, r, http.StatusBadRequest, "subscribe to own channel")
}
