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

	// Option is a functional option configurer.
	// https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
	Option interface {
		apply(t *T) error
	}

	optionFunc func(*T) error
)

func (fn optionFunc) apply(t *T) error {
	return fn(t)
}

// New allocates a new client configuration and returns the reference
// so users are not tempted to use client.Config{} dereferenced, which would
// make loadContext useless.
func New(opts ...Option) (*T, error) {
	t := &T{}
	for _, opt := range opts {
		if err := opt.apply(t); err != nil {
			return nil, err
		}
	}
	if err := t.Configure(); err != nil {
		return nil, err
	}
	return t, nil
}

//
// URL is the option pointing the api location and protocol using the
// [<scheme>://]<addr>[:<port>] format.
//
// Supported schemes:
// * raw
//   json rpc, AES-256-CBC encrypted payload if transported by AF_INET,
//   cleartext on unix domain socket.
// * https
//   http/2 with TLS
// * tls
//   http/2 with TLS
//
// If unset, a unix domain socket connection and the http/2 protocol is
// selected.
//
// If URL is a unix domain socket path, use the corresponding protocol.
//
// If scheme is omitted, select the http/2 protocol.
//
// Examples:
// * /opt/opensvc/var/lsnr/lsnr.sock
// * /opt/opensvc/var/lsnr/h2.sock
// * https://acme.com:1215
// * raw://acme.com:1214
//
func URL(url string) Option {
	return optionFunc(func(t *T) error {
		t.url = url
		return nil
	})
}

// InsecureSkipVerify skips certificate validity checks.
func InsecureSkipVerify() Option {
	return optionFunc(func(t *T) error {
		t.insecureSkipVerify = true
		return nil
	})
}

// Certificate sets the x509 client certificate.
func Certificate(s string) Option {
	return optionFunc(func(t *T) error {
		t.clientCertificate = s
		return nil
	})
}

// Key sets the x509 client private key..
func Key(s string) Option {
	return optionFunc(func(t *T) error {
		t.clientKey = s
		return nil
	})
}

// Configure allocates a new requester with a requester for the server found in Config,
// or for the server found in Context.
func (t *T) Configure() error {
	if t.url == "" {
		if err := t.loadContext(); err != nil {
			return err
		}
	}
	err := t.newRequester()
	if err != nil {
		return err
	}
	log.Debug().Msgf("connected %s", t.requester)
	return nil
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
