package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostNodeActionShutdown(ctx echo.Context, nodename string, params api.PostNodeActionShutdownParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionShutdown(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeActionShutdown(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) localNodeActionShutdown(ctx echo.Context, params api.PostNodeActionShutdownParams) error {
	log := LogHandler(ctx, "PostNodeActionShutdown")
	var requesterSessionID uuid.UUID
	args := []string{"node", "shutdown"}
	if params.SessionID != nil {
		requesterSessionID = *params.SessionID
	}
	if sessionID, execID, err := a.apiExec(ctx, naming.Path{}, requesterSessionID, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.NodeActionAccepted{SessionID: sessionID, ExecID: execID})
	}
}
