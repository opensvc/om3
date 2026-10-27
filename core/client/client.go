package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/v3/core/client/api"
	reqh2 "github.com/opensvc/om3/v3/core/client/requester/h2"
	"github.com/opensvc/om3/v3/core/client/tokencache"
	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodesinfo"
	oapi "github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/daemonenv"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/homedir"
	"github.com/opensvc/om3/v3/util/hostname"
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
		bearerFunc         func() (string, error)
		rootCA             string
		serverName         string
		timeout            time.Duration
		tokens             tokencache.Entry
	}
)

var (
	// DefaultClientTimeout is the default client timeout value used by New
	DefaultClientTimeout = 30 * time.Second
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

// WithServerName sets the name the server certificate is verified for, when
// it is not the host of the url, as when a peer node is dialed by the address
// of its listener.
func WithServerName(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.serverName = s
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

// WithBearer sets the client bearer tokens to use for newRequests
func WithBearer(s string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.bearer = s
		return nil
	})
}

// WithBearerFunc sets the function returning the bearer token of each
// request, called as the request is sent: a token valid a few seconds, as
// the node token, is not expired by a request waiting to be sent.
func WithBearerFunc(fn func() (string, error)) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.bearerFunc = fn
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

// userSocketURL returns the url of the user socket, or why the caller can
// not reach it.
func userSocketURL() (string, error) {
	p := daemonenv.HTTPUserUnixFile()
	_, err := os.Stat(p)
	if err == nil {
		err = unix.Access(p, unix.R_OK|unix.W_OK)
	}
	switch {
	case err == nil:
		return daemonenv.HTTPUserUnixURL(), nil
	case errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("%s: no user socket: the daemon is not running, or serves none, listener.ux_owner_group being empty or naming a group the node does not know; set a context to reach a cluster remotely", p)
	case errors.Is(err, os.ErrPermission):
		return "", fmt.Errorf("%s: permission denied: the user socket is reached by the members of the group %s", p, ownerGroup(filepath.Dir(p)))
	default:
		return "", err
	}
}

// ownerGroup returns the name of the group owning the file p, or its id.
func ownerGroup(p string) string {
	info, err := os.Stat(p)
	if err != nil {
		return "listener.ux_owner_group names"
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "listener.ux_owner_group names"
	}
	gid := strconv.FormatUint(uint64(st.Gid), 10)
	if g, err := user.LookupGroupId(gid); err == nil {
		return g.Name
	}
	return gid
}

// configure allocates a new requester with a requester for the server found in Config,
// or for the server found in Context.
func (t *T) configure() error {
	if env.Context() != "" {
		if err := t.loadContext(); err != nil {
			return err
		}
	} else {
		if t.url == "" && clientcontext.UsesUserSocket() {
			u, err := userSocketURL()
			if err != nil {
				return err
			}
			t.url = u
		}
		if t.url == "" {
			t.url = daemonenv.HTTPUnixURL()
		}
		// A client on a cluster node, the daemon among them, reaches
		// the nodes of its cluster, whose certificates the cluster
		// authorities sign.
		if t.rootCA == "" && file.Exists(daemonenv.CAsCertFile()) {
			t.rootCA = daemonenv.CAsCertFile()
		}
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
			BearerFunc:         t.bearerFunc,
			RootCA:             t.rootCA,
			ServerName:         t.serverName,
			Timeout:            t.timeout,
			Tokens:             t.tokens,
		})
	default:
		if !strings.Contains(t.url, ":") {
			// The url is a node name, and the node may be dialed by
			// the address of its listener: its certificate names it.
			if t.serverName == "" {
				t.serverName = t.url
			}
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
			BearerFunc:         t.bearerFunc,
			RootCA:             t.rootCA,
			ServerName:         t.serverName,
			Timeout:            t.timeout,
			Tokens:             t.tokens,
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
		if context.Cluster.InsecureSkipVerify != nil {
			t.insecureSkipVerify = *context.Cluster.InsecureSkipVerify
		}
		// The authority of a cluster whose certificate no system one
		// signs, as one its own ca issued. An insecure context verifies
		// nothing, and does not read it: a file gone is no failure.
		if ca := context.Cluster.CertificateAuthority; ca != nil && *ca != "" && t.rootCA == "" && !t.insecureSkipVerify {
			if t.rootCA, err = homedir.Expand(*ca); err != nil {
				return fmt.Errorf("context certificate authority %s: %w", *ca, err)
			}
		}
		if context.User.ClientCertificate != nil {
			t.clientCertificate = *context.User.ClientCertificate
		}
		if context.User.ClientKey != nil {
			t.clientKey = *context.User.ClientKey
		}
		if context.User.Name != nil {
			t.username = *context.User.Name
		}

		// A bearer the caller gives, or signs as the requests go out, is
		// the credential: the tokens of the context are not loaded.
		if t.bearer == "" && t.bearerFunc == nil {
			tok, err := tokencache.Load(env.Context())
			if err != nil && (t.username == "" || t.password == "") {
				return tokencache.ReconnectError(err, env.Context())
			}
			if tok != nil && tok.AccessToken != "" {
				t.tokens = *tok
				t.bearer = tok.AccessToken
			}
		}
	}
	return nil
}

func (t *T) NewGetEvents() *api.GetEvents {
	return api.NewGetEvents(t)
}

func (t *T) NewGetLogs(nodename string) *api.GetLogs {
	return api.NewGetLogs(t, nodename)
}

// GetInstanceContainerLog is a placeholder for the generated API type
// This will be replaced by the actual generated type from the API spec
type GetInstanceContainerLog struct {
	client   *T
	nodename string
	path     naming.Path
	rid      string
}

func (t *T) NewGetContainerLogs(path naming.Path, nodename, rid string) *GetInstanceContainerLog {
	return &GetInstanceContainerLog{
		client:   t,
		nodename: nodename,
		rid:      rid,
		path:     path,
	}
}

func (t *GetInstanceContainerLog) Logs(ctx context.Context, follow bool, lines int) (<-chan []byte, error) {
	// Use the generated API client
	params := &oapi.GetInstanceContainerLogParams{}
	if t.rid != "" {
		params.Rid = &t.rid
	}
	if follow {
		params.Follow = &follow
	}
	if lines > 0 {
		logLines := oapi.LogLines(lines)
		params.Lines = &logLines
	}

	resp, err := t.client.GetInstanceContainerLog(ctx, t.nodename, t.path.Namespace, t.path.Kind, t.path.Name, params)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Create log channel and stream response
	logChan := make(chan []byte)

	go func() {
		defer resp.Body.Close()
		defer close(logChan)

		// Stream the response body
		buf := make([]byte, 4096)
		for {
			n, err := resp.Body.Read(buf)
			if err != nil {
				if err == io.EOF {
					return
				}
				// Log read error but don't fail the channel
				return
			}
			if n > 0 {
				// Copy data to avoid race conditions
				data := make([]byte, n)
				copy(data, buf[:n])
				logChan <- data
			}
		}
	}()

	return logChan, nil
}

func (t *T) NewGetClusterStatus() *api.GetClusterStatus {
	return api.NewGetClusterStatus(t)
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
