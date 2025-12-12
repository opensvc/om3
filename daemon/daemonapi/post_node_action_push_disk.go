package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostNodeActionPushDisk(ctx echo.Context, nodename string, params api.PostNodeActionPushDiskParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionPushDisk(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeActionPushDisk(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) localNodeActionPushDisk(ctx echo.Context, params api.PostNodeActionPushDiskParams) error {
	log := LogHandler(ctx, "PostNodeActionPushDisk")
	var requesterSID uuid.UUID
	args := []string{"node", "push", "disk"}
	if params.RequesterSid != nil {
		requesterSID = *params.RequesterSid
	}
	if sid, err := a.apiExec(ctx, naming.Path{}, requesterSID, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.NodeActionAccepted{SessionID: sid})
	}
}
