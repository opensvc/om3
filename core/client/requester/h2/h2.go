package reqh2

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/securityprovider"

	reqtoken "github.com/opensvc/om3/core/client/token"
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
		Token              reqtoken.Token
	}

	RefreshTransport struct {
		Base     http.RoundTripper
		baseURL  string
		token    reqtoken.Token
		Username string
		Password string
	}
)

const (
	UDSPrefix  = "http:///"
	InetPrefix = "https://"
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
		token:    config.Token,
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

	resp, err := base.RoundTrip(req.Clone(ctx))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	defer resp.Body.Close()

	hasTokens := t.token.AccessToken != "" || t.token.RefreshToken != ""
	hasCredentials := t.Username != "" && t.Password != ""

	if !hasTokens && hasCredentials {
		return resp, nil
	}

	if t.isAccessTokenValid() {
		lreq := req.Clone(ctx)
		lreq.Header.Set("Authorization", "Bearer "+t.token.AccessToken)
		return base.RoundTrip(lreq)
	}

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
	return t.token.AccessToken != "" && time.Now().Before(t.token.AccessTokenExpire)
}

func (t *RefreshTransport) retryWithAccessToken(ctx context.Context, req *http.Request, base http.RoundTripper) (*http.Response, error) {
	retryReq := req.Clone(ctx)
	retryReq.Header.Set("Authorization", "Bearer "+t.token.AccessToken)
	return base.RoundTrip(retryReq)
}

func (t *RefreshTransport) retryWithToken(ctx context.Context, req *http.Request, base http.RoundTripper, token string) (*http.Response, error) {
	if token == "" {
		return nil, fmt.Errorf("no valid token available")
	}
	retryReq := req.Clone(ctx)
	retryReq.Header.Set("Authorization", "Bearer "+token)
	return base.RoundTrip(retryReq)
}

func (t *RefreshTransport) authenticateOrRefresh(ctx context.Context, base http.RoundTripper) (string, error) {
	now := time.Now()

	if t.token.AccessToken == "" && t.token.RefreshToken == "" {
		return t.authenticateWithCredentials(ctx, base, "no access or refresh token available, use `om daemon login` to authenticate")
	}

	if now.After(t.token.RefreshTokenExpire) {
		return t.authenticateWithCredentials(ctx, base, "both access and refresh tokens are expired, use `om daemon login` to reauthenticate")
	}

	if now.After(t.token.AccessTokenExpire) {
		return t.refreshAccessToken(ctx, base)
	}

	return t.token.AccessToken, nil
}

func (t *RefreshTransport) authenticateWithCredentials(ctx context.Context, base http.RoundTripper, errorMessage string) (string, error) {
	if t.Username == "" || t.Password == "" {
		return "", fmt.Errorf(errorMessage)
	}

	params := url.Values{}
	params.Add("refresh", "true")

	loginURL := strings.TrimRight(t.baseURL, "/") + "/api/auth/token?" + params.Encode()
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

	var tokenResp reqtoken.Token
	if err := json.NewDecoder(loginResp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if tokenResp.AccessToken == "" || tokenResp.RefreshToken == "" {
		return "", fmt.Errorf("token login response missing access_token or refresh_token")
	}

	t.updateToken(tokenResp)
	return tokenResp.AccessToken, reqtoken.SaveToken(env.Context(), t.token)
}

func (t *RefreshTransport) refreshAccessToken(ctx context.Context, base http.RoundTripper) (string, error) {
	refreshURL := strings.TrimRight(t.baseURL, "/") + "/api/auth/refresh"
	refreshReq, err := http.NewRequestWithContext(ctx, http.MethodPost, refreshURL, nil)
	if err != nil {
		return "", err
	}

	refreshReq.Header.Set("Authorization", "Bearer "+t.token.RefreshToken)
	refreshResp, err := base.RoundTrip(refreshReq)
	if err != nil {
		return "", err
	}
	defer refreshResp.Body.Close()

	if refreshResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token refresh failed with status: %d", refreshResp.StatusCode)
	}

	var tokenResp struct {
		AccessToken       string    `json:"access_token"`
		AccessTokenExpire time.Time `json:"access_expired_at"`
	}

	if err := json.NewDecoder(refreshResp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("token refresh response missing access_token")
	}

	t.token.AccessToken = tokenResp.AccessToken
	t.token.AccessTokenExpire = tokenResp.AccessTokenExpire
	return tokenResp.AccessToken, reqtoken.SaveToken(env.Context(), t.token)
}

func (t *RefreshTransport) updateToken(token reqtoken.Token) {
	t.token.AccessToken = token.AccessToken
	t.token.AccessTokenExpire = token.AccessTokenExpire
	t.token.RefreshToken = token.RefreshToken
	t.token.RefreshTokenExpire = token.RefreshTokenExpire
}
