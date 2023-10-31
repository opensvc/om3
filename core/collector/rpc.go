package collector

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ybbus/jsonrpc"

	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/plog"
)

var (
	Alive atomic.Bool
)

// Client exposes the jsonrpc2 Call function wrapped to add the auth arg
type Client struct {
	client jsonrpc.RPCClient
	secret string
	log    *plog.Logger
}

func (c Client) NewPinger(d time.Duration) func() {
	stop := make(chan bool)
	go func() {
		ticker := time.NewTicker(d)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
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
	c.log.Attr("collector_alive", alive).Debugf("ping collector")
	switch {
	case (err != nil) && alive:
		c.log.Infof("disable collector clients: %s", err)
		Alive.Store(false)
	case (err == nil) && !alive:
		c.log.Infof("enable collector clients")
		Alive.Store(true)
	}
}

func ComplianceURL(s string) (*url.URL, error) {
	if url, err := BaseURL(s); err != nil {
		return nil, err
	} else if url.Host == "" {
		return nil, fmt.Errorf("collector compliance url host is empty")
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
	} else if url.Host == "" {
		return nil, fmt.Errorf("collector url host is empty")
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
	} else if url.Host == "" {
		return nil, fmt.Errorf("collector feed url host is empty")
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
		log:    plog.NewDefaultLogger().WithPrefix("collector: rpc: ").Attr("pkg", "core/collector/rpc"),
	}
	return client, nil
}

func (t Client) paramsWithAuth(params []interface{}) []interface{} {
	return append(params, []string{t.secret, hostname.Hostname()})
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
func (t Client) Call(method string, params ...interface{}) (*jsonrpc.RPCResponse, error) {
	response, err := t.client.Call(method, t.paramsWithAuth(params))
	l := t.log.Attr("collector_rpc_method", method).Attr("collector_rpc_params", params)
	if response != nil && response.Error != nil {
		l.Attr("collector_rpc_response_data", response.Error.Data).
			Attr("collector_rpc_response_code", response.Error.Code).
			Errorf("call: %s: %s", response.Error.Message, response.Error.Data)
	} else if err != nil {
		l.Errorf("call: %s: %s", method, err)
	} else {
		l.Infof("call: %s", method)
	}
	return response, err
}

func (t Client) CallFor(out interface{}, method string, params ...interface{}) error {
	l := t.log.Attr("collector_rpc_method", method).Attr("collector_rpc_params", params)
	err := t.client.CallFor(out, method, t.paramsWithAuth(params))
	if err != nil {
		l.Errorf("call for: %s: %s", method, err)
	} else {
		l.Infof("call for: %s", method)
	}
	return err
}
