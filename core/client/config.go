package client

import (
	"fmt"
	"strings"
)

type (
	// Config is the agent api client configuration
	Config struct {
		url                string
		insecureSkipVerify bool
		clientCertificate  string
		clientKey          string
	}
)

// NewConfig allocates a new client configuration and returns the reference
// so users are not tempted to use client.Config{} dereferenced, which would
// make loadContext useless.
func NewConfig() *Config {
	return &Config{}
}

// SetURL sets the config key.
func (c *Config) SetURL(url string) *Config {
	c.url = url
	return c
}

// SetInsecureSkipVerify sets the config key.
func (c *Config) SetInsecureSkipVerify(b bool) *Config {
	c.insecureSkipVerify = b
	return c
}

// SetClientCertificate sets the config key.
func (c *Config) SetClientCertificate(s string) *Config {
	c.clientCertificate = s
	return c
}

// SetClientKey sets the config key.
func (c *Config) SetClientKey(s string) *Config {
	c.clientKey = s
	return c
}

// NewAPI allocates a new api with a requester for the server found in Config,
// or for the server found in Context.
func (c *Config) NewAPI() (API, error) {
	a := &API{}
	if c.url == "" {
		if err := c.loadContext(); err != nil {
			return *a, err
		}
	}
	r, err := c.newRequester()
	if err != nil {
		return *a, err
	}
	a.Requester = r
	fmt.Println(c.url, r)
	return *a, nil
}

// newRequester allocates the Requester interface implementing struct selected
// by the scheme of the URL key in Config{}.
func (c *Config) newRequester() (Requester, error) {
	switch {
	case strings.HasPrefix(c.url, "tls://"):
		c.url = "https://" + c.url[6:]
		fallthrough
	case c.url == "raw", c.url == "raw://", c.url == "raw:///":
		c.url = ""
		return newJSONRPC(*c)
	case strings.HasPrefix(c.url, jsonrpcUDSPrefix) == true:
		return newJSONRPC(*c)
	case strings.HasSuffix(c.url, "lsnr.sock"):
		return newJSONRPC(*c)
	case strings.HasPrefix(c.url, jsonrpcInetPrefix):
		return newJSONRPC(*c)
	case strings.HasPrefix(c.url, h2UDSPrefix):
		return newH2UDS(*c)
	case strings.HasSuffix(c.url, "h2.sock"):
		return newH2UDS(*c)
	case strings.HasPrefix(c.url, h2InetPrefix):
		return newH2Inet(*c)
	default:
		c.url = ""
		return newH2UDS(*c)
	}
}

func (c *Config) loadContext() error {
	context, err := NewContext()
	if err != nil {
		return err
	}
	if context.Cluster.Server != "" {
		c.url = context.Cluster.Server
		c.insecureSkipVerify = context.Cluster.InsecureSkipVerify
		c.clientCertificate = context.User.ClientCertificate
		c.clientKey = context.User.ClientKey
	}
	return nil
}
