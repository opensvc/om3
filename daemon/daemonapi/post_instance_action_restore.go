package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostInstanceActionRestore(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceActionRestoreParams) error {
	if v, err := assertOperator(ctx, namespace); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.postLocalInstanceActionRestore(ctx, namespace, kind, name, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostInstanceActionRestore(ctx.Request().Context(), nodename, namespace, kind, name, &params)
	})
}

func (a *DaemonAPI) postLocalInstanceActionRestore(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceActionRestoreParams) error {
	log := LogHandler(ctx, "PostInstanceActionRestore")
	var requesterSessionID uuid.UUID
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)
	if v, err := assertConfigUpdatedAt(ctx, p, params.ConfigUpdatedAt); !v {
		return err
	}
	args := []string{p.String(), "sync", "restore"}
	if params.Rid != nil && *params.Rid != "" {
		args = append(args, "--rid", *params.Rid)
	}
	if params.Subset != nil && *params.Subset != "" {
		args = append(args, "--subset", *params.Subset)
	}
	if params.Tag != nil && *params.Tag != "" {
		args = append(args, "--tag", *params.Tag)
	}
	if params.Force != nil && *params.Force {
		args = append(args, "--force")
	}
	if params.Snapshot != nil && *params.Snapshot != "" {
		args = append(args, "--snapshot", *params.Snapshot)
	}
	if params.Into != nil && *params.Into != "" {
		args = append(args, "--into", *params.Into)
	}
	if params.Config != nil && *params.Config {
		args = append(args, "--config")
	}
	if params.Head != nil {
		for _, rid := range *params.Head {
			args = append(args, "--head", rid)
		}
	}
	if params.SessionID != nil {
		requesterSessionID = *params.SessionID
	}
	if sessionID, execID, err := a.apiExec(ctx, p, requesterSessionID, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.InstanceActionAccepted{SessionID: sessionID, ExecID: execID})
	}
}
