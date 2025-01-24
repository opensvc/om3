package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodePing(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if a.localhost == nodename {
		return a.getLocalNodePing(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodePing(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodePing(ctx echo.Context) error {
	return ctx.NoContent(http.StatusNoContent)
}
