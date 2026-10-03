package reqh2

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/securityprovider"

	"github.com/opensvc/om3/v3/core/client/tokencache"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/util/httpclientcache"
	"github.com/opensvc/om3/v3/util/oidc"

	"golang.org/x/net/http2"
)

type (
	// Config is the agent HTTP/2 requester configuration
	Config struct {
		Certificate        string
		Key                string
		Username           string
		Password           string `json:"-"`
		URL                string `json:"url"`
		Authorization      string `json:"-"`
		Bearer             string `json:"-"`
		Timeout            time.Duration
		InsecureSkipVerify bool
		RootCA             string
		Tokens             tokencache.Entry
	}

	RefreshTransport struct {
		Base     http.RoundTripper
		baseURL  string
		tokens   tokencache.Entry
		Username string
		Password string

		// explicitAuth says the requests carry a credential their caller
		// chose, as the scoped token a cluster join or leave presents: a
		// response refusing it is the caller's answer, and is not retried
		// with the credentials the context has cached, which would run the
		// request as someone else.
		explicitAuth bool
	}
)

const (
	UDSPrefix  = "http:///"
	InetPrefix = "https://"

	authURLPath = "/api/auth/token"
)

var (
	udsRetryConnect      = 10
	udsRetryConnectDelay = 10 * time.Millisecond
)

func (t Config) String() string {
	b, _ := json.Marshal(t)
	return "H2" + string(b)
}

func NewUDSClient(config Config) *http.Client {
	if config.URL == "" {
		config.URL = daemonenv.HTTPUnixFile()
	}
	tp := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (con net.Conn, err error) {
			i := 0
			for {
				i++
				con, err = net.Dial("unix", config.URL)
				if err == nil {
					return
				}
				if i >= udsRetryConnect {
					return
				}
				if strings.Contains(err.Error(), "connect: connection refused") {
					time.Sleep(udsRetryConnectDelay)
					continue
				}
			}
		},
	}
	httpClient := &http.Client{
		Transport: tp,
		Timeout:   config.Timeout,
	}
	return httpClient
}

func NewUDS(config Config) (apiClient *api.ClientWithResponses, err error) {
	httpClient := NewUDSClient(config)
	return api.NewClientWithResponses("http://localhost", api.WithHTTPClient(httpClient))
}

// NewInet returns api *api.ClientWithResponses from config.
//
//	request authorization header will be created from one of config properties:
//	- Username & Password
//	- Bearer
//	- Authorization
func NewInet(config Config) (apiClient *api.ClientWithResponses, err error) {
	cachedClient, err := httpclientcache.Client(httpclientcache.Options{
		CertFile:           config.Certificate,
		KeyFile:            config.Key,
		Timeout:            config.Timeout,
		InsecureSkipVerify: config.InsecureSkipVerify,
		RootCA:             config.RootCA,
	})
	if err != nil {
		return nil, err
	}

	baseTransport := cachedClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}

	if !strings.Contains(config.URL[8:], ":") {
		config.URL += fmt.Sprintf(":%d", daemonenv.HTTPPort)
	}

	httpClient := *cachedClient
	httpClient.Transport = &RefreshTransport{
		Base:     baseTransport,
		baseURL:  config.URL,
		tokens:   config.Tokens,
		Username: config.Username,
		Password: config.Password,
		// The bearer of the cached tokens is the context's own, which
		// the transport refreshes. Another one, or an authorization
		// header, is the caller's.
		explicitAuth: config.Authorization != "" || (config.Bearer != "" && config.Bearer != config.Tokens.AccessToken),
	}

	options := []api.ClientOption{api.WithHTTPClient(&httpClient)}

	if config.Username != "" && config.Password != "" {
		provider, err := securityprovider.NewSecurityProviderBasicAuth(config.Username, config.Password)
		if err != nil {
			return nil, err
		}
		options = append(options, api.WithRequestEditorFn(provider.Intercept))
	}
	if config.Bearer != "" {
		provider, err := securityprovider.NewSecurityProviderBearerToken(config.Bearer)
		if err != nil {
			return nil, err
		}
		options = append(options, api.WithRequestEditorFn(provider.Intercept))
	}

	if config.Authorization != "" {
		fn := requestAuthorizationEditorFn(config.Authorization)
		options = append(options, api.WithRequestEditorFn(fn))
	}

	if apiClient, err = api.NewClientWithResponses(config.URL, options...); err != nil {
		return apiClient, err
	} else {
		return apiClient, nil
	}
}

// requestAuthorizationEditorFn returns request editor function that sets the
// request authorization header.
func requestAuthorizationEditorFn(s string) func(context.Context, *http.Request) error {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", s)
		return nil
	}
}

func (t *RefreshTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	base := t.getBaseTransport()

	reqClone := req.Clone(ctx)
	if strings.HasSuffix(req.URL.Path, authURLPath) && (reqClone.Header != nil && strings.HasSuffix(reqClone.Header.Get("Authorization"), t.tokens.AccessToken)) {
		reqClone.Header.Del("Authorization")
		if t.Username != "" && t.Password != "" {
			reqClone.SetBasicAuth(t.Username, t.Password)
		}
	}

	resp, err := base.RoundTrip(reqClone)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	if t.explicitAuth {
		return resp, nil
	}

	hasTokens := t.tokens.AccessToken != "" || t.tokens.RefreshToken != ""
	hasCredentials := t.Username != "" && t.Password != ""

	if !hasTokens && hasCredentials {
		return resp, nil
	}

	if !hasTokens && env.Context() != "" {
		// A context never logged in may share the openid issuer and
		// client of one that was: its tokens are valid here too.
		if adopted := t.adoptOpenID(ctx, base); adopted {
			_ = resp.Body.Close()
			newToken, err := t.refreshOpenID(ctx, time.Now())
			if err != nil {
				return nil, err
			}
			return t.retryWithToken(ctx, req, base, newToken)
		}
	}

	_ = resp.Body.Close()

	newToken, err := t.authenticateOrRefresh(ctx, base)
	if err != nil {
		return nil, err
	}

	return t.retryWithToken(ctx, req, base, newToken)
}

func (t *RefreshTransport) getBaseTransport() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func (t *RefreshTransport) isAccessTokenValid() bool {
	return t.tokens.AccessToken != "" && time.Now().Before(t.tokens.AccessTokenExpire)
}

func (t *RefreshTransport) retryWithAccessToken(ctx context.Context, req *http.Request, base http.RoundTripper) (*http.Response, error) {
	retryReq := req.Clone(ctx)
	retryReq.Header.Set("Authorization", "Bearer "+t.tokens.AccessToken)
	return base.RoundTrip(retryReq)
}

func (t *RefreshTransport) retryWithToken(ctx context.Context, req *http.Request, base http.RoundTripper, token string) (*http.Response, error) {
	if token == "" {
		return nil, fmt.Errorf("no valid tokens available")
	}
	retryReq := req.Clone(ctx)
	retryReq.Header.Set("Authorization", "Bearer "+token)
	return base.RoundTrip(retryReq)
}

func (t *RefreshTransport) authenticateOrRefresh(ctx context.Context, base http.RoundTripper) (string, error) {
	now := time.Now()

	if t.tokens.OpenID != nil {
		return t.refreshOpenID(ctx, now)
	}

	if t.tokens.AccessToken == "" && t.tokens.RefreshToken == "" {
		return t.authenticateWithCredentials(ctx, base, "no access or refresh tokens available, use `ox context login` to authenticate")
	}

	if now.After(t.tokens.RefreshTokenExpire) {
		return t.authenticateWithCredentials(ctx, base, "both access and refresh tokens are expired, use `ox context login` to reauthenticate")
	}

	if now.After(t.tokens.AccessTokenExpire) {
		return t.refreshAccessToken(ctx, base)
	}

	return t.tokens.AccessToken, nil
}

func (t *RefreshTransport) authenticateWithCredentials(ctx context.Context, base http.RoundTripper, errorMessage string) (string, error) {
	if t.Username == "" || t.Password == "" {
		return "", errors.New(errorMessage)
	}

	params := url.Values{}
	params.Add("refresh", "true")

	loginURL := strings.TrimRight(t.baseURL, "/") + authURLPath + "?" + params.Encode()
	loginReq, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, nil)
	if err != nil {
		return "", err
	}

	loginReq.SetBasicAuth(t.Username, t.Password)
	loginResp, err := base.RoundTrip(loginReq)
	if err != nil {
		return "", err
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("authentication failed with status: %d", loginResp.StatusCode)
	}

	var tokenResp tokencache.Entry
	if err := json.NewDecoder(loginResp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		return "", fmt.Errorf("tokens login response missing access_token or refresh_token")
	}

	t.updateTokens(tokenResp)
	return tokenResp.AccessToken, tokencache.Save(env.Context(), t.tokens)
}

func (t *RefreshTransport) refreshAccessToken(ctx context.Context, base http.RoundTripper) (string, error) {
	refreshURL := strings.TrimRight(t.baseURL, "/") + "/api/auth/refresh"
	if t.tokens.AccessTokenDuration != nil && t.tokens.AccessTokenDuration.Positive() {
		refreshURL += "?access_duration=" + t.tokens.AccessTokenDuration.String()
	}
	refreshReq, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, nil)
	if err != nil {
		return "", err
	}

	refreshReq.Header.Set("Authorization", "Bearer "+t.tokens.RefreshToken)
	refreshResp, err := base.RoundTrip(refreshReq)
	if err != nil {
		return "", err
	}
	defer refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tokens refresh failed with status: %d", refreshResp.StatusCode)
	}

	var tokenResp struct {
		AccessToken       string    `json:"access_token"`
		AccessTokenExpire time.Time `json:"access_expired_at"`
	}

	if err := json.NewDecoder(refreshResp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("tokens refresh response missing access_token")
	}

	t.tokens.AccessToken = tokenResp.AccessToken
	t.tokens.AccessTokenExpire = tokenResp.AccessTokenExpire
	return tokenResp.AccessToken, tokencache.Save(env.Context(), t.tokens)
}

// adoptOpenID takes the openid tokens cached for the issuer and client the
// cluster trusts, when another context logged in there, and says whether it
// did. The cluster tells them without authentication.
func (t *RefreshTransport) adoptOpenID(ctx context.Context, base http.RoundTripper) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(t.baseURL, "/")+"/api/auth/info", nil)
	if err != nil {
		return false
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var info api.AuthInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil || info.Openid == nil || info.Openid.Issuer == "" {
		return false
	}
	shared := tokencache.FindOpenID(info.Openid.Issuer, info.Openid.ClientId)
	if shared == nil || !shared.HasValidRefresh(time.Now()) {
		return false
	}
	t.tokens = *shared
	return true
}

// refreshOpenID obtains a new access token from the openid issuer that gave
// the tokens. The cluster does not refresh a token it did not issue.
//
// The new tokens are saved for the other contexts logged in at the same
// issuer and client, which share them.
func (t *RefreshTransport) refreshOpenID(ctx context.Context, now time.Time) (string, error) {
	ref := t.tokens.OpenID
	if !t.tokens.HasValidRefresh(now) {
		return "", fmt.Errorf("the openid refresh token of %s is expired, use `ox context login` to reauthenticate", ref.Issuer)
	}
	tok, err := oidc.Refresh(ctx, ref.TokenEndpoint, ref.ClientID, t.tokens.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("%w, use `ox context login` to reauthenticate", err)
	}
	t.tokens.AccessToken = tok.AccessToken
	t.tokens.AccessTokenExpire = tok.AccessExpiry
	t.tokens.RefreshToken = tok.RefreshToken
	if !tok.RefreshExpiry.IsZero() {
		t.tokens.RefreshTokenExpire = tok.RefreshExpiry
	}
	return tok.AccessToken, tokencache.Save(env.Context(), t.tokens)
}

func (t *RefreshTransport) updateTokens(token tokencache.Entry) {
	t.tokens.AccessToken = token.AccessToken
	t.tokens.AccessTokenExpire = token.AccessTokenExpire
	t.tokens.RefreshToken = token.RefreshToken
	t.tokens.RefreshTokenExpire = token.RefreshTokenExpire
}
