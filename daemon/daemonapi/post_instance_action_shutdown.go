package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonAPI) PostInstanceActionShutdown(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceActionShutdownParams) error {
	if a.localhost == nodename {
		return a.postLocalInstanceActionShutdown(ctx, namespace, kind, name, params)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.postPeerInstanceActionShutdown(ctx, nodename, namespace, kind, name, params)
	}
}

func (a *DaemonAPI) postPeerInstanceActionShutdown(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceActionShutdownParams) error {
	c, err := newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	if resp, err := c.PostInstanceActionShutdownWithResponse(ctx.Request().Context(), nodename, namespace, kind, name, &params); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonAPI) postLocalInstanceActionShutdown(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceActionShutdownParams) error {
	if v, err := assertGrant(ctx, rbac.NewGrant(rbac.RoleOperator, namespace), rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot); !v {
		return err
	}
	log := LogHandler(ctx, "PostInstanceActionShutdown")
	var requesterSid uuid.UUID
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)
	args := []string{p.String(), "shutdown", "--local"}
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
	if params.RequesterSid != nil {
		requesterSid = *params.RequesterSid
	}
	if sid, err := a.apiExec(ctx, p, requesterSid, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.InstanceActionAccepted{SessionID: sid})
	}

}
