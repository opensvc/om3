package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/drivercatalog"
	"github.com/opensvc/om3/v3/daemon/api"
)

// GetNodeDriver lists the drivers of a node.
//
// Any user may read it: it says what the binary ships, as the documentation
// does, and nothing of the node. The capabilities do, and stay root's.
func (a *DaemonAPI) GetNodeDriver(ctx echo.Context, nodename api.InPathNodeName) error {
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.getLocalNodeDriver(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeDriver(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodeDriver(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, drivercatalog.APIList(a.localhost))
}
