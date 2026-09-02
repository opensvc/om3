package daemonauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIssuer   = "https://auth.example.com/application/o/cluster/"
	testClientID = "om3"
)

// openIDFixture is a provider serving one key, and the strategy of a
// cluster configured to trust it.
type openIDFixture struct {
	key      *rsa.PrivateKey
	strategy *openIDStrategy
}

func newOpenIDFixture(t *testing.T) *openIDFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	var keys atomic.Value
	keys.Store([]jsonWebKey{rsaJWK(t, "k1", key)})
	srv, _ := jwksServer(t, &keys)
	return &openIDFixture{
		key: key,
		strategy: &openIDStrategy{
			jwks:     newJWKS(srv.URL),
			issuer:   testIssuer,
			clientID: testClientID,
		},
	}
}

// idToken signs claims as the provider would, with the kid header the key
// set publishes.
func (f *openIDFixture) idToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tk := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tk.Header["kid"] = "k1"
	s, err := tk.SignedString(f.key)
	require.NoError(t, err)
	return s
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss":                testIssuer,
		"aud":                testClientID,
		"sub":                "9f0e0b1e",
		"preferred_username": "alice",
		"email":              "alice@example.com",
		"entitlements":       []string{"root", "admin:test"},
		"exp":                time.Now().Add(time.Minute).Unix(),
		"iat":                time.Now().Unix(),
	}
}

func TestOpenIDAcceptsAnIDToken(t *testing.T) {
	authCache = newCache()
	f := newOpenIDFixture(t)
	info, err := f.strategy.Authenticate(context.Background(), bearerRequest(t, f.idToken(t, validClaims())))
	require.NoError(t, err)
	assert.Equal(t, "alice", info.Username)
	assert.Equal(t, testIssuer, info.Issuer)
	assert.Equal(t, StrategyJWTOpenID, info.Strategy)
	assert.Equal(t, []string{"root", "admin:test"}, info.Grants)
}

func TestOpenIDNamesTheUser(t *testing.T) {
	// The provider is free to leave out the friendlier claims. The
	// subject is the one it must set, so it is the last resort.
	for name, tc := range map[string]struct {
		drop []string
		want string
	}{
		"by preferred username": {nil, "alice"},
		"by email":              {[]string{"preferred_username"}, "alice@example.com"},
		"by subject":            {[]string{"preferred_username", "email"}, "9f0e0b1e"},
	} {
		t.Run(name, func(t *testing.T) {
			authCache = newCache()
			f := newOpenIDFixture(t)
			claims := validClaims()
			for _, k := range tc.drop {
				delete(claims, k)
			}
			info, err := f.strategy.Authenticate(context.Background(), bearerRequest(t, f.idToken(t, claims)))
			require.NoError(t, err)
			assert.Equal(t, tc.want, info.Username)
		})
	}
}

func TestOpenIDRefusals(t *testing.T) {
	for name, edit := range map[string]func(jwt.MapClaims){
		"a token for another audience": func(c jwt.MapClaims) { c["aud"] = "another-client" },
		"a token from another issuer":  func(c jwt.MapClaims) { c["iss"] = "https://evil.example.com/" },
		"an expired token":             func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() },
		"a token that never expires":   func(c jwt.MapClaims) { delete(c, "exp") },
	} {
		t.Run(name, func(t *testing.T) {
			authCache = newCache()
			f := newOpenIDFixture(t)
			claims := validClaims()
			edit(claims)
			_, err := f.strategy.Authenticate(context.Background(), bearerRequest(t, f.idToken(t, claims)))
			assert.Error(t, err)
		})
	}
}

func TestOpenIDRefusesATokenSignedWithAKeyItDoesNotKnow(t *testing.T) {
	authCache = newCache()
	f := newOpenIDFixture(t)
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tk := jwt.NewWithClaims(jwt.SigningMethodRS256, validClaims())
	tk.Header["kid"] = "k1"
	s, err := tk.SignedString(other)
	require.NoError(t, err)

	_, err = f.strategy.Authenticate(context.Background(), bearerRequest(t, s))
	assert.Error(t, err)
}

func TestOpenIDRefusesATokenWithNoKeyID(t *testing.T) {
	authCache = newCache()
	f := newOpenIDFixture(t)
	s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, validClaims()).SignedString(f.key)
	require.NoError(t, err)
	_, err = f.strategy.Authenticate(context.Background(), bearerRequest(t, s))
	assert.Error(t, err)
}

func TestOpenIDRefusesATokenSignedWithTheWrongAlgorithm(t *testing.T) {
	// The key set publishes k1 for RS256. A token asking for anything
	// else must not be verified with it, and one asking for an hmac must
	// not be verified with a key anyone can fetch.
	authCache = newCache()
	f := newOpenIDFixture(t)
	tk := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims())
	tk.Header["kid"] = "k1"
	s, err := tk.SignedString([]byte("secret"))
	require.NoError(t, err)

	_, err = f.strategy.Authenticate(context.Background(), bearerRequest(t, s))
	assert.Error(t, err)
}

func TestOpenIDNeedsABearerToken(t *testing.T) {
	authCache = newCache()
	f := newOpenIDFixture(t)
	_, err := f.strategy.Authenticate(context.Background(), bearerRequest(t, ""))
	assert.Error(t, err)
}

func TestDecodeOpenIDConfiguration(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/application/o/cluster/.well-known/openid-configuration", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":   testIssuer,
			"jwks_uri": "https://auth.example.com/jwks/",
		})
	}))
	defer srv.Close()

	config, err := fetchOpenIDConfiguration(context.Background(), time.Second, srv.URL+"/application/o/cluster/")
	require.NoError(t, err)
	assert.Equal(t, testIssuer, config.Issuer)
	assert.Equal(t, "https://auth.example.com/jwks/", config.JwsksUri)
}

func TestOpenIDDiscoverURL(t *testing.T) {
	got, err := OpenIDDiscoverURL("https://auth.example.com/application/o/cluster/")
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com/application/o/cluster/.well-known/openid-configuration", got)
}
