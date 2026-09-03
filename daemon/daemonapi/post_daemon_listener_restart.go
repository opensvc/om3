package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostDaemonListenerRestart(ctx echo.Context, nodename api.InPathNodeName, name api.InPathListenerName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	localName, err := lifecycleListenerName(name, "restart")
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "%s", err)
	}
	return a.postDaemonSubAction(ctx, nodename, "restart", []string{localName}, func(c *client.T) (*http.Response, error) {
		return c.PostDaemonListenerRestart(ctx.Request().Context(), nodename, name)
	})
}
