package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonApi) PostInstanceActionUpdate(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceActionUpdateParams) error {
	log := LogHandler(ctx, "PostInstanceActionUpdate")

	if v, err := assertGrant(ctx, rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot); !v {
		return err
	}

	if isRoot, err := assertGrant(ctx, rbac.GrantRoot); err != nil {
		return err
	} else if !isRoot {
		// Non-root is not allowed to set dangerous keywords.
		if params.Set != nil {
			for _, op := range *params.Set {
				if err := keyopStringRbac(op); err != nil {
					return JSONProblemf(ctx, http.StatusUnauthorized, "Unauthorized keyword", "%s", err)
				}
			}
		}
	}

	var requesterSid uuid.UUID
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	args := []string{p.String(), "update", "--local"}
	if params.Delete != nil {
		for _, kw := range *params.Delete {
			args = append(args, "--delete", kw)
		}
	}
	if params.Unset != nil {
		for _, kw := range *params.Unset {
			args = append(args, "--unset", kw)
		}
	}
	if params.Set != nil {
		for _, kw := range *params.Set {
			args = append(args, "--set", kw)
		}
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
