package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostNodeActionReboot(ctx echo.Context, nodename string, params api.PostNodeActionRebootParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionReboot(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeActionReboot(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) localNodeActionReboot(ctx echo.Context, params api.PostNodeActionRebootParams) error {
	log := LogHandler(ctx, "PostNodeActionReboot")
	var requesterSessionID uuid.UUID
	args := []string{"node", "reboot"}
	if params.SessionID != nil {
		requesterSessionID = *params.SessionID
	}
	if sessionID, execID, err := a.apiExec(ctx, naming.Path{}, requesterSessionID, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.NodeActionAccepted{SessionID: sessionID, ExecID: execID})
	}
}
