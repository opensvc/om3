package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
)

func (a *DaemonAPI) PostNodeActionClear(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionClear(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeActionClear(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) localNodeActionClear(ctx echo.Context) error {
	state := node.MonitorStateIdle
	a.Publisher.Pub(&msgbus.SetNodeMonitor{Node: a.localhost, Value: node.MonitorUpdate{State: &state}},
		labelOriginAPI)
	return ctx.JSON(http.StatusOK, nil)
}
