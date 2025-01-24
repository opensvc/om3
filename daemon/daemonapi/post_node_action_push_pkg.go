package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PostNodeActionPushPkg(ctx echo.Context, nodename string, params api.PostNodeActionPushPkgParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if nodename == a.localhost {
		return a.localNodeActionPushPkg(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeActionPushPkg(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) localNodeActionPushPkg(ctx echo.Context, params api.PostNodeActionPushPkgParams) error {
	log := LogHandler(ctx, "PostNodeActionPushPkg")
	var requesterSid uuid.UUID
	args := []string{"node", "push", "pkg", "--local"}
	if params.RequesterSid != nil {
		requesterSid = *params.RequesterSid
	}
	if sid, err := a.apiExec(ctx, naming.Path{}, requesterSid, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.NodeActionAccepted{SessionID: sid})
	}
}
