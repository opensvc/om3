package client

import (
	"strings"

	"github.com/rs/zerolog/log"
)

type (
	// T is the agent api client configuration
	T struct {
		url                string
		insecureSkipVerify bool
		clientCertificate  string
		clientKey          string
		requester          Requester
	}
)

// New allocates a new client configuration and returns the reference
// so users are not tempted to use client.Config{} dereferenced, which would
// make loadContext useless.
func New() *T {
	return &T{}
}

// SetURL sets the config key.
func (t *T) SetURL(url string) *T {
	t.url = url
	return t
}

// SetInsecureSkipVerify sets the config key.
func (t *T) SetInsecureSkipVerify(b bool) *T {
	t.insecureSkipVerify = b
	return t
}

// SetClientCertificate sets the config key.
func (t *T) SetClientCertificate(s string) *T {
	t.clientCertificate = s
	return t
}

// SetClientKey sets the config key.
func (t *T) SetClientKey(s string) *T {
	t.clientKey = s
	return t
}

// Configure allocates a new requester with a requester for the server found in Config,
// or for the server found in Context.
func (t *T) Configure() (*T, error) {
	if t.url == "" {
		if err := t.loadContext(); err != nil {
			return t, err
		}
	}
	err := t.newRequester()
	if err != nil {
		return t, err
	}
	log.Debug().Msgf("connected %s", t.requester)
	return t, nil
}

// newRequester allocates the Requester interface implementing struct selected
// by the scheme of the URL key in Config{}.
func (t *T) newRequester() error {
	if strings.HasPrefix(t.url, "tls://") {
		t.url = "https://" + t.url[6:]
	}
	switch {
	case t.url == "raw", t.url == "raw://", t.url == "raw:///":
		t.url = ""
		return t.configureJSONRPC()
	case strings.HasPrefix(t.url, jsonrpcUDSPrefix) == true:
		return t.configureJSONRPC()
	case strings.HasSuffix(t.url, "lsnr.sock"):
		return t.configureJSONRPC()
	case strings.HasPrefix(t.url, jsonrpcInetPrefix):
		return t.configureJSONRPC()
	case strings.HasPrefix(t.url, h2UDSPrefix):
		return t.configureH2UDS()
	case strings.HasSuffix(t.url, "h2.sock"):
		return t.configureH2UDS()
	case strings.HasPrefix(t.url, h2InetPrefix):
		return t.configureH2Inet()
	default:
		t.url = ""
		return t.configureH2UDS()
	}
}

func (t *T) loadContext() error {
	context, err := NewContext()
	if err != nil {
		return err
	}
	if context.Cluster.Server != "" {
		t.url = context.Cluster.Server
		t.insecureSkipVerify = context.Cluster.InsecureSkipVerify
		t.clientCertificate = context.User.ClientCertificate
		t.clientKey = context.User.ClientKey
	}
	return nil
}
