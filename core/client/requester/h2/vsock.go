package reqh2

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/securityprovider"
	"golang.org/x/net/http2"

	"github.com/opensvc/om3/v3/daemon/api"
)

const (
	// VsockPrefix is the scheme of the url of the daemon of a guest, reached
	// from its hypervisor: vsock://<context id>:<port>.
	VsockPrefix = "vsock://"

	// vsockIdleConnTimeout closes the connection of a client done with its
	// requests: the master makes one per operation on a guest.
	vsockIdleConnTimeout = 5 * time.Second
)

// ParseVsockURL returns the context id and the port of a vsock url.
func ParseVsockURL(s string) (cid, port uint32, err error) {
	host, portS, ok := strings.Cut(strings.TrimPrefix(s, VsockPrefix), ":")
	if !ok {
		return 0, 0, fmt.Errorf("%s: expect %s<context id>:<port>", s, VsockPrefix)
	}
	v, err := strconv.ParseUint(host, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("%s: context id: %w", s, err)
	}
	cid = uint32(v)
	if v, err = strconv.ParseUint(portS, 10, 32); err != nil {
		return 0, 0, fmt.Errorf("%s: port: %w", s, err)
	}
	return cid, uint32(v), nil
}

// NewVsock returns a client of the daemon of a guest. A daemon not running
// is an error at the first request, with no retry: the caller has other ways
// to reach the guest.
func NewVsock(config Config) (*api.ClientWithResponses, error) {
	cid, port, err := ParseVsockURL(config.URL)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http2.Transport{
			AllowHTTP:       true,
			IdleConnTimeout: vsockIdleConnTimeout,
			DialTLSContext: func(ctx context.Context, _, _ string, _ *tls.Config) (net.Conn, error) {
				return DialVsock(ctx, cid, port)
			},
		},
	}
	options := []api.ClientOption{api.WithHTTPClient(httpClient)}
	if config.Bearer != "" {
		provider, err := securityprovider.NewSecurityProviderBearerToken(config.Bearer)
		if err != nil {
			return nil, err
		}
		options = append(options, api.WithRequestEditorFn(provider.Intercept))
	}
	return api.NewClientWithResponses("http://localhost", options...)
}
