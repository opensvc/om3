package daemonauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

// signedToken returns a token signed with key, from the claims as they
// would be written by CreateToken.
func signedToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	require.NoError(t, err)
	return s
}

func bearerRequest(t *testing.T, tk string) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, "https://localhost/api/whoami", nil)
	require.NoError(t, err)
	if tk != "" {
		r.Header.Set("Authorization", "Bearer "+tk)
	}
	return r
}

func TestBearerToken(t *testing.T) {
	for name, tc := range map[string]struct {
		header string
		want   string
	}{
		"a bearer token":            {"Bearer abc", "abc"},
		"padded":                    {"  Bearer abc  ", "abc"},
		"no header":                 {"", ""},
		"another scheme":            {"Basic abc", ""},
		"the wrong case":            {"bearer abc", ""},
		"a scheme and nothing else": {"Bearer", ""},
		"an empty token":            {"Bearer ", ""},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodGet, "https://localhost/", nil)
			require.NoError(t, err)
			if tc.header != "" {
				r.Header.Set("Authorization", tc.header)
			}
			tk, err := bearerToken(r)
			if tc.want == "" {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, tk)
			}
		})
	}
}

func TestJWTStrategyAcceptsATokenThisClusterIssued(t *testing.T) {
	authCache = newCache()
	key := testKey(t)
	s := &jwtStrategy{verifyKey: &key.PublicKey}
	tk := signedToken(t, key, jwt.MapClaims{
		"sub":      "alice",
		"iss":      "node1",
		"grant":    []string{"root"},
		TkUseClaim: TkUseAccess,
		"exp":      time.Now().Add(time.Minute).Unix(),
	})

	info, err := s.Authenticate(context.Background(), bearerRequest(t, tk))
	require.NoError(t, err)
	assert.Equal(t, "alice", info.Username)
	assert.Equal(t, "node1", info.Issuer)
	assert.Equal(t, StrategyJWT, info.Strategy)
	assert.Equal(t, TkUseAccess, info.TokenUse)
	assert.Equal(t, []string{"root"}, info.Grants)

	// The second request is answered from the cache, with the same
	// decision.
	again, err := s.Authenticate(context.Background(), bearerRequest(t, tk))
	require.NoError(t, err)
	assert.Same(t, info, again)
}

func TestJWTStrategyRefusals(t *testing.T) {
	key := testKey(t)
	other := testKey(t)

	hs256 := func(t *testing.T) string {
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "alice",
			"exp": time.Now().Add(time.Minute).Unix(),
		}).SignedString([]byte("secret"))
		require.NoError(t, err)
		return s
	}

	for name, tc := range map[string]struct {
		token func(t *testing.T) string
	}{
		"a token signed by someone else": {func(t *testing.T) string {
			return signedToken(t, other, jwt.MapClaims{"sub": "alice", "exp": time.Now().Add(time.Minute).Unix()})
		}},
		"an expired token": {func(t *testing.T) string {
			return signedToken(t, key, jwt.MapClaims{"sub": "alice", "exp": time.Now().Add(-time.Minute).Unix()})
		}},
		"a token that never expires": {func(t *testing.T) string {
			return signedToken(t, key, jwt.MapClaims{"sub": "alice"})
		}},
		// The signing method is checked before the key is asked for: an
		// hmac token must not be verified with a public key, whether or
		// not that happens to fail on its own.
		"a token signed with an hmac": {hs256},
		"not a token at all": {func(t *testing.T) string {
			return "hello"
		}},
	} {
		t.Run(name, func(t *testing.T) {
			authCache = newCache()
			s := &jwtStrategy{verifyKey: &key.PublicKey}
			_, err := s.Authenticate(context.Background(), bearerRequest(t, tc.token(t)))
			assert.Error(t, err)
		})
	}
}

func TestJWTStrategyNeedsABearerToken(t *testing.T) {
	authCache = newCache()
	key := testKey(t)
	s := &jwtStrategy{verifyKey: &key.PublicKey}
	_, err := s.Authenticate(context.Background(), bearerRequest(t, ""))
	assert.Error(t, err)
}

func TestCreateTokenWithoutASignKey(t *testing.T) {
	// The daemon that failed to load its sign key must say so here,
	// rather than hand out an empty token for a peer to reject.
	jwtSignKey.Store(nil)
	_, _, err := (&JWTCreator{}).CreateToken(time.Minute, map[string]any{"sub": "alice"})
	assert.Error(t, err)
}

func TestCreateNodeTokenIsAcceptedByTheJWTStrategy(t *testing.T) {
	authCache = newCache()
	key := testKey(t)
	jwtSignKey.Store(key)
	defer jwtSignKey.Store(nil)

	tk, err := CreateNodeToken()
	require.NoError(t, err)

	s := &jwtStrategy{verifyKey: &key.PublicKey}
	info, err := s.Authenticate(context.Background(), bearerRequest(t, tk))
	require.NoError(t, err)
	assert.Equal(t, []string{"root"}, info.Grants)
	assert.NotEmpty(t, info.Username)
	assert.Equal(t, info.Username, info.Issuer, "a node token is issued by the node it is about")
}
