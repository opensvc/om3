package reqh2

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/util/httpclientcache"

	"golang.org/x/net/http2"
)

type (
	// T is the agent HTTP/2 requester
	T struct {
		Client      api.ClientWithResponses
		Certificate string
		Username    string
		Password    string `json:"-"`
		URL         string `json:"url"`
		Bearer      string `json:"-"`
	}
)

const (
	UDSPrefix  = "http:///"
	InetPrefix = "https://"
)

var (
	udsRetryConnect      = 10
	udsRetryConnectDelay = 10 * time.Millisecond

	clientTimeout = 5 * time.Second
)

func (t T) String() string {
	b, _ := json.Marshal(t)
	return "H2" + string(b)
}

func defaultUDSPath() string {
	return filepath.FromSlash(fmt.Sprintf("%s/lsnr/h2.sock", rawconfig.Paths.Var))
}

func NewUDS(url string) (apiClient *api.ClientWithResponses, err error) {
	if url == "" {
		url = defaultUDSPath()
	}
	tp := &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (con net.Conn, err error) {
			i := 0
			for {
				i++
				con, err = net.Dial("unix", url)
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
		Timeout:   clientTimeout,
	}
	if apiClient, err = api.NewClientWithResponses("http://localhost", api.WithHTTPClient(httpClient)); err != nil {
		return apiClient, err
	} else {
		return apiClient, nil
	}
}

func NewInet(url, clientCertificate, clientKey string, insecureSkipVerify bool, username, password string, bearer string, rootCa string) (apiClient *api.ClientWithResponses, err error) {
	httpClient, err := httpclientcache.Client(httpclientcache.Options{
		CertFile:           clientCertificate,
		KeyFile:            clientKey,
		Timeout:            clientTimeout,
		InsecureSkipVerify: insecureSkipVerify,
		RootCA:             rootCa,
	})
	if err != nil {
		return nil, err
	}
	if !strings.Contains(url[8:], ":") {
		url += fmt.Sprintf(":%d", daemonenv.HttpPort)
	}

	options := []api.ClientOption{api.WithHTTPClient(httpClient)}

	if username != "" && password != "" {
		provider, err := securityprovider.NewSecurityProviderBasicAuth(username, password)
		if err != nil {
			return nil, err
		}
		options = append(options, api.WithRequestEditorFn(provider.Intercept))
	}

	if bearer != "" {
		provider, err := securityprovider.NewSecurityProviderBearerToken(bearer)
		if err != nil {
			return nil, err
		}
		options = append(options, api.WithRequestEditorFn(provider.Intercept))
	}

	if apiClient, err = api.NewClientWithResponses(url, options...); err != nil {
		return apiClient, err
	} else {
		return apiClient, nil
	}
}
