package daemonauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rsaJWK(t *testing.T, kid string, key *rsa.PrivateKey) jsonWebKey {
	t.Helper()
	return jsonWebKey{
		Kid: kid,
		Kty: "RSA",
		Alg: "RS256",
		Use: "sig",
		N:   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}
}

// jwksServer serves the key set it is pointed at, and counts the fetches.
func jwksServer(t *testing.T, keys *atomic.Value) (*httptest.Server, *int64) {
	t.Helper()
	var fetches int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&fetches, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys.Load()})
	}))
	t.Cleanup(srv.Close)
	return srv, &fetches
}

func TestJWKSParsesAnRSAKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pub, err := rsaJWK(t, "k1", key).publicKey()
	require.NoError(t, err)
	assert.Equal(t, &key.PublicKey, pub)
}

func TestJWKSParsesAnECKey(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	jwk := jsonWebKey{
		Kid: "k1",
		Kty: "EC",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(key.PublicKey.X.Bytes()),
		Y:   base64.RawURLEncoding.EncodeToString(key.PublicKey.Y.Bytes()),
	}
	pub, err := jwk.publicKey()
	require.NoError(t, err)
	assert.Equal(t, &key.PublicKey, pub)
}

func TestJWKSRefusesAKeyItCannotUse(t *testing.T) {
	for name, jwk := range map[string]jsonWebKey{
		"an unknown key type": {Kty: "oct", N: "AQAB", E: "AQAB"},
		"an unknown curve":    {Kty: "EC", Crv: "P-192", X: "AQAB", Y: "AQAB"},
		"an empty modulus":    {Kty: "RSA", E: "AQAB"},
		"a point off the curve": {
			Kty: "EC",
			Crv: "P-256",
			X:   base64.RawURLEncoding.EncodeToString(big.NewInt(1).Bytes()),
			Y:   base64.RawURLEncoding.EncodeToString(big.NewInt(1).Bytes()),
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := jwk.publicKey()
			assert.Error(t, err)
		})
	}
}

func TestJWKSFetchesOnceUntilItGoesStale(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	var keys atomic.Value
	keys.Store([]jsonWebKey{rsaJWK(t, "k1", key)})
	srv, fetches := jwksServer(t, &keys)

	j := newJWKS(srv.URL)
	for i := 0; i < 3; i++ {
		_, err := j.get(context.Background(), "k1")
		require.NoError(t, err)
	}
	assert.EqualValues(t, 1, *fetches, "a known key id must be answered from the cached set")
}

func TestJWKSRefetchesOnAnUnknownKeyID(t *testing.T) {
	// This is what a key rotation looks like from here: a token signed
	// with a key that was published after the last fetch. Waiting for the
	// set to expire would refuse every request until then.
	first, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	second, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	var keys atomic.Value
	keys.Store([]jsonWebKey{rsaJWK(t, "k1", first)})
	srv, fetches := jwksServer(t, &keys)

	j := newJWKS(srv.URL)
	_, err = j.get(context.Background(), "k1")
	require.NoError(t, err)

	keys.Store([]jsonWebKey{rsaJWK(t, "k2", second)})
	got, err := j.get(context.Background(), "k2")
	require.NoError(t, err)
	assert.Equal(t, &second.PublicKey, got.key)
	assert.EqualValues(t, 2, *fetches)

	_, err = j.get(context.Background(), "k3")
	assert.Error(t, err, "a key id the provider does not publish is an error, not a third fetch")
	assert.EqualValues(t, 3, *fetches)
}

func TestJWKSRefusesAKeySetWithNothingUsable(t *testing.T) {
	var keys atomic.Value
	keys.Store([]jsonWebKey{{Kid: "k1", Kty: "oct"}})
	srv, _ := jwksServer(t, &keys)
	_, err := newJWKS(srv.URL).get(context.Background(), "k1")
	assert.Error(t, err)
}

func TestJWKSReportsAFetchFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, err := newJWKS(srv.URL).get(context.Background(), "k1")
	assert.Error(t, err)
}

func TestMaxAge(t *testing.T) {
	for name, tc := range map[string]struct {
		header string
		want   time.Duration
	}{
		"no header":         {"", jwksDefaultInterval},
		"a max age":         {"max-age=60", time.Minute},
		"among directives":  {"public, max-age=60, must-revalidate", time.Minute},
		"any case":          {"Max-Age=60", time.Minute},
		"spaced":            {"max-age = 60", time.Minute},
		"not a number":      {"max-age=soon", jwksDefaultInterval},
		"zero":              {"max-age=0", jwksDefaultInterval},
		"another directive": {"no-store", jwksDefaultInterval},
	} {
		t.Run(name, func(t *testing.T) {
			h := http.Header{}
			if tc.header != "" {
				h.Set("Cache-Control", tc.header)
			}
			assert.Equal(t, tc.want, maxAge(h, jwksDefaultInterval))
		})
	}
}

func TestJWKSHonoursMaxAge(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwk := rsaJWK(t, "k1", key)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", 3600))
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []jsonWebKey{jwk}})
	}))
	defer srv.Close()

	j := newJWKS(srv.URL)
	_, err = j.get(context.Background(), "k1")
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(time.Hour), j.expireAt, time.Minute)
}
