package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetDaemonRunning(ctx echo.Context, nodename api.InPathNodeName) error {
	if a.localhost == nodename {
		return a.getLocalDaemonRunning(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetDaemonRunning(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalDaemonRunning(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, true)
}
