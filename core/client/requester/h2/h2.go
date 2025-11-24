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

	"github.com/opensvc/om3/core/client/tokencache"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/util/httpclientcache"

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

func NewUDS(config Config) (apiClient *api.ClientWithResponses, err error) {
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
	if apiClient, err = api.NewClientWithResponses("http://localhost", api.WithHTTPClient(httpClient)); err != nil {
		return apiClient, err
	} else {
		return apiClient, nil
	}
}

// NewInet returns api *api.ClientWithResponses from config.
//
//	request authorization header will be created from one of config properties:
//	- Username & Password
//	- Bearer
//	- Authorization
func NewInet(config Config) (apiClient *api.ClientWithResponses, err error) {
	httpClient, err := httpclientcache.Client(httpclientcache.Options{
		CertFile:           config.Certificate,
		KeyFile:            config.Key,
		Timeout:            config.Timeout,
		InsecureSkipVerify: config.InsecureSkipVerify,
		RootCA:             config.RootCA,
	})
	if err != nil {
		return nil, err
	}

	baseTransport := httpClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	httpClient.Transport = &RefreshTransport{
		Base:     baseTransport,
		baseURL:  config.URL,
		tokens:   config.Tokens,
		Username: config.Username,
		Password: config.Password,
	}

	if !strings.Contains(config.URL[8:], ":") {
		config.URL += fmt.Sprintf(":%d", daemonenv.HTTPPort)
	}

	options := []api.ClientOption{api.WithHTTPClient(httpClient)}

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

	hasTokens := t.tokens.AccessToken != "" || t.tokens.RefreshToken != ""
	hasCredentials := t.Username != "" && t.Password != ""

	if !hasTokens && hasCredentials {
		return resp, nil
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

	if t.tokens.AccessToken == "" && t.tokens.RefreshToken == "" {
		return t.authenticateWithCredentials(ctx, base, "no access or refresh tokens available, use `om context login` to authenticate")
	}

	if now.After(t.tokens.RefreshTokenExpire) {
		return t.authenticateWithCredentials(ctx, base, "both access and refresh tokens are expired, use `om context login` to reauthenticate")
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

func (t *RefreshTransport) updateTokens(token tokencache.Entry) {
	t.tokens.AccessToken = token.AccessToken
	t.tokens.AccessTokenExpire = token.AccessTokenExpire
	t.tokens.RefreshToken = token.RefreshToken
	t.tokens.RefreshTokenExpire = token.RefreshTokenExpire
}
