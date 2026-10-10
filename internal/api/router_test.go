package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ak-ansari/mytube/internal/testutil/apptest"
)

func TestCORS(t *testing.T) {
	e := apptest.New(t)
	tests := []struct {
		name, origin string
		allowed      bool
	}{
		{"frontend origin", "http://localhost:5173", true},
		{"other origin", "http://evil.test", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/api/videos/browse", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Access-Control-Request-Method", "PUT")
			req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
			w := httptest.NewRecorder()
			e.Router.ServeHTTP(w, req)
			got := w.Header().Get("Access-Control-Allow-Origin")
			if tt.allowed {
				if w.Code != http.StatusNoContent || got != tt.origin || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
					t.Fatalf("preflight = %d, allow-origin %q, headers %v", w.Code, got, w.Header())
				}
			} else if got != "" || w.Code != http.StatusForbidden {
				t.Fatalf("foreign origin got %d, allow-origin %q", w.Code, got)
			}
		})
	}
}

func TestAuthAppliesToAllAPIRoutes(t *testing.T) {
	e := apptest.New(t)
	// an invalid token is rejected even on public routes, so clients know to refresh
	w, env := e.Do(t, http.MethodGet, "/api/videos/browse", "garbage", nil)
	if w.Code != http.StatusUnauthorized || env.Message != "invalid or expired token" {
		t.Fatalf("got %d %+v", w.Code, env)
	}
	id, tok := e.User(t, "me")
	w, env = e.Do(t, http.MethodGet, "/api/users/me", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d %+v", w.Code, env)
	}
	if _, ok := e.Users.User(id); !ok {
		t.Fatal("user not synced")
	}
}
