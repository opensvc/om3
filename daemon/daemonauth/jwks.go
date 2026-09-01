package daemonauth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// jwks holds the public keys an openid provider signs its tokens with,
// fetched from the jwks_uri its discovery document names.
//
// The provider rotates them on its own schedule and announces nothing, so
// the set is refetched when it goes stale: after what the Cache-Control
// header of the last fetch asked for, or jwksDefaultInterval when it
// asked for nothing.
type jwks struct {
	uri    string
	client *http.Client

	mu       sync.Mutex
	keys     map[string]jwksKey
	expireAt time.Time
}

// jwksKey is one signing key, with the algorithm the provider says it is
// to be used with. The algorithm is optional in a JWK, and empty means
// the provider did not say.
type jwksKey struct {
	key crypto.PublicKey
	alg string
}

const (
	// jwksDefaultInterval is how long a key set is kept when the provider
	// sends no Cache-Control max-age.
	jwksDefaultInterval = 5 * time.Minute

	// jwksMaxSize bounds what is read from the jwks_uri. A key set is a
	// few kilobytes; this is only here so that a wrong url pointing at
	// something large cannot be read into the daemon's memory.
	jwksMaxSize = 1 << 20

	// jwksTimeout bounds one fetch. It happens on the path of a request
	// being authenticated, so it cannot wait long.
	jwksTimeout = 5 * time.Second
)

func newJWKS(uri string) *jwks {
	return &jwks{
		uri:    uri,
		client: &http.Client{Timeout: jwksTimeout},
		keys:   make(map[string]jwksKey),
	}
}

// get returns the key of the given id, fetching the set when it has gone
// stale.
//
// An unknown key id refetches once, whatever the expiry says: it is what
// a rotation looks like from here, and refusing a token until the cached
// set expires would be an outage of up to jwksDefaultInterval. The fetch
// is guarded by the same lock as the rest, so a burst of tokens signed
// with an unknown key makes one request, not one per request.
func (j *jwks) get(ctx context.Context, kid string) (jwksKey, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if time.Now().After(j.expireAt) {
		if err := j.load(ctx); err != nil {
			return jwksKey{}, err
		}
	}
	if key, ok := j.keys[kid]; ok {
		return key, nil
	}
	if err := j.load(ctx); err != nil {
		return jwksKey{}, err
	}
	key, ok := j.keys[kid]
	if !ok {
		return jwksKey{}, fmt.Errorf("no key %s in %s", kid, j.uri)
	}
	return key, nil
}

// load replaces the key set. The caller holds the lock.
func (j *jwks) load(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, jwksTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.uri, nil)
	if err != nil {
		return fmt.Errorf("jwks request: %w", err)
	}
	resp, err := j.client.Do(req)
	if err != nil {
		return fmt.Errorf("jwks fetch %s: %w", j.uri, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks fetch %s: unexpected status %d", j.uri, resp.StatusCode)
	}
	var set struct {
		Keys []jsonWebKey `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, jwksMaxSize)).Decode(&set); err != nil {
		return fmt.Errorf("jwks decode %s: %w", j.uri, err)
	}
	keys := make(map[string]jwksKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		key, err := k.publicKey()
		if err != nil {
			// One unusable key is not a reason to refuse the others.
			continue
		}
		keys[k.Kid] = jwksKey{key: key, alg: k.Alg}
	}
	if len(keys) == 0 {
		return fmt.Errorf("jwks %s has no usable signing key", j.uri)
	}
	j.keys = keys
	j.expireAt = time.Now().Add(maxAge(resp.Header, jwksDefaultInterval))
	return nil
}

// jsonWebKey is the subset of RFC 7517 needed to verify a signature.
type jsonWebKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`

	// RSA
	N string `json:"n"`
	E string `json:"e"`

	// EC
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (k jsonWebKey) publicKey() (crypto.PublicKey, error) {
	switch k.Kty {
	case "RSA":
		n, err := k.bigInt(k.N)
		if err != nil {
			return nil, fmt.Errorf("rsa modulus: %w", err)
		}
		e, err := k.bigInt(k.E)
		if err != nil {
			return nil, fmt.Errorf("rsa exponent: %w", err)
		}
		if !e.IsInt64() || e.Int64() <= 0 || e.Int64() > 1<<31-1 {
			return nil, fmt.Errorf("rsa exponent out of range")
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	case "EC":
		var curve elliptic.Curve
		switch k.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, fmt.Errorf("unsupported curve %s", k.Crv)
		}
		x, err := k.bigInt(k.X)
		if err != nil {
			return nil, fmt.Errorf("ec x: %w", err)
		}
		y, err := k.bigInt(k.Y)
		if err != nil {
			return nil, fmt.Errorf("ec y: %w", err)
		}
		if !curve.IsOnCurve(x, y) {
			return nil, fmt.Errorf("ec point is not on curve %s", k.Crv)
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	default:
		return nil, fmt.Errorf("unsupported key type %s", k.Kty)
	}
}

// bigInt decodes a JWK base64url big endian unsigned integer.
func (k jsonWebKey) bigInt(s string) (*big.Int, error) {
	if s == "" {
		return nil, fmt.Errorf("empty value")
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(b), nil
}

// maxAge returns the Cache-Control max-age of a response, or def when it
// has none, or a value that is not a number of seconds.
func maxAge(h http.Header, def time.Duration) time.Duration {
	for _, directive := range strings.Split(h.Get("Cache-Control"), ",") {
		name, value, found := strings.Cut(directive, "=")
		if !found || !strings.EqualFold(strings.TrimSpace(name), "max-age") {
			continue
		}
		seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil || seconds <= 0 {
			continue
		}
		return time.Duration(seconds) * time.Second
	}
	return def
}
