package daemonapi

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) PostInstanceActionRestart(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceActionRestartParams) error {
	if v, err := assertOperator(ctx, namespace); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.postLocalInstanceActionRestart(ctx, namespace, kind, name, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostInstanceActionRestart(ctx.Request().Context(), nodename, namespace, kind, name, &params)
	})
}

func (a *DaemonAPI) postLocalInstanceActionRestart(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceActionRestartParams) error {
	log := LogHandler(ctx, "PostInstanceActionRestart")
	var requesterSid uuid.UUID
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)
	args := []string{p.String(), "instance", "restart"}
	if params.DisableRollback != nil && *params.DisableRollback {
		args = append(args, "--disable-rollback")
	}
	if params.Force != nil && *params.Force {
		args = append(args, "--force")
	}
	if params.To != nil && *params.To != "" {
		args = append(args, "--to", *params.To)
	}
	if params.Rid != nil && *params.Rid != "" {
		args = append(args, "--rid", *params.Rid)
	}
	if params.Subset != nil && *params.Subset != "" {
		args = append(args, "--subset", *params.Subset)
	}
	if params.Tag != nil && *params.Tag != "" {
		args = append(args, "--tag", *params.Tag)
	}
	if params.Slave != nil && len(*params.Slave) > 0 {
		args = append(args, "--slave", strings.Join(*params.Slave, ","))
	}
	if params.Slaves != nil && *params.Slaves {
		args = append(args, "--slaves")
	}
	if params.Master != nil && *params.Master {
		args = append(args, "--master")
	}
	if params.RequesterSid != nil {
		requesterSid = *params.RequesterSid
	}
	if sid, err := a.apiExec(ctx, p, requesterSid, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.InstanceActionAccepted{SessionID: sid})
	}
}
