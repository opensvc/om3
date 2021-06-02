package client

import (
	"strings"

	"github.com/rs/zerolog/log"
	"opensvc.com/opensvc/core/client/api"
	reqh2 "opensvc.com/opensvc/core/client/requester/h2"
	reqjsonrpc "opensvc.com/opensvc/core/client/requester/jsonrpc"
	"opensvc.com/opensvc/core/clientcontext"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	// T is the agent api client configuration
	T struct {
		url                string
		insecureSkipVerify bool
		clientCertificate  string
		clientKey          string
		requester          api.Requester
	}
)

//
// New allocates a new client configuration and returns the reference
// so users are not tempted to use client.Config{} dereferenced, which would
// make loadContext useless.
//
func New(opts ...funcopt.O) (*T, error) {
	t := &T{}
	if err := funcopt.Apply(t, opts...); err != nil {
		return nil, err
	}
	if err := t.configure(); err != nil {
		return nil, err
	}
	return t, nil
}

//
// WithURL is the option pointing the api location and protocol using the
// [<scheme>://]<addr>[:<port>] format.
//
// Supported schemes:
//
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
// If WithURL is a unix domain socket path, use the corresponding protocol.
//
// If scheme is omitted, select the http/2 protocol.
//
// Examples:
// * /opt/opensvc/var/lsnr/lsnr.sock
// * /opt/opensvc/var/lsnr/h2.sock
// * https://acme.com:1215
// * raw://acme.com:1214
//
func WithURL(url string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.url = url
		return nil
	})
}

// WithInsecureSkipVerify skips certificate validity checks.
func WithInsecureSkipVerify() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.insecureSkipVerify = true
		return nil
	})
}

// WithCertificate sets the x509 client certificate.
func WithCertificate(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.clientCertificate = s
		return nil
	})
}

// WithKey sets the x509 client private key..
func WithKey(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.clientKey = s
		return nil
	})
}

// configure allocates a new requester with a requester for the server found in Config,
// or for the server found in Context.
func (t *T) configure() error {
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
func (t *T) newRequester() (err error) {
	if strings.HasPrefix(t.url, "tls://") {
		t.url = "https://" + t.url[6:]
	}
	switch {
	case t.url == "raw", t.url == "raw://", t.url == "raw:///":
		t.url = ""
		t.requester, err = reqjsonrpc.New(t.url)
	case strings.HasPrefix(t.url, reqjsonrpc.UDSPrefix) == true:
		t.requester, err = reqjsonrpc.New(t.url)
	case strings.HasSuffix(t.url, "lsnr.sock"):
		t.requester, err = reqjsonrpc.New(t.url)
	case strings.HasPrefix(t.url, reqjsonrpc.InetPrefix):
		t.requester, err = reqjsonrpc.New(t.url)
	case strings.HasPrefix(t.url, reqh2.UDSPrefix):
		t.requester, err = reqh2.NewUDS(t.url)
	case strings.HasSuffix(t.url, "h2.sock"):
		t.requester, err = reqh2.NewUDS(t.url)
	case strings.HasPrefix(t.url, reqh2.InetPrefix):
		t.requester, err = reqh2.NewInet(t.url, t.clientCertificate, t.clientKey, t.insecureSkipVerify)
	default:
		t.url = ""
		t.requester, err = reqh2.NewUDS(t.url)
	}
	return err
}

func (t *T) loadContext() error {
	context, err := clientcontext.New()
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
