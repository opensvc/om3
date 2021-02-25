package client

import (
	"net/http"
	"strings"
)

type (
	// API abstracts the requester and exposes the agent API methods
	API struct {
		Requester Requester
	}

	// Config is the agent api client configuration
	Config struct {
		URL                string
		InsecureSkipVerify bool
		ClientCertificate  string
		ClientKey          string
	}

	// Requester abstracts the requesting details of supported protocols
	Requester interface {
		Get(req Request) (*http.Response, error)
	}

	// Request is a api request abstracting the protocol differences
	Request struct {
		Method  string                 `json:"method"`
		Action  string                 `json:"action,omitempty"`
		Node    string                 `json:"node,omitempty"`
		Options map[string]interface{} `json:"options,omitempty"`
	}
)

// NewClientFromConfig allocates a new agent api client struct
func NewClientFromConfig(c Config) (API, error) {
	a := &API{}
	r, err := NewRequester(c)
	if err != nil {
		return *a, err
	}
	a.Requester = r
	return *a, nil
}

// New allocates a new agent api client struct
func New() (API, error) {
	context, err := NewContext()
	if err != nil {
		return API{}, err
	}
	c := &Config{}
	if context.Cluster.Server != "" {
		c.URL = context.Cluster.Server
		c.InsecureSkipVerify = context.Cluster.InsecureSkipVerify
		c.ClientCertificate = context.User.ClientCertificate
		c.ClientKey = context.User.ClientKey
	}
	return NewClientFromConfig(*c)
}

// NewRequest allocates an unconfigured RequestOptions and returns its
// address.
func (a API) NewRequest() *Request {
	r := &Request{}
	r.Options = make(map[string]interface{})
	return r
}

// NewRequester allocates the Requester interface implementing struct selected
// by the scheme of the URL key in Config{}.
func NewRequester(c Config) (Requester, error) {
	if c.URL == "" {
		c.URL = JSONRPCScheme + JSONRPCUDSPath()
		return newJSONRPC(c)
	}
	if strings.HasPrefix(c.URL, H2UDSScheme) {
		return newH2UDS(c)
	}
	if strings.HasPrefix(c.URL, JSONRPCScheme) {
		return newJSONRPC(c)
	}
	return newH2Inet(c)
}
