package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PostNodeActionSysreport(ctx echo.Context, nodename string, params api.PostNodeActionSysreportParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionSysreport(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeActionSysreport(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) localNodeActionSysreport(ctx echo.Context, params api.PostNodeActionSysreportParams) error {
	log := LogHandler(ctx, "PostNodeActionSysreport")
	var requesterSid uuid.UUID
	args := []string{"node", "sysreport"}
	if params.Force != nil && *params.Force {
		args = append(args, "--force")
	}
	if params.RequesterSid != nil {
		requesterSid = *params.RequesterSid
	}
	if sid, err := a.apiExec(ctx, naming.Path{}, requesterSid, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.NodeActionAccepted{SessionID: sid})
	}
}
