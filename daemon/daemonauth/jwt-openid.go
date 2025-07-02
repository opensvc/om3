package daemonauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shaj13/go-guardian/v2/auth"
	"github.com/shaj13/go-guardian/v2/auth/claims"
	"github.com/shaj13/go-guardian/v2/auth/strategies/oauth2"
	"github.com/shaj13/go-guardian/v2/auth/strategies/oauth2/jwt"
	"github.com/shaj13/go-guardian/v2/auth/strategies/token"
	"github.com/shaj13/libcache"
)

type (
	k struct {
		baseStrategy auth.Strategy
	}

	IDTokenGrant struct {
		*jwt.IDToken
		Grant []string `json:"grant"`
	}

	OpenIDAuthority struct {
		ScopesSupported []string `json:"scopes_supported"`
		JwsksUri        string   `json:"jwks_uri"`
	}
)

func (i IDTokenGrant) New() oauth2.ClaimsResolver {
	return &IDTokenGrant{
		IDToken: &jwt.IDToken{
			Info:     auth.NewUserInfo("", "", []string{}, make(auth.Extensions)),
			Standard: new(claims.Standard),
		},
		Grant: make([]string, 0),
	}
}

func (i IDTokenGrant) Verify(options claims.VerifyOptions) error {
	return i.IDToken.Verify(options)
}

func (i IDTokenGrant) Resolve() auth.Info {
	return i
}

// Authenticate verifies user credentials using the base strategy and maps
// the ID token to user information with extensions with grant.
func (s *k) Authenticate(ctx context.Context, r *http.Request) (auth.Info, error) {
	info, err := s.baseStrategy.Authenticate(ctx, r)
	if err != nil {
		return nil, err
	}
	tk := info.(IDTokenGrant)
	extensions := authenticatedExtensions(StrategyJWTOpenID, tk.Issuer, tk.Grant...)
	return auth.NewUserInfo(info.GetUserName(), info.GetUserName(), nil, *extensions), nil
}

// initJWTOpenID initializes the JWT OpenID authentication strategy using the provided input interface.
// Returns the strategy name, the initialized authentication strategy, or an error if initialization fails.
func initJWTOpenID(i interface{}) (string, auth.Strategy, error) {
	o, ok := i.(*OpenIDAuthority)
	if !ok || o == nil {
		return StrategyJWTOpenID, nil, nil
	} else if o.JwsksUri == "" {
		return StrategyJWTOpenID, nil, fmt.Errorf("jwks uri is empty")
	}
	cache := libcache.FIFO.New(100)
	cache.SetTTL(time.Second)
	verifyOptionTime := func() time.Time {
		return time.Now().Add(time.Second * 5)
	}
	opt := []auth.Option{
		token.SetParser(token.AuthorizationParser("Bearer")),
		jwt.SetVerifyOptions(claims.VerifyOptions{Time: verifyOptionTime}),
		jwt.SetClaimResolver(&IDTokenGrant{}),
	}
	strategy := jwt.New(o.JwsksUri, cache, opt...)
	return StrategyJWTOpenID, &k{baseStrategy: strategy}, nil
}

const (
	openIDWellKnownPath = "/.well-known/openid-configuration"
)

// FetchOpenIDAuthority retrieves OpenID authority configuration from the given configuration URL.
// It fetches the OpenID discovery document, parses the JSON response, and returns the OpenIDAuthority struct.
// Returns an error if the URL is invalid, the request fails, or if the response cannot be processed correctly.
func FetchOpenIDAuthority(ctx context.Context, timeout time.Duration, configURL string) (*OpenIDAuthority, error) {
	if configURL == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL+openIDWellKnownPath, nil)
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

	return decodeOpenIDAuthority(resp.Body, req.URL.String())
}

func decodeOpenIDAuthority(body io.Reader, source string) (*OpenIDAuthority, error) {
	var openID OpenIDAuthority
	if err := json.NewDecoder(body).Decode(&openID); err != nil {
		return nil, fmt.Errorf("failed to decode OpenID configuration from %s: %w", source, err)
	}
	return &openID, nil
}
