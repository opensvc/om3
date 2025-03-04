package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PostDaemonListenerStop(ctx echo.Context, nodename api.InPathNodeName, name api.InPathHeartbeatName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	return a.postDaemonSubAction(ctx, nodename, "stop", fmt.Sprintf("lsnr-%s", name), func(c *client.T) (*http.Response, error) {
		return c.PostDaemonListenerStop(ctx.Request().Context(), nodename, name)
	})
}
