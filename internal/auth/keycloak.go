package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/ak-ansari/mytube/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

var ErrInvalidToken = errors.New("invalid or expired token")

// Claims are the parts of a Keycloak access token we care about.
type Claims struct {
	jwt.RegisteredClaims
	AuthorizedParty   string `json:"azp"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Name              string `json:"name"`
}

// KeycloakVerifier validates access tokens issued by a Keycloak realm using the realm's JWKS.
type KeycloakVerifier struct {
	keyfunc  keyfunc.Keyfunc
	issuer   string
	clientId string
}

func NewKeycloakVerifier(ctx context.Context, conf config.Keycloak) (*KeycloakVerifier, error) {
	if conf.Url == "" || conf.Realm == "" {
		return nil, fmt.Errorf("KEYCLOAK.URL and KEYCLOAK.REALM must be configured")
	}
	issuer := strings.TrimRight(conf.Url, "/") + "/realms/" + conf.Realm
	// the JWKS is fetched lazily, so the API can start before Keycloak is up
	kf, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{issuer + "/protocol/openid-connect/certs"}, keyfunc.Override{
		RefreshUnknownKID: rate.NewLimiter(rate.Every(10*time.Second), 1),
	})
	if err != nil {
		return nil, err
	}
	return &KeycloakVerifier{keyfunc: kf, issuer: issuer, clientId: conf.ClientId}, nil
}

func (v *KeycloakVerifier) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, v.keyfunc.Keyfunc,
		jwt.WithIssuer(v.issuer),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	// Keycloak public-client tokens carry the client in "azp" rather than "aud"
	if v.clientId != "" && claims.AuthorizedParty != v.clientId {
		return nil, fmt.Errorf("%w: unexpected azp %q", ErrInvalidToken, claims.AuthorizedParty)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: missing sub", ErrInvalidToken)
	}
	return claims, nil
}
