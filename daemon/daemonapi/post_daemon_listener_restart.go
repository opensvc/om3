package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PostDaemonListenerRestart(ctx echo.Context, nodename api.InPathNodeName, name api.InPathListenerName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	return a.postDaemonSubAction(ctx, nodename, "restart", fmt.Sprintf("lsnr-%s", name), func(c *client.T) (*http.Response, error) {
		return c.PostDaemonListenerRestart(ctx.Request().Context(), nodename, name)
	})
}
