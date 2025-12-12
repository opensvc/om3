package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostPeerActionFreeze(ctx echo.Context, nodename string, params api.PostPeerActionFreezeParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionFreeze(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostPeerActionFreeze(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) localNodeActionFreeze(ctx echo.Context, params api.PostPeerActionFreezeParams) error {
	log := LogHandler(ctx, "PostPeerActionFreeze")
	var requesterSid uuid.UUID
	args := []string{"node", "freeze"}
	if params.RequesterSid != nil {
		requesterSid = *params.RequesterSid
	}
	if sid, err := a.apiExec(ctx, naming.Path{}, requesterSid, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.NodeActionAccepted{SessionID: sid})
	}
}
