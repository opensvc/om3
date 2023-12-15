package daemonapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func keywordRbac(s string) error {
	op := keyop.Parse(s)
	if strings.HasSuffix(op.Key.Option, "_trigger") {
		return fmt.Errorf("%s", op.Key)
	}
	drvGroup := strings.Split(op.Key.Section, "#")[0]
	switch drvGroup {
	case "app", "task":
		switch op.Key.Option {
		case "script", "start", "stop", "check", "info":
			return fmt.Errorf("%s", op.Key)
		}
	}
	return nil
}

func (a *DaemonApi) PostInstanceActionSet(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceActionSetParams) error {
	log := LogHandler(ctx, "PostInstanceActionSet")

	if v, err := assertGrant(ctx, rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot); !v {
		return err
	}

	if params.Kw == nil {
		return nil
	}

	if isRoot, err := assertGrant(ctx, rbac.GrantRoot); err != nil {
		return err
	} else if !isRoot {
		// Non-root is not allowed to set dangerous keywords.
		for _, s := range *params.Kw {
			if err := keywordRbac(s); err != nil {
				return JSONProblemf(ctx, http.StatusUnauthorized, "Unauthorized keyword", "%s", err)
			}
		}
	}

	var requesterSid uuid.UUID
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	args := []string{p.String(), "set", "--local"}
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
