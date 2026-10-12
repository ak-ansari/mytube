//go:build e2e

// Package e2e holds black-box end-to-end tests that drive the running API over HTTP.
// See main_test.go for prerequisites and the run command.
package e2e

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------- configuration ----------

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return strings.TrimRight(v, "/")
	}
	return def
}

var (
	apiURL      = envOr("E2E_API_URL", "http://localhost:8080")
	keycloakURL = envOr("E2E_KEYCLOAK_URL", "http://localhost:8180")
	realm       = envOr("E2E_KEYCLOAK_REALM", "mytube")
	clientID    = envOr("E2E_KEYCLOAK_CLIENT", "mytube-frontend")
	kcAdminUser = envOr("E2E_KEYCLOAK_ADMIN", "admin")
	kcAdminPass = envOr("E2E_KEYCLOAK_ADMIN_PASSWORD", "admin")
	demoUser    = envOr("E2E_DEMO_USER", "demo")
	demoPass    = envOr("E2E_DEMO_PASSWORD", "demo123")
	// pgDSN is only used to remove the temporary user's profile row in teardown (best effort).
	pgDSN = envOr("E2E_PG_DSN", "postgres://mytube:mytube@localhost:5432/mytube?sslmode=disable")
)

var httpClient = &http.Client{Timeout: 60 * time.Second}

// ---------- users / tokens ----------

// user is a Keycloak account whose access token is fetched (and refreshed) via direct grant.
type user struct {
	username, password string
	id                 string // Keycloak sub == users.id

	mu      sync.Mutex
	token   string
	expires time.Time
}

func newUser(username, password string) (*user, error) {
	u := &user{username: username, password: password}
	tok, err := u.Token()
	if err != nil {
		return nil, err
	}
	claims, err := jwtClaims(tok)
	if err != nil {
		return nil, err
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return nil, fmt.Errorf("token for %s has no sub", username)
	}
	u.id = sub
	return u, nil
}

// Token returns a valid access token, refreshing it shortly before expiry.
func (u *user) Token() (string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.token != "" && time.Now().Before(u.expires) {
		return u.token, nil
	}
	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {clientID},
		"username":   {u.username},
		"password":   {u.password},
	}
	tok, ttl, err := passwordGrant(keycloakURL+"/realms/"+realm+"/protocol/openid-connect/token", form)
	if err != nil {
		return "", fmt.Errorf("token for %s: %w", u.username, err)
	}
	u.token = tok
	u.expires = time.Now().Add(time.Duration(ttl)*time.Second - 30*time.Second)
	return tok, nil
}

func passwordGrant(endpoint string, form url.Values) (string, int, error) {
	resp, err := httpClient.PostForm(endpoint, form)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", 0, err
	}
	if out.ExpiresIn <= 60 {
		out.ExpiresIn = 60
	}
	return out.AccessToken, out.ExpiresIn, nil
}

func jwtClaims(tok string) (map[string]any, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a JWT")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(raw, &m)
}

// ---------- API client ----------

// envelope is util.Response: {message, success, data, status, error}.
type envelope struct {
	Message string          `json:"message"`
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Status  int             `json:"status"`
}

type apiResp struct {
	Code int
	Env  envelope
	Raw  []byte
}

// decode unmarshals the envelope's data into v.
func (r apiResp) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Env.Data, v); err != nil {
		t.Fatalf("decode data: %v\nbody: %s", err, r.Raw)
	}
}

// call performs an API request. as == nil means anonymous. body is JSON-encoded unless it is []byte.
func call(t *testing.T, as *user, method, path string, body any) apiResp {
	t.Helper()
	auth := ""
	if as != nil {
		tok, err := as.Token()
		if err != nil {
			t.Fatalf("get token: %v", err)
		}
		auth = "Bearer " + tok
	}
	return callAuth(t, auth, method, path, body)
}

// callAuth performs an API request with a raw Authorization header value ("" for none).
func callAuth(t *testing.T, authHeader, method, path string, body any) apiResp {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		switch b := body.(type) {
		case []byte:
			rdr = bytes.NewReader(b)
		case string:
			rdr = strings.NewReader(b)
		default:
			buf, err := json.Marshal(b)
			if err != nil {
				t.Fatalf("marshal body: %v", err)
			}
			rdr = bytes.NewReader(buf)
		}
	}
	req, err := http.NewRequest(method, apiURL+"/api"+path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := apiResp{Code: resp.StatusCode, Raw: raw}
	_ = json.Unmarshal(raw, &out.Env)
	return out
}

// expect fails the test unless the response has the wanted HTTP status and a matching envelope.
func expect(t *testing.T, r apiResp, want int, what string) {
	t.Helper()
	if r.Code != want {
		t.Fatalf("%s: status %d, want %d; body: %s", what, r.Code, want, truncate(r.Raw))
	}
	if r.Env.Status != want {
		t.Fatalf("%s: envelope status %d, want %d; body: %s", what, r.Env.Status, want, truncate(r.Raw))
	}
	if wantOK := want >= 200 && want < 400; r.Env.Success != wantOK {
		t.Fatalf("%s: envelope success=%v, want %v; body: %s", what, r.Env.Success, wantOK, truncate(r.Raw))
	}
}

// expectAny is expect that accepts any of several statuses.
func expectAny(t *testing.T, r apiResp, what string, want ...int) {
	t.Helper()
	for _, w := range want {
		if r.Code == w {
			return
		}
	}
	t.Fatalf("%s: status %d, want one of %v; body: %s", what, r.Code, want, truncate(r.Raw))
}

func truncate(b []byte) string {
	if len(b) > 800 {
		return string(b[:800]) + "…"
	}
	return string(b)
}

// rawGet fetches an absolute URL without any auth.
func rawGet(t *testing.T, u string) (int, []byte) {
	t.Helper()
	resp, err := httpClient.Get(u)
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// ---------- response shapes (mirrors internal/models/catalog.go) ----------

type channelSummary struct {
	ID          string  `json:"id"`
	UserName    string  `json:"user_name"`
	ChannelName *string `json:"channel_name"`
	Avatar      *string `json:"avatar"`
	AvatarURL   string  `json:"avatar_url"`
}

type channel struct {
	channelSummary
	Description  *string `json:"description"`
	Subscribers  int64   `json:"subscribers"`
	VideoCount   int64   `json:"video_count"`
	IsSubscribed bool    `json:"is_subscribed"`
}

type profile struct {
	channel
	Email *string `json:"email"`
}

type videoCard struct {
	ID           string         `json:"id"`
	Title        *string        `json:"title"`
	Description  *string        `json:"description"`
	Thumbnail    *string        `json:"thumbnail"`
	ThumbnailURL string         `json:"thumbnail_url"`
	Visibility   *string        `json:"visibility"`
	Status       string         `json:"status"`
	Views        int64          `json:"views"`
	Channel      channelSummary `json:"channel"`
}

type videoDetails struct {
	videoCard
	ManifestPath       *string  `json:"manifest_path"`
	ManifestURL        string   `json:"manifest_url"`
	AvailableQualities []string `json:"available_qualities"`
	Likes              int64    `json:"likes"`
	Dislikes           int64    `json:"dislikes"`
	MyReaction         int      `json:"my_reaction"`
	CommentCount       int64    `json:"comment_count"`
	Subscribers        int64    `json:"subscribers"`
	IsSubscribed       bool     `json:"is_subscribed"`
}

type comment struct {
	ID      string         `json:"id"`
	VideoID string         `json:"video_id"`
	Content string         `json:"content"`
	Author  channelSummary `json:"author"`
}

type page[T any] struct {
	Items   []T  `json:"items"`
	Page    int  `json:"page"`
	Size    int  `json:"size"`
	HasMore bool `json:"has_more"`
}

func listVideos(t *testing.T, as *user, path string) page[videoCard] {
	t.Helper()
	r := call(t, as, http.MethodGet, path, nil)
	expect(t, r, http.StatusOK, "GET "+path)
	var p page[videoCard]
	r.decode(t, &p)
	return p
}

func containsVideo(p page[videoCard], id string) bool {
	for _, v := range p.Items {
		if v.ID == id {
			return true
		}
	}
	return false
}

func details(t *testing.T, as *user, id string) (apiResp, videoDetails) {
	t.Helper()
	r := call(t, as, http.MethodGet, "/videos/"+id+"/details", nil)
	var d videoDetails
	if r.Code == http.StatusOK {
		r.decode(t, &d)
	}
	return r, d
}

func mustDetails(t *testing.T, as *user, id string) videoDetails {
	t.Helper()
	r, d := details(t, as, id)
	expect(t, r, http.StatusOK, "GET details")
	return d
}

// ---------- misc ----------

// poll calls fn every interval until it returns true or timeout passes; then it fails with fn's last reason.
func poll(t *testing.T, timeout, interval time.Duration, what string, fn func() (bool, string)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	last := ""
	for {
		ok, reason := fn()
		if ok {
			return
		}
		last = reason
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for %s; last state: %s", timeout, what, last)
		}
		time.Sleep(interval)
	}
}

// randWord returns a lowercase alphanumeric word that is safe as a single ES token.
func randWord(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		x, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b[i] = letters[x.Int64()]
	}
	return string(b)
}

func strp(s string) *string { return &s }

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
