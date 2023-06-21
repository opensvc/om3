package client

import (
	"fmt"
	"strings"
	"time"

	"github.com/opensvc/om3/core/client/api"
	reqh2 "github.com/opensvc/om3/core/client/requester/h2"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/rawconfig"
	oapi "github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/daemonenv"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/hostname"
)

type (
	// T is the agent api client configuration
	T struct {
		*oapi.ClientWithResponses
		url                string
		insecureSkipVerify bool
		clientCertificate  string
		clientKey          string
		username           string
		password           string
		bearer             string
		rootCA             string
		timeout            time.Duration
	}
)

// New allocates a new client configuration and returns the reference
// so users are not tempted to use client.Config{} dereferenced, which would
// make loadContext useless.
func New(opts ...funcopt.O) (*T, error) {
	t := &T{
		timeout: 5 * time.Second,
	}
	if err := funcopt.Apply(t, opts...); err != nil {
		return nil, err
	}
	if err := t.configure(); err != nil {
		return nil, err
	}
	return t, nil
}

// WithURL is the option pointing the api location and protocol using the
// [<scheme>://]<addr>[:<port>] format.
//
// Supported schemes:
//
//   - raw
//     json rpc, AES-256-CBC encrypted payload if transported by AF_INET,
//     cleartext on unix domain socket.
//   - https
//     http/2 with TLS
//   - tls
//     http/2 with TLS
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
func WithURL(url string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.url = url
		return nil
	})
}

// WithTimeout set a timeout on the connection
func WithTimeout(v time.Duration) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.timeout = v
		return nil
	})
}

// WithInsecureSkipVerify skips certificate validity checks.
func WithInsecureSkipVerify(v bool) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.insecureSkipVerify = v
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

// WithRootCa sets the client RootCA filename, httpclient cache don't cache
// clients with RootCa because of possible tmp filename conflict signature.
// The cert from s file is appended to x509.SystemCertPool
func WithRootCa(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.rootCA = s
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

// WithBearer sets the client bearer token to use for newRequests
func WithBearer(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.bearer = s
		return nil
	})
}

// WithUsername sets the username to use for login.
func WithUsername(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.username = s
		return nil
	})
}

// WithPassword sets the password to use for login.
func WithPassword(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.password = s
		return nil
	})
}

func (t *T) URL() string {
	return t.url
}

// configure allocates a new requester with a requester for the server found in Config,
// or for the server found in Context.
func (t *T) configure() error {
	if env.Context() != "" {
		if err := t.loadContext(); err != nil {
			return err
		}
	} else if t.url == "" {
		t.url = daemonenv.UrlUxHttp()
	} else if t.bearer == "" && t.username == "" {
		t.username = hostname.Hostname()
		t.password = rawconfig.ClusterSection().Secret
	}

	err := t.newRequester()
	if err != nil {
		return err
	}
	return nil
}

// newRequester allocates the Requester interface implementing struct selected
// by the scheme of the URL key in Config{}.
func (t *T) newRequester() (err error) {
	if strings.HasPrefix(t.url, "tls://") {
		t.url = "https://" + t.url[6:]
	}
	switch {
	case t.url == "":
	case strings.HasPrefix(t.url, reqh2.UDSPrefix):
		t.url = t.url[7:]
		t.ClientWithResponses, err = reqh2.NewUDS(reqh2.Config{
			URL:     t.url,
			Timeout: t.timeout,
		})
	case strings.HasSuffix(t.url, "h2.sock"):
		t.ClientWithResponses, err = reqh2.NewUDS(reqh2.Config{
			URL:     t.url,
			Timeout: t.timeout,
		})
	case strings.HasPrefix(t.url, reqh2.InetPrefix):
		t.ClientWithResponses, err = reqh2.NewInet(reqh2.Config{
			URL:                t.url,
			Certificate:        t.clientCertificate,
			Key:                t.clientKey,
			InsecureSkipVerify: t.insecureSkipVerify,
			Username:           t.username,
			Password:           t.password,
			Bearer:             t.bearer,
			RootCA:             t.rootCA,
			Timeout:            t.timeout,
		})
	default:
		if !strings.Contains(t.url, ":") {
			t.url += ":" + fmt.Sprint(daemonenv.HttpPort)
		}
		t.url = reqh2.InetPrefix + t.url
		t.ClientWithResponses, err = reqh2.NewInet(reqh2.Config{
			URL:                t.url,
			Certificate:        t.clientCertificate,
			Key:                t.clientKey,
			InsecureSkipVerify: t.insecureSkipVerify,
			Username:           t.username,
			Password:           t.password,
			Bearer:             t.bearer,
			RootCA:             t.rootCA,
			Timeout:            t.timeout,
		})
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

func (t *T) NewGetEvents() *api.GetEvents {
	return api.NewGetEvents(t)
}

func (t *T) NewGetLogs() *api.GetLogs {
	return api.NewGetLogs(t)
}

func (t *T) NewGetDaemonStatus() *api.GetDaemonStatus {
	return api.NewGetDaemonStatus(t)
}
