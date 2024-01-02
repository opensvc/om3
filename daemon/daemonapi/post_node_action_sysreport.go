package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonApi) PostNodeActionSysreport(ctx echo.Context, nodename string, params api.PostNodeActionSysreportParams) error {
	if nodename == a.localhost {
		return a.localNodeActionSysreport(ctx, params)
	} else {
		return a.remoteNodeActionSysreport(ctx, nodename, params)
	}
	return nil
}

func (a *DaemonApi) remoteNodeActionSysreport(ctx echo.Context, nodename string, params api.PostNodeActionSysreportParams) error {
	c, err := client.New(client.WithURL(nodename))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	resp, err := c.PostNodeActionSysreportWithResponse(ctx.Request().Context(), nodename, &params)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonApi) localNodeActionSysreport(ctx echo.Context, params api.PostNodeActionSysreportParams) error {
	if v, err := assertGrant(ctx, rbac.GrantRoot); !v {
		return err
	}
	log := LogHandler(ctx, "PostNodeActionSysreport")
	var requesterSid uuid.UUID
	args := []string{"node", "sysreport", "--local"}
	if params.Force != nil && *params.Force {
		args = append(args, "--force")
	}
	if params.RequesterSid != nil {
		requesterSid = *params.RequesterSid
	}
	if sid, err := a.apiExec(ctx, naming.Path{}, requesterSid, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.NodeActionAccepted{SessionId: sid})
	}
}
