package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostPeerActionDequeue(ctx echo.Context, nodename string, params api.PostPeerActionDequeueParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionDequeue(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostPeerActionDequeue(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) localNodeActionDequeue(ctx echo.Context, params api.PostPeerActionDequeueParams) error {
	log := LogHandler(ctx, "PostPeerActionDequeue")
	var requesterSessionID uuid.UUID
	args := []string{"node", "dequeue"}
	if params.SessionID != nil {
		requesterSessionID = *params.SessionID
	}
	if sessionID, execID, err := a.apiExec(ctx, naming.Path{}, requesterSessionID, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.NodeActionAccepted{SessionID: sessionID, ExecID: execID})
	}
}
