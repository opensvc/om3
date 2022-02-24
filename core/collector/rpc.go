package collector

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog"
	"github.com/ybbus/jsonrpc"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/logging"
	"opensvc.com/opensvc/util/xsession"
)

// Client exposes the jsonrpc2 Call function wrapped to add the auth arg
type Client struct {
	client jsonrpc.RPCClient
	secret string
	log    zerolog.Logger
}

func parseCollectorURL(s string) (*url.URL, error) {
	url, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	// sanitize
	url.Opaque = ""
	url.User = nil
	url.ForceQuery = false
	url.RawQuery = ""
	url.Fragment = ""
	url.RawFragment = ""

	// default scheme is https
	if url.Scheme == "" {
		url.Scheme = "https"
	}

	// dbopensvc = collector must be interpreted as a host-only url
	// but url.Parse sees that as a path-only
	if url.Host == "" && !strings.Contains(url.Path, "/") {
		url.Host = url.Path
		url.Path = ""
		url.RawPath = ""
	}

	// default path
	if url.Path == "" {
		url.Path = "/feed/default/call/jsonrpc2"
		url.RawPath = "/feed/default/call/jsonrpc2"
	}
	return url, nil
}

// NewClient returns a Client to call the collector jsonrpc2 methods.
func NewClient(endpoint, secret string) (*Client, error) {
	url, err := parseCollectorURL(endpoint)
	if err != nil {
		return nil, err
	}
	client := &Client{
		client: jsonrpc.NewClientWithOpts(url.String(), &jsonrpc.RPCClientOpts{
			HTTPClient: &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			},
		}),
		secret: secret,
		log: logging.Configure(logging.Config{
			ConsoleLoggingEnabled: false,
			EncodeLogsAsJSON:      true,
			FileLoggingEnabled:    true,
			Directory:             rawconfig.Node.Paths.Log,
			Filename:              "rpc.log",
			MaxSize:               5,
			MaxBackups:            1,
			MaxAge:                30,
			WithCaller:            logging.WithCaller,
		}).
			With().
			Str("n", hostname.Hostname()).
			Str("sid", xsession.ID).
			Logger(),
	}
	return client, nil
}

func (t Client) paramsWithAuth(params []interface{}) []interface{} {
	return append(params, []string{t.secret, hostname.Hostname()})
}

// Call executes a jsonrpc2 collector call and returns the response.
func (t Client) Call(method string, params ...interface{}) (*jsonrpc.RPCResponse, error) {
	t.log.Info().Str("method", method).Interface("params", params).Msg("call")
	response, err := t.client.Call(method, t.paramsWithAuth(params))
	if err != nil {
		t.log.Error().Str("method", method).Interface("params", params).Err(err).Msg("call")
	}
	return response, err
}
