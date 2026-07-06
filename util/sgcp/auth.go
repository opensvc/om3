package sgcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/opensvc/om3/v3/util/ageingcache"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// AuthInfo represents authentication details required for API access.
	AuthInfo struct {
		AccountID    string
		ClientID     string
		ClientSecret string

		// Signature identifies auth info
		Signature string
	}

	// TokenFactory is responsible for generating and caching authentication tokens for API access.
	TokenFactory struct {
		authInfo   *AuthInfo
		authConfig *AuthConfig
		log        *plog.Logger
		client     *http.Client
	}

	// Token represents an authentication token used for API access.
	Token struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope,omitempty"`
		TokenType   string `json:"token_type,omitempty"`
		ExpiresIn   int    `json:"expires_in,omitempty"`
	}
)

var (
	singleFlightGrp singleflight.Group
)

// NewTokenFactory initializes a new instance of TokenFactory with the provided logger, authentication config, and auth info.
func NewTokenFactory(log *plog.Logger, client *http.Client, authCfg *AuthConfig, authInfo *AuthInfo) *TokenFactory {
	return &TokenFactory{
		client:     client,
		log:        log,
		authConfig: authCfg,
		authInfo:   authInfo,
	}
}

// Get retrieves an authentication token for the given context and scopes, utilizing caching for performance optimization.
// Returns the token as a string or an error if retrieval fails.
func (t *TokenFactory) Get(ctx context.Context, scope ...string) (string, error) {
	cacheMaxAge := time.Duration(t.authConfig.TTLSeconds) * time.Second
	sig := t.signature(scope...)
	o := ageingcache.NewOutputter(func() ([]byte, error) {
		token, err := t.newToken(ctx, scope...)
		if err != nil {
			t.log.Debugf("get token failed on missing cache sig %s.out, ttl: %s", sig, cacheMaxAge)
			return nil, fmt.Errorf("get token failed: %w", err)
		}
		return []byte(token.AccessToken), nil
	})
	i, err, _ := singleFlightGrp.Do(sig, func() (interface{}, error) {
		return ageingcache.Output(o, sig, cacheMaxAge)
	})

	if err != nil {
		return "", err
	}
	if b, ok := i.([]byte); ok {
		return string(b), nil
	}
	return "", fmt.Errorf("unexpected returned token type")
}

// Clear removes the cached authentication token associated with the provided scope(s).
// Returns an error if the operation fails.
func (t *TokenFactory) Clear(scope ...string) error {

	return ageingcache.Clear(t.signature(scope...))
}

// newToken generates a new authentication token for the provided context and scope(s) using client credentials flow.
func (t *TokenFactory) newToken(ctx context.Context, scope ...string) (*Token, error) {
	if t.authConfig.BaseURL == "" {
		return nil, fmt.Errorf("empty auth config BaseURL")
	}
	if t.authInfo.ClientID == "" || t.authInfo.ClientSecret == "" {
		return nil, fmt.Errorf("client_id and client_secret are required")
	}
	if len(scope) == 0 {
		return nil, fmt.Errorf("scope is required")
	}

	requestURL := strings.TrimRight(t.authConfig.BaseURL, "/") + "/access_token"
	values := url.Values{}
	values.Set("grant_type", "client_credentials")
	scopes := t.formatScopeRequest(scope...)
	values.Set("scope", scopes)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(t.authInfo.ClientID, t.authInfo.ClientSecret)

	//client := &http.Client{Timeout: time.Duration(max(1, t.authConfig.Timeout)) * time.Second}
	//t.client
	client := *t.client
	client.Timeout = time.Duration(max(1, t.authConfig.Timeout)) * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("iam token request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var token Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("iam token response missing access_token")
	}
	return &token, nil
}

// signature returns a sanitized signature for the scopes.
func (t *TokenFactory) signature(scope ...string) string {
	sanitize := func(a ...string) string {
		var l []string
		for _, s := range a {
			sanitized := strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
					return r
				}
				return '-'
			}, s)
			l = append(l, sanitized)
		}
		return strings.Join(l, "-")
	}

	l := append([]string{}, t.authInfo.Signature)
	l = append(l, scope...)

	return fmt.Sprintf("sgcp-token-%s", sanitize(l...))
}

// formatScopeRequest constructs a formatted scope string that can be used to request new token,
// by prepending the account ID if it's not empty and processing each scope.
func (t *TokenFactory) formatScopeRequest(scopes ...string) string {
	if t.authInfo.AccountID == "" {
		return strings.Join(scopes, " ")
	}
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		out = append(out, t.authInfo.AccountID+":"+scope)
	}
	return strings.Join(out, " ")
}
