package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostNodeActionSCSIScan(ctx echo.Context, nodename string, params api.PostNodeActionSCSIScanParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionSCSIScan(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeActionSCSIScan(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) localNodeActionSCSIScan(ctx echo.Context, params api.PostNodeActionSCSIScanParams) error {
	log := LogHandler(ctx, "PostNodeActionSCSIScan")
	var requesterSid uuid.UUID
	args := []string{"node", "scsi", "scan"}

	if params.Hba != nil {
		args = append(args, "--hba", *params.Hba)
	}
	if params.Target != nil {
		args = append(args, "--target", *params.Target)
	}
	if params.Lun != nil {
		args = append(args, "--lun", *params.Lun)
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
