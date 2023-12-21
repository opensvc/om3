package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
)

func (a *DaemonApi) PostPeerActionAbort(ctx echo.Context, nodename string) error {
	if nodename == a.localhost {
		return a.localNodeActionAbort(ctx)
	}

	c, err := client.New(client.WithURL(nodename))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}

	resp, err := c.PostPeerActionAbortWithResponse(ctx.Request().Context(), nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonApi) localNodeActionAbort(ctx echo.Context) error {
	v := node.MonitorLocalExpectNone
	a.EventBus.Pub(&msgbus.SetNodeMonitor{Node: a.localhost, Value: node.MonitorUpdate{LocalExpect: &v}},
		labelApi)
	return ctx.JSON(http.StatusOK, nil)
}
