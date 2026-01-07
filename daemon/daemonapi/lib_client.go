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
	if data := node.StatusData.GetByNode(nodename); data == nil {
		return JSONProblemf(ctx, http.StatusNotFound, "node status data not found", "%s", nodename)
	}
	GetLogger(ctx).Tracef("create proxy client for %s", nodename)
	c, err := a.newProxyClient(ctx, nodename)
	if err != nil {
		GetLogger(ctx).Errorf("create proxy client for %s: %s", nodename, err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	if resp, err := fn(c); err != nil {
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
	}
	tkDuration := 5 * time.Second
	authHeader := ctx.Request().Header.Get("authorization")
	if authHeader != "" {
		options = append(options, client.WithAuthorization(authHeader))
	} else {
		strategy := strategyFromContext(ctx)
		switch strategy {
		case daemonauth.StrategyUX, daemonauth.StrategyX509:
			username := userFromContext(ctx).GetUserName()
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
