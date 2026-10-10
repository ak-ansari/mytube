//go:build e2e

// End-to-end API tests. They are black-box: they only talk HTTP to the running API, Keycloak,
// the MinIO presigned URL and the HLS gateway (plus one best-effort Postgres DELETE in teardown
// to drop the temporary user's profile row).
//
// Prerequisites (all running locally):
//   - docker infra (`make`): Postgres, Redis, MinIO + nginx (:9002 S3, :80 HLS), ES, Keycloak (:8180)
//   - API (`make run-api`) and worker (`make run-worker`)
//   - ffmpeg on PATH or at /usr/local/bin/ffmpeg (to generate the test clip)
//
// Run from the repo root:
//
//	go test -tags e2e -count=1 -v -timeout 10m ./test/e2e/
//
// Env overrides: E2E_API_URL, E2E_KEYCLOAK_URL, E2E_KEYCLOAK_REALM, E2E_KEYCLOAK_CLIENT,
// E2E_KEYCLOAK_ADMIN(_PASSWORD), E2E_DEMO_USER, E2E_DEMO_PASSWORD, E2E_PG_DSN, E2E_FFMPEG.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	// owner is the demo user; it uploads the test video.
	owner *user
	// other is a throwaway Keycloak user created for ownership/permission tests (nil if that failed).
	other *user
	// otherSkipReason explains why other is nil.
	otherSkipReason string
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	if err := waitReachable(apiURL+"/api/videos/browse?size=1", 10*time.Second); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: API not reachable at %s (start it with `make run-api`): %v\n", apiURL, err)
		return 1
	}
	var err error
	owner, err = newUser(demoUser, demoPass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: cannot log in as %s via Keycloak direct grant at %s: %v\n", demoUser, keycloakURL, err)
		return 1
	}

	kcID, u, err := createTempUser()
	if err != nil {
		otherSkipReason = "second user unavailable (Keycloak admin API): " + err.Error()
		fmt.Fprintln(os.Stderr, "e2e: "+otherSkipReason)
	} else {
		other = u
	}

	code := m.Run()

	if kcID != "" {
		if err := deleteTempUser(kcID); err != nil {
			fmt.Fprintf(os.Stderr, "e2e: teardown: delete Keycloak user %s: %v\n", kcID, err)
		}
		if err := deleteProfileRow(kcID); err != nil {
			fmt.Fprintf(os.Stderr, "e2e: teardown: delete users row %s (best effort): %v\n", kcID, err)
		}
	}
	return code
}

// requireOther skips the test when the second user could not be created.
func requireOther(t *testing.T) *user {
	t.Helper()
	if other == nil {
		t.Skip("skipping: " + otherSkipReason)
	}
	return other
}

func waitReachable(u string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		resp, err := httpClient.Get(u)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// ---------- Keycloak admin ----------

func adminToken() (string, error) {
	tok, _, err := passwordGrant(keycloakURL+"/realms/master/protocol/openid-connect/token", url.Values{
		"grant_type": {"password"},
		"client_id":  {"admin-cli"},
		"username":   {kcAdminUser},
		"password":   {kcAdminPass},
	})
	return tok, err
}

func adminDo(method, path string, body any) (*http.Response, []byte, error) {
	tok, err := adminToken()
	if err != nil {
		return nil, nil, fmt.Errorf("admin token: %w", err)
	}
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, keycloakURL+"/admin/realms/"+realm+path, rdr)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b, nil
}

// createTempUser creates a random, fully set-up user and logs it in. kcID is returned even
// when login fails so teardown can still remove the account.
func createTempUser() (kcID string, u *user, err error) {
	name := "e2e" + randWord(10)
	pass := "Pw-" + randWord(16)
	resp, body, err := adminDo(http.MethodPost, "/users", map[string]any{
		"username":        name,
		"email":           name + "@e2e.mytube.local",
		"firstName":       "E2E",
		"lastName":        "Tester",
		"enabled":         true,
		"emailVerified":   true,
		"requiredActions": []string{},
		"credentials":     []map[string]any{{"type": "password", "value": pass, "temporary": false}},
	})
	if err != nil {
		return "", nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		return "", nil, fmt.Errorf("create user: status %d: %s", resp.StatusCode, body)
	}
	loc := resp.Header.Get("Location")
	kcID = loc[strings.LastIndex(loc, "/")+1:]
	if kcID == "" {
		return "", nil, fmt.Errorf("create user: no Location header")
	}
	u, err = newUser(name, pass)
	if err != nil {
		return kcID, nil, err
	}
	if u.id != kcID {
		return kcID, nil, fmt.Errorf("token sub %s != created user id %s", u.id, kcID)
	}
	return kcID, u, nil
}

func deleteTempUser(id string) error {
	resp, body, err := adminDo(http.MethodDelete, "/users/"+id, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// deleteProfileRow removes the users row the API created on first login (no API exists for it).
// Reactions, comments and subscriptions of that user cascade.
func deleteProfileRow(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, pgDSN)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}
