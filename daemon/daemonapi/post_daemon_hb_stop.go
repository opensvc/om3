package daemonapi

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PostDaemonHeartbeatStop(ctx echo.Context, nodename api.InPathNodeName, name api.InPathHeartbeatName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	return a.postDaemonSubAction(ctx, nodename, "stop", fmt.Sprintf("hb#%s", name), func(c *client.T) (*http.Response, error) {
		return c.PostDaemonHeartbeatStop(ctx.Request().Context(), nodename, name)
	})
}
