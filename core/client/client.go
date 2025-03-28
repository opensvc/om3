package client

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/opensvc/om3/core/client/api"
	reqh2 "github.com/opensvc/om3/core/client/requester/h2"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/core/nodesinfo"
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
		authorization      string
		bearer             string
		rootCA             string
		timeout            time.Duration
	}
)

var (
	// DefaultClientTimeout is the default client timeout value used by New
	DefaultClientTimeout = 5 * time.Second
)

// New allocates a new client configuration and returns the reference
// so users are not tempted to use client.Config{} dereferenced, which would
// make loadContext useless.
func New(opts ...funcopt.O) (*T, error) {
	t := &T{
		timeout: DefaultClientTimeout,
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
// * /var/lib/opensvc/lsnr/http.sock
// * https://acme.com:1215
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

// WithAuthorization sets the client authorization to use for newRequests
func WithAuthorization(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.authorization = s
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
		t.url = daemonenv.HTTPUnixURL()
	} else if t.bearer == "" && t.username == "" && t.authorization == "" {
		// TODO: need refactor or remove, this may send credential to unexpected url
		t.username = hostname.Hostname()
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
	case strings.HasSuffix(t.url, daemonenv.HTTPUnixFileBasename):
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
			Authorization:      t.authorization,
			Password:           t.password,
			Bearer:             t.bearer,
			RootCA:             t.rootCA,
			Timeout:            t.timeout,
		})
	default:
		if !strings.Contains(t.url, ":") {
			if nodesInfo, err := nodesinfo.Load(); err == nil {
				addr := nodesInfo[t.url].Lsnr.Addr
				if addr == "::" || addr == "" || addr == "0.0.0.0" {
					addr = t.url
				}
				port := nodesInfo[t.url].Lsnr.Port
				if port == "" {
					port = fmt.Sprint(daemonenv.HTTPPort)
				}
				t.url = fmt.Sprintf("https://%s:%s", addr, port)
			} else {
				t.url = fmt.Sprintf("https://%s:%d", t.url, daemonenv.HTTPPort)
			}
		} else {
			t.url = reqh2.InetPrefix + t.url
		}
		t.ClientWithResponses, err = reqh2.NewInet(reqh2.Config{
			URL:                t.url,
			Certificate:        t.clientCertificate,
			Key:                t.clientKey,
			InsecureSkipVerify: t.insecureSkipVerify,
			Authorization:      t.authorization,
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
		t.password = context.User.Password
		t.username = context.User.Name
	}
	return nil
}

func (t *T) NewGetEvents() *api.GetEvents {
	return api.NewGetEvents(t)
}

func (t *T) NewGetLogs(nodename string) *api.GetLogs {
	return api.NewGetLogs(t, nodename)
}

func (t *T) NewGetDaemonStatus() *api.GetDaemonStatus {
	return api.NewGetDaemonStatus(t)
}

func (t *T) Hostname() string {
	if u, err := url.Parse(t.url); err == nil {
		return u.Hostname()
	} else if strings.HasPrefix(t.url, "/") {
		return hostname.Hostname()
	} else {
		return ""
	}
}
