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
	}

	// Requester abstracts the requesting details of supported protocols
	Requester interface {
		Get(req string) (*http.Response, error)
	}
)

// New allocates a new agent api client struct
func New(c Config) API {
	return API{
		Requester: NewRequester(c),
	}
}

// NewRequester allocates the Requester interface implementing struct selected
// by the scheme of the URL key in Config{}.
func NewRequester(c Config) Requester {
	if c.URL == "" {
		//c.URL = "https://127.0.0.1:1215"
		c.URL = JSONRPCScheme + JSONRPCUDSPath
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
