package daemonauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type (
	// openIDStrategy authenticates the id tokens of the openid provider
	// the cluster is configured with.
	openIDStrategy struct {
		jwks     *jwks
		issuer   string
		clientID string
	}

	// openIDClaims is what this daemon reads from an id token. The
	// entitlements claim is where the provider is expected to put the
	// opensvc grants: the rest of the claims say who the user is, this
	// one says what they may do.
	openIDClaims struct {
		PreferredUsername string   `json:"preferred_username"`
		Email             string   `json:"email"`
		Grant             []string `json:"entitlements"`
		jwt.RegisteredClaims
	}

	OpenIDConfiguration struct {
		Issuer          string   `json:"issuer"`
		ScopesSupported []string `json:"scopes_supported"`
		JwsksUri        string   `json:"jwks_uri"`
	}

	OpenIDSettings interface {
		OpenIDIssuer() string
		OpenIDClientID() string
	}
)

// openIDSigningMethods are the algorithms an id token may be signed with.
//
// The list is asymmetric algorithms only: the provider signs with a
// private key we never see, and we verify with the public key its jwks
// publishes. A token naming HMAC would have us verify it with a key
// anyone can fetch, and one naming none would have us verify nothing.
var openIDSigningMethods = []string{
	"RS256", "RS384", "RS512",
	"PS256", "PS384", "PS512",
	"ES256", "ES384", "ES512",
}

func (s *openIDStrategy) Authenticate(ctx context.Context, r *http.Request) (*Info, error) {
	tkString, err := bearerToken(r)
	if err != nil {
		return nil, fmt.Errorf("strategies/openid: %w", err)
	}
	key := cacheKey(StrategyJWTOpenID, tkString)
	if info, ok := authCache.get(key); ok {
		return info, nil
	}
	claims := &openIDClaims{}
	if _, err := jwt.ParseWithClaims(tkString, claims, s.keyFunc(ctx),
		jwt.WithValidMethods(openIDSigningMethods),
		jwt.WithIssuer(s.issuer),
		jwt.WithAudience(s.clientID),
		jwt.WithExpirationRequired(),
	); err != nil {
		return nil, fmt.Errorf("strategies/openid: %w", err)
	}
	info := &Info{
		Username: claims.username(),
		Strategy: StrategyJWTOpenID,
		Issuer:   claims.Issuer,
		Grants:   claims.Grant,
	}
	authCache.set(key, info, time.Until(claims.ExpiresAt.Time))
	return info, nil
}

// keyFunc returns the public key a token is to be verified with, the one
// its kid header names in the provider's key set.
//
// The key set says which algorithm each key is for, and a token asking
// for a different one is refused rather than verified: a key published
// for RS256 is not to be used to check an ES256 signature, whatever the
// token says.
func (s *openIDStrategy) keyFunc(ctx context.Context) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok || kid == "" {
			return nil, fmt.Errorf("token has no kid header")
		}
		key, err := s.jwks.get(ctx, kid)
		if err != nil {
			return nil, err
		}
		if key.alg != "" && key.alg != token.Method.Alg() {
			return nil, fmt.Errorf("token alg %s does not match the alg %s of key %s",
				token.Method.Alg(), key.alg, kid)
		}
		return key.key, nil
	}
}

// username is who the token is about: what the provider calls them, then
// their email, then the subject, which is the only one it must have.
func (c *openIDClaims) username() string {
	for _, candidate := range []string{c.PreferredUsername, c.Email, c.Subject} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

// initJWTOpenID initializes the JWT OpenID authentication strategy using the provided input interface.
// Returns the strategy name, the initialized authentication strategy, or an error if initialization fails.
func initJWTOpenID(ctx context.Context, i interface{}) (string, Strategy, error) {
	settings, ok := i.(OpenIDSettings)
	if !ok {
		return StrategyJWTOpenID, nil, nil
	}
	providerURL := settings.OpenIDIssuer()
	if providerURL == "" {
		return StrategyJWTOpenID, nil, nil
	}
	clientID := settings.OpenIDClientID()
	if clientID == "" {
		return StrategyJWTOpenID, nil, fmt.Errorf("undefined client id for provider %s", providerURL)
	}
	config, err := fetchOpenIDConfiguration(ctx, discoverOpenIDTimeout, providerURL)
	if err != nil {
		return StrategyJWTOpenID, nil, err
	} else if config == nil {
		return StrategyJWTOpenID, nil, nil
	}
	if config.JwsksUri == "" {
		return StrategyJWTOpenID, nil, fmt.Errorf("jwks uri is empty")
	}
	return StrategyJWTOpenID, &openIDStrategy{
		jwks:     newJWKS(config.JwsksUri),
		issuer:   config.Issuer,
		clientID: clientID,
	}, nil
}

// fetchOpenIDConfiguration retrieves OpenID issuer configuration from the given configuration URL.
// It fetches the OpenID discovery document, parses the JSON response, and returns the OpenIDConfiguration struct.
// Returns an error if the URL is invalid, the request fails, or if the response cannot be processed correctly.
func fetchOpenIDConfiguration(ctx context.Context, timeout time.Duration, configURL string) (*OpenIDConfiguration, error) {
	var (
		req *http.Request
	)
	if configURL == "" {
		return nil, nil
	}
	discoverURL, err := OpenIDDiscoverURL(configURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse discover url %s: %v", configURL, err)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, discoverURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for OpenID config: %w", err)
	}

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenID configuration from %s: %w", configURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %d from %s", resp.StatusCode, req.URL)
	}

	return decodeOpenIDConfiguration(resp.Body, req.URL.String())
}

func decodeOpenIDConfiguration(body io.Reader, source string) (*OpenIDConfiguration, error) {
	var openID OpenIDConfiguration
	if err := json.NewDecoder(body).Decode(&openID); err != nil {
		return nil, fmt.Errorf("failed to decode OpenID configuration from %s: %w", source, err)
	}
	return &openID, nil
}

func OpenIDDiscoverURL(base string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	u.Path = path.Join(u.Path, "/.well-known/openid-configuration")
	return u.String(), nil
}
