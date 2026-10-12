package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ak-ansari/mytube/internal/auth"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/ak-ansari/mytube/internal/testutil/authtest"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestNewKeycloakVerifierRequiresConfig(t *testing.T) {
	for _, c := range []config.Keycloak{{}, {Url: "http://kc"}, {Realm: "mytube"}} {
		if _, err := auth.NewKeycloakVerifier(context.Background(), c); err == nil {
			t.Errorf("config %+v: expected error", c)
		}
	}
}

func TestVerify(t *testing.T) {
	kc := authtest.New(t)
	v := kc.Verifier(t)
	sub := uuid.NewString()

	mod := func(f func(c jwt.MapClaims)) jwt.MapClaims {
		c := kc.Claims(sub)
		f(c)
		return c
	}
	tests := []struct {
		name  string
		token string
		ok    bool
	}{
		{"valid", kc.Token(t, sub), true},
		{"within leeway", kc.Sign(t, mod(func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-10 * time.Second).Unix() })), true},
		{"expired", kc.Sign(t, mod(func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() })), false},
		{"no exp", kc.Sign(t, mod(func(c jwt.MapClaims) { delete(c, "exp") })), false},
		{"not yet valid", kc.Sign(t, mod(func(c jwt.MapClaims) { c["nbf"] = time.Now().Add(time.Hour).Unix() })), false},
		{"wrong issuer", kc.Sign(t, mod(func(c jwt.MapClaims) { c["iss"] = "http://evil/realms/mytube" })), false},
		{"other realm", kc.Sign(t, mod(func(c jwt.MapClaims) { c["iss"] = kc.Server.URL + "/realms/master" })), false},
		{"wrong azp", kc.Sign(t, mod(func(c jwt.MapClaims) { c["azp"] = "admin-cli" })), false},
		{"missing azp", kc.Sign(t, mod(func(c jwt.MapClaims) { delete(c, "azp") })), false},
		{"missing sub", kc.Sign(t, mod(func(c jwt.MapClaims) { delete(c, "sub") })), false},
		{"bad signature", kc.SignRogue(t, kc.Claims(sub)), false},
		{"HS256", kc.SignHS256(t, kc.Claims(sub)), false},
		{"garbage", "not.a.jwt", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := v.Verify(tt.token)
			if tt.ok {
				if err != nil {
					t.Fatalf("Verify: %v", err)
				}
				if c.Subject != sub || c.AuthorizedParty != authtest.ClientID || c.PreferredUsername != "demo" ||
					c.Email != "demo@example.com" || c.Name != "Demo Channel" {
					t.Errorf("claims = %+v", c)
				}
				return
			}
			if !errors.Is(err, auth.ErrInvalidToken) || c != nil {
				t.Fatalf("got (%v, %v), want ErrInvalidToken", c, err)
			}
		})
	}
}

func TestVerifyWithoutClientIDSkipsAzpCheck(t *testing.T) {
	kc := authtest.New(t)
	conf := kc.Config()
	conf.ClientId = ""
	v, err := auth.NewKeycloakVerifier(t.Context(), conf)
	if err != nil {
		t.Fatal(err)
	}
	c := kc.Claims(uuid.NewString())
	c["azp"] = "anything"
	if _, err := v.Verify(kc.Sign(t, c)); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifierToleratesTrailingSlashInURL(t *testing.T) {
	kc := authtest.New(t)
	conf := kc.Config()
	conf.Url += "/"
	v, err := auth.NewKeycloakVerifier(t.Context(), conf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(kc.Token(t, uuid.NewString())); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}
