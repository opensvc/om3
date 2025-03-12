package reqh2

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/securityprovider"

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
