package collector

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/ybbus/jsonrpc"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/logging"
	"github.com/opensvc/om3/util/xsession"
)

var (
	Alive atomic.Bool
)

// Client exposes the jsonrpc2 Call function wrapped to add the auth arg
type Client struct {
	client jsonrpc.RPCClient
	secret string
	log    zerolog.Logger
}

func (c Client) NewPinger(d time.Duration) func() {
	stop := make(chan bool)
	go func() {
		ticker := time.NewTicker(d)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				break
			case <-ticker.C:
				c.Ping()
			}
		}
	}()
	stopFunc := func() {
		stop <- true
	}
	return stopFunc
}

func (c *Client) Ping() {
	alive := Alive.Load()
	_, err := c.Call("daemon_ping")
	c.log.Debug().Bool("alive", alive).Msgf("ping collector: %s", err)
	switch {
	case (err != nil) && alive:
		c.log.Info().Msgf("disable collector clients: %s", err)
		Alive.Store(false)
	case (err == nil) && !alive:
		c.log.Info().Msgf("enable collector clients")
		Alive.Store(true)
	}
}

func ComplianceURL(s string) (*url.URL, error) {
	if url, err := BaseURL(s); err != nil {
		return nil, err
	} else {
		// default path
		if url.Path == "" {
			url.Path = "/init/compliance/call/jsonrpc2"
			url.RawPath = "/init/compliance/call/jsonrpc2"
		}
		return url, nil
	}
}

func InitURL(s string) (*url.URL, error) {
	if url, err := BaseURL(s); err != nil {
		return nil, err
	} else {
		// default path
		if url.Path == "" {
			url.Path = "/init/default/call/jsonrpc2"
			url.RawPath = "/init/default/call/jsonrpc2"
		}
		return url, nil
	}
}

func FeedURL(s string) (*url.URL, error) {
	if url, err := BaseURL(s); err != nil {
		return nil, err
	} else {
		// default path
		if url.Path == "" {
			url.Path = "/feed/default/call/jsonrpc2"
			url.RawPath = "/feed/default/call/jsonrpc2"
		}
		return url, nil
	}
}

func RestURL(s string) (*url.URL, error) {
	if url, err := BaseURL(s); err != nil {
		return nil, err
	} else {
		// default path
		url.Path = "/init/rest/api"
		url.RawPath = "/init/rest/api"
		return url, nil
	}
}

func BaseURL(s string) (*url.URL, error) {
	url, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
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

	return url, nil
}

// NewFeedClient returns a Client to call the collector feed app jsonrpc2 methods.
func NewFeedClient(endpoint, secret string) (*Client, error) {
	url, err := FeedURL(endpoint)
	if err != nil {
		return nil, err
	}
	return newClient(url, secret)
}

// NewComplianceClient returns a Client to call the collector init app jsonrpc2 methods.
func NewComplianceClient(endpoint, secret string) (*Client, error) {
	url, err := ComplianceURL(endpoint)
	if err != nil {
		return nil, err
	}
	return newClient(url, secret)
}

// NewInitClient returns a Client to call the collector init app jsonrpc2 methods.
func NewInitClient(endpoint, secret string) (*Client, error) {
	url, err := InitURL(endpoint)
	if err != nil {
		return nil, err
	}
	return newClient(url, secret)
}

func newClient(url *url.URL, secret string) (*Client, error) {
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
			Directory:             rawconfig.Paths.Log,
			Filename:              "rpc.log",
			MaxSize:               5,
			MaxBackups:            1,
			MaxAge:                30,
		}).
			With().
			Str("n", hostname.Hostname()).
			Stringer("sid", xsession.ID).
			Logger(),
	}
	return client, nil
}

func (t Client) paramsWithAuth(params []interface{}) []interface{} {
	return append(params, []string{t.secret, hostname.Hostname()})
}

func LogSimpleResponse(response *jsonrpc.RPCResponse, log zerolog.Logger) {
	switch m := response.Result.(type) {
	case map[string]interface{}:
		if info, ok := m["info"]; ok {
			switch v := info.(type) {
			case string:
				log.Info().Msg(v)
			case []string:
				for _, s := range v {
					log.Info().Msg(s)
				}
			}
		}
		if err, ok := m["error"]; ok {
			switch v := err.(type) {
			case string:
				log.Error().Msg(v)
			case []string:
				for _, s := range v {
					log.Error().Msg(s)
				}
			}
		}
	}
}

// Call executes a jsonrpc2 collector call and returns the response.
func (t Client) Call(method string, params ...interface{}) (*jsonrpc.RPCResponse, error) {
	t.log.Info().Str("method", method).Interface("params", params).Msg("call")
	response, err := t.client.Call(method, t.paramsWithAuth(params))
	if err != nil {
		t.log.Error().Str("method", method).Interface("params", params).Err(err).Msg("call")
	}
	if response != nil && response.Error != nil {
		t.log.Error().Str("method", method).Interface("params", params).Interface("data", response.Error.Data).Int("code", response.Error.Code).Msg(response.Error.Message)
	}
	return response, err
}

func (t Client) CallFor(out interface{}, method string, params ...interface{}) error {
	t.log.Info().Str("method", method).Interface("params", params).Msg("call")
	err := t.client.CallFor(out, method, t.paramsWithAuth(params))
	if err != nil {
		t.log.Error().Str("method", method).Interface("params", params).Err(err).Msg("call")
	}
	return err
}
