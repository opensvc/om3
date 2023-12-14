package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonApi) PostInstanceActionUnset(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceActionUnsetParams) error {
	log := LogHandler(ctx, "PostInstanceActionUnset")

	if v, err := assertGrant(ctx, rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot); !v {
		return err
	}

	if params.Kw == nil {
		return nil
	}

	var requesterSid uuid.UUID
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	args := []string{p.String(), "unset", "--local"}
	for _, kw := range *params.Kw {
		args = append(args, "--kw", kw)
	}
	if params.WaitLock != nil {
		args = append(args, "--waitlock", *params.WaitLock)
	}
	if params.NoLock != nil {
		args = append(args, "--no-lock")
	}
	if params.RequesterSid != nil {
		requesterSid = *params.RequesterSid
	}
	if sid, err := a.apiExec(ctx, p, requesterSid, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.InstanceActionAccepted{SessionId: sid})
	}
}
