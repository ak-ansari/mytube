// Package authtest runs a fake Keycloak JWKS endpoint (httptest) and mints RS256 access tokens,
// so auth.KeycloakVerifier and middleware.Authenticate can be unit-tested without Keycloak.
package authtest

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/auth"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

const (
	Realm    = "mytube"
	ClientID = "mytube-frontend"
	KeyID    = "test-key"
)

var (
	keysOnce       sync.Once
	signKey, rogue *rsa.PrivateKey
	keyGenErr      error
)

func keys(t testing.TB) (*rsa.PrivateKey, *rsa.PrivateKey) {
	t.Helper()
	keysOnce.Do(func() {
		signKey, keyGenErr = rsa.GenerateKey(rand.Reader, 2048)
		if keyGenErr == nil {
			rogue, keyGenErr = rsa.GenerateKey(rand.Reader, 2048)
		}
	})
	if keyGenErr != nil {
		t.Fatalf("generate rsa key: %v", keyGenErr)
	}
	return signKey, rogue
}

// Keycloak is a fake realm: a JWKS server plus a token minter.
type Keycloak struct {
	Server *httptest.Server
	key    *rsa.PrivateKey
	rogue  *rsa.PrivateKey
}

// Issuer is the "iss" the verifier expects.
func (k *Keycloak) Issuer() string { return k.Server.URL + "/realms/" + Realm }

// Config is the KEYCLOAK config section pointing at this fake.
func (k *Keycloak) Config() config.Keycloak {
	return config.Keycloak{Url: k.Server.URL, Realm: Realm, ClientId: ClientID}
}

// New starts the fake realm; it is closed when the test ends.
func New(t testing.TB) *Keycloak {
	t.Helper()
	key, rogue := keys(t)
	jwks := map[string]any{"keys": []map[string]string{{
		"kty": "RSA",
		"kid": KeyID,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}}
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/"+Realm+"/protocol/openid-connect/certs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &Keycloak{Server: srv, key: key, rogue: rogue}
}

// Verifier builds a real auth.KeycloakVerifier against this fake.
func (k *Keycloak) Verifier(t testing.TB) *auth.KeycloakVerifier {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	v, err := auth.NewKeycloakVerifier(ctx, k.Config())
	if err != nil {
		t.Fatalf("NewKeycloakVerifier: %v", err)
	}
	return v
}

// Claims returns valid default claims for subject sub (expires in 5 minutes).
func (k *Keycloak) Claims(sub string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":                k.Issuer(),
		"sub":                sub,
		"azp":                ClientID,
		"exp":                now.Add(5 * time.Minute).Unix(),
		"iat":                now.Unix(),
		"preferred_username": "demo",
		"email":              "demo@example.com",
		"name":               "Demo Channel",
	}
}

// Sign signs claims with the realm key (RS256, kid=KeyID).
func (k *Keycloak) Sign(t testing.TB, claims jwt.MapClaims) string {
	t.Helper()
	return sign(t, k.key, jwt.SigningMethodRS256, claims)
}

// SignRogue signs claims with a key that is NOT in the JWKS but claims the same kid (bad signature).
func (k *Keycloak) SignRogue(t testing.TB, claims jwt.MapClaims) string {
	t.Helper()
	return sign(t, k.rogue, jwt.SigningMethodRS256, claims)
}

// SignHS256 signs claims with HMAC (wrong algorithm for the verifier).
func (k *Keycloak) SignHS256(t testing.TB, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tok.Header["kid"] = KeyID
	s, err := tok.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

// Token is a valid token for sub.
func (k *Keycloak) Token(t testing.TB, sub string) string {
	t.Helper()
	return k.Sign(t, k.Claims(sub))
}

func sign(t testing.TB, key *rsa.PrivateKey, m jwt.SigningMethod, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(m, claims)
	tok.Header["kid"] = KeyID
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}
