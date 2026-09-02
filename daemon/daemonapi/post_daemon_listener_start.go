package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostDaemonListenerStart(ctx echo.Context, nodename api.InPathNodeName, name api.InPathListenerName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	// parseNodename here too: start was the one action of the three not
	// normalizing the nodename it was given.
	nodename = a.parseNodename(nodename)
	localName, err := lifecycleListenerName(name, "start")
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "%s", err)
	}
	return a.postDaemonSubAction(ctx, nodename, "start", []string{localName}, func(c *client.T) (*http.Response, error) {
		return c.PostDaemonListenerStart(ctx.Request().Context(), nodename, name)
	})
}
