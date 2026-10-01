package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/opensvc/om3/v3/core/console"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/nodesinfo"
	oapi "github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/httpclientcache"
)

// TLSConfig returns the tls configuration the api is reached with.
func (t *T) TLSConfig() (*tls.Config, error) {
	return httpclientcache.TLSConfig(httpclientcache.Options{
		CertFile:           t.clientCertificate,
		KeyFile:            t.clientKey,
		InsecureSkipVerify: t.insecureSkipVerify,
		RootCA:             t.rootCA,
	})
}

// Host returns the host the api is reached on, and "" when it is reached
// through the unix socket of the local node.
func (t *T) Host() string {
	if !strings.HasPrefix(t.url, "https://") {
		return ""
	}
	u, err := url.Parse(t.url)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// consoleURL is the url to open the session of the ticket on: the one the
// cluster configures, or the console port of the node the api is reached
// on. A client on the unix socket of a node reaches the console of the node
// the session is for.
func (t *T) consoleURL(nodename string, ticket *oapi.ConsoleTicket) (string, error) {
	var base string
	if ticket.Url != nil && *ticket.Url != "" {
		base = *ticket.Url
	} else {
		host := t.Host()
		if host == "" {
			host = nodename
			if nodesInfo, err := nodesinfo.Load(); err == nil {
				switch addr := nodesInfo[nodename].Lsnr.Addr; addr {
				case "", "::", "0.0.0.0":
				default:
					host = addr
				}
			}
		}
		base = "wss://" + net.JoinHostPort(host, fmt.Sprint(ticket.Port)) + "/"
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("console url %s: %w", base, err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "wss":
	default:
		return "", fmt.Errorf("console url %s: the scheme is not wss", base)
	}
	q := u.Query()
	q.Set("ticket", ticket.Ticket)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Console opens a console session on the resource of the instance, and
// connects stdin and stdout to it until it ends.
//
// The api is asked for the ticket of the session, and the session is opened
// beside the api, on the console endpoint.
func (t *T) Console(ctx context.Context, nodename string, path naming.Path, rid string, stdin *os.File, stdout io.Writer) (console.Result, error) {
	var result console.Result
	params := oapi.PostInstanceResourceConsoleParams{}
	if rid != "" {
		params.Rid = &rid
	}
	resp, err := t.PostInstanceResourceConsoleWithResponse(ctx, nodename, path.Namespace, path.Kind, path.Name, &params)
	if err != nil {
		return result, err
	}
	switch {
	case resp.JSON201 != nil:
	case resp.JSON400 != nil:
		return result, fmt.Errorf("%s: node %s: %s", path, nodename, *resp.JSON400)
	case resp.JSON401 != nil:
		return result, fmt.Errorf("%s: node %s: %s", path, nodename, *resp.JSON401)
	case resp.JSON403 != nil:
		return result, fmt.Errorf("%s: node %s: %s", path, nodename, *resp.JSON403)
	case resp.JSON404 != nil:
		return result, fmt.Errorf("%s: node %s: %s", path, nodename, *resp.JSON404)
	case resp.JSON500 != nil:
		return result, fmt.Errorf("%s: node %s: %s", path, nodename, *resp.JSON500)
	default:
		if resp.StatusCode() == http.StatusCreated {
			return result, fmt.Errorf("%s: node %s: no console ticket in the answer", path, nodename)
		}
		return result, fmt.Errorf("%s: node %s: unexpected status code %d", path, nodename, resp.StatusCode())
	}
	consoleURL, err := t.consoleURL(nodename, resp.JSON201)
	if err != nil {
		return result, err
	}
	tlsConfig, err := t.TLSConfig()
	if err != nil {
		return result, err
	}
	return console.Attach(ctx, console.AttachOptions{
		URL:       consoleURL,
		TLSConfig: tlsConfig,
		Stdin:     stdin,
		Stdout:    stdout,
	})
}
