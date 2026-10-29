package daemonapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/daemonauth"
	"github.com/opensvc/om3/v3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/v3/util/funcopt"
)

func (a *DaemonAPI) proxy(ctx echo.Context, nodename string, fn func(*client.T) (*http.Response, error)) error {
	return a.proxyOr(ctx, nodename, fn, nil)
}

// proxyWaiting is proxy for a request the peer holds for wait, when set: the
// peer answers once the wait ends, later than a client allows an answer by
// default, and an orchestration or an exec waited for longer than that was
// answered a 500 while it ran on. The request of the caller bounds it instead.
func (a *DaemonAPI) proxyWaiting(ctx echo.Context, nodename string, wait *string, fn func(*client.T) (*http.Response, error)) error {
	var opts []funcopt.O
	if wait != nil && *wait != "" {
		opts = append(opts, client.WithTimeout(0))
	}
	return a.proxyWith(ctx, nodename, fn, nil, opts...)
}

// proxyOr is proxy, with onErr answering a request the peer did not answer,
// in place of the internal error proxy answers.
func (a *DaemonAPI) proxyOr(ctx echo.Context, nodename string, fn func(*client.T) (*http.Response, error), onErr func(error) error) error {
	return a.proxyWith(ctx, nodename, fn, onErr)
}

// proxyWith is proxyOr, with options of the client of the peer.
func (a *DaemonAPI) proxyWith(ctx echo.Context, nodename string, fn func(*client.T) (*http.Response, error), onErr func(error) error, opts ...funcopt.O) error {
	if data := node.StatusData.GetByNode(nodename); data == nil {
		return JSONProblemf(ctx, http.StatusNotFound, "node status data not found", "%s", nodename)
	}
	GetLogger(ctx).Tracef("create proxy client for %s", nodename)
	c, err := a.newProxyClient(ctx, nodename, opts...)
	if err != nil {
		GetLogger(ctx).Errorf("create proxy client for %s: %s", nodename, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	if resp, err := fn(c); err != nil && onErr != nil {
		return onErr(err)
	} else if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else {
		for key, values := range resp.Header {
			for _, v := range values {
				ctx.Response().Header().Add(key, v)
			}
		}
		return ctx.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
	}
}

func (a *DaemonAPI) newProxyClient(ctx echo.Context, nodename string, opts ...funcopt.O) (*client.T, error) {
	options := []funcopt.O{
		client.WithURL(daemonsubsystem.PeerURL(nodename)),
		client.WithServerName(nodename),
	}
	tkDuration := 5 * time.Second
	authHeader := ctx.Request().Header.Get("authorization")
	if authHeader != "" {
		options = append(options, client.WithAuthorization(authHeader))
	} else {
		strategy := strategyFromContext(ctx)
		switch strategy {
		case daemonauth.StrategyUX, daemonauth.StrategyUXUser, daemonauth.StrategyX509:
			username := userFromContext(ctx).Username
			grantL := grantsFromContext(ctx).AsStringList()
			GetLogger(ctx).Tracef("create proxy client token for %s@%s with grants %s", username, nodename, grantL)
			tk, err := a.createAccessTokenWithGrants(username, tkDuration, daemonauth.TkUseProxy, grantL)
			if err != nil {
				return nil, fmt.Errorf("proxy abort: can't create token for %s with grants %s: %w", username, grantL, err)
			}
			options = append(options, client.WithBearer(tk.AccessToken))
		}
	}
	options = append(options, opts...)
	return client.New(options...)
}
