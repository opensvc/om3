package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostDaemonHeartbeatStop(ctx echo.Context, nodename api.InPathNodeName, name api.InPathHeartbeatName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	localName, err := heartbeatStreamName(name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "%s", err)
	}
	return a.postDaemonSubAction(ctx, nodename, "stop", localName, func(c *client.T) (*http.Response, error) {
		return c.PostDaemonHeartbeatStop(ctx.Request().Context(), nodename, name)
	})
}
