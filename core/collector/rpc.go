package collector

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/ybbus/jsonrpc"

	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
)

var (
	Alive atomic.Bool
)

type (
	// Client exposes the jsonrpc2 Call function wrapped to add the auth arg
	Client struct {
		client   jsonrpc.RPCClient
		endpoint string
		secret   string
		log      *plog.Logger
	}
	Pinger struct {
		ctx    context.Context
		cancel context.CancelFunc
		client *Client
		id     uuid.UUID
	}

	// pinger command channel messages
	pingerStop    struct{}
	pingerStopped struct{}
)

func (c *Client) String() string {
	return c.endpoint
}

func (c *Client) SetLogger(log *plog.Logger) {
	c.log = log
}

func (c *Client) NewPinger() *Pinger {
	pinger := Pinger{
		id:     uuid.New(),
		client: c,
	}
	return &pinger
}

func (t *Pinger) Start(ctx context.Context, interval time.Duration) {
	t.ctx, t.cancel = context.WithCancel(ctx)
	go func() {
		defer t.cancel()
		t.client.log.Infof("collector pinger %s started", t.id)
		defer t.client.log.Infof("collector pinger %s stopped", t.id)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if !Alive.Load() {
					if t.client.Ping() {
						t.client.log.Infof("enable collector clients")
						Alive.Store(true)
					}
				}
			case <-t.ctx.Done():
				return
			}
		}
	}()
}

func (t *Pinger) Stop() {
	if t == nil {
		return
	}
	if t.cancel != nil {
		t.cancel()
	}
}

func (c *Client) Ping() bool {
	_, err := c.Call("daemon_ping")
	switch {
	case err == nil:
		return true
	default:
		return false
	}
}

func ComplianceURL(s string) (*url.URL, error) {
	if u, err := BaseURL(s); err != nil {
		return nil, err
	} else if u.Host == "" {
		return nil, fmt.Errorf("collector compliance url host is empty")
	} else {
		// default path
		if u.Path == "" {
			u.Path = "/init/compliance/call/jsonrpc2"
			u.RawPath = "/init/compliance/call/jsonrpc2"
		}
		return u, nil
	}
}

func InitURL(s string) (*url.URL, error) {
	if u, err := BaseURL(s); err != nil {
		return nil, err
	} else if u.Host == "" {
		return nil, fmt.Errorf("collector url host is empty")
	} else {
		// default path
		if u.Path == "" {
			u.Path = "/init/default/call/jsonrpc2"
			u.RawPath = "/init/default/call/jsonrpc2"
		}
		return u, nil
	}
}

func FeedURL(s string) (*url.URL, error) {
	if u, err := BaseURL(s); err != nil {
		return nil, err
	} else if u.Host == "" {
		return nil, fmt.Errorf("collector feed url host is empty")
	} else {
		// default path
		if u.Path == "" {
			u.Path = "/feed/default/call/jsonrpc2"
			u.RawPath = "/feed/default/call/jsonrpc2"
		}
		return u, nil
	}
}

func RestURL(s string) (*url.URL, error) {
	if u, err := BaseURL(s); err != nil {
		return nil, err
	} else {
		// default path
		u.Path = "/init/rest/api"
		u.RawPath = "/init/rest/api"
		return u, nil
	}
}

func BaseURL(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	// sanitize
	u.Opaque = ""
	u.User = nil
	u.ForceQuery = false
	u.RawQuery = ""
	u.Fragment = ""
	u.RawFragment = ""

	// default scheme is https
	if u.Scheme == "" {
		u.Scheme = "https"
	}

	// dbopensvc = collector must be interpreted as a host-only url
	// but url.Parse sees that as a path-only
	if u.Host == "" && !strings.Contains(u.Path, "/") {
		u.Host = u.Path
		u.Path = ""
		u.RawPath = ""
	}

	return u, nil
}

// NewFeedClient returns a Client to call the collector feed app jsonrpc2 methods.
func NewFeedClient(endpoint, secret string) (*Client, error) {
	u, err := FeedURL(endpoint)
	if err != nil {
		return nil, err
	}
	return newClient(u, secret)
}

// NewComplianceClient returns a Client to call the collector init app jsonrpc2 methods.
func NewComplianceClient(endpoint, secret string) (*Client, error) {
	u, err := ComplianceURL(endpoint)
	if err != nil {
		return nil, err
	}
	return newClient(u, secret)
}

// NewInitClient returns a Client to call the collector init app jsonrpc2 methods.
func NewInitClient(endpoint, secret string) (*Client, error) {
	u, err := InitURL(endpoint)
	if err != nil {
		return nil, err
	}
	return newClient(u, secret)
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
		endpoint: url.String(),
		secret:   secret,
		log:      plog.NewDefaultLogger().WithPrefix("collector: rpc: ").Attr("pkg", "core/collector/rpc"),
	}
	return client, nil
}

func (c *Client) paramsWithAuth(params []interface{}) []interface{} {
	return append(params, []string{c.secret, hostname.Hostname()})
}

func LogSimpleResponse(response *jsonrpc.RPCResponse, log *plog.Logger) {
	switch m := response.Result.(type) {
	case map[string]interface{}:
		if info, ok := m["info"]; ok {
			switch v := info.(type) {
			case string:
				log.Infof(v)
			case []string:
				for _, s := range v {
					log.Infof(s)
				}
			}
		}
		if err, ok := m["error"]; ok {
			switch v := err.(type) {
			case string:
				log.Errorf(v)
			case []string:
				for _, s := range v {
					log.Errorf(s)
				}
			}
		}
	}
}

// Call executes a jsonrpc2 collector call and returns the response.
func (c *Client) Call(method string, params ...interface{}) (*jsonrpc.RPCResponse, error) {
	response, err := c.client.Call(method, c.paramsWithAuth(params))
	l := c.log.Attr("collector_rpc_method", method).Attr("collector_rpc_params", params)
	if response != nil && response.Error != nil {
		l.Attr("collector_rpc_response_data", response.Error.Data).Attr("collector_rpc_response_code", response.Error.Code).Debugf("call: %s: %s", response.Error.Message, response.Error.Data)
	} else if err != nil {
		if Alive.Load() {
			l.Errorf("disable collector clients: call: %s: %s", method, err)
			Alive.Store(false)
		}
	} else {
		l.Infof("call: %s", method)
	}
	return response, err
}

func (c *Client) CallFor(out interface{}, method string, params ...interface{}) error {
	l := c.log.Attr("collector_rpc_method", method).Attr("collector_rpc_params", params)
	err := c.client.CallFor(out, method, c.paramsWithAuth(params))
	if err != nil {
		l.Errorf("call for: %s: %s", method, err)
	} else {
		l.Infof("call for: %s", method)
	}
	return err
}
