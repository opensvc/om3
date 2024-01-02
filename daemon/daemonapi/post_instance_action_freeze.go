package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonApi) PostInstanceActionFreeze(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceActionFreezeParams) error {
	if a.localhost == nodename {
		return a.postLocalInstanceActionFreeze(ctx, namespace, kind, name, params)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.postPeerInstanceActionFreeze(ctx, nodename, namespace, kind, name, params)
	}
}

func (a *DaemonApi) postPeerInstanceActionFreeze(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string, params api.PostInstanceActionFreezeParams) error {
	c, err := client.New(client.WithURL(nodename))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	if resp, err := c.PostInstanceActionFreezeWithResponse(ctx.Request().Context(), nodename, namespace, kind, name, &params); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonApi) postLocalInstanceActionFreeze(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.PostInstanceActionFreezeParams) error {
	if v, err := assertGrant(ctx, rbac.NewGrant(rbac.RoleOperator, namespace), rbac.NewGrant(rbac.RoleAdmin, namespace), rbac.GrantRoot); !v {
		return err
	}
	log := LogHandler(ctx, "PostInstanceActionFreeze")
	var requesterSid uuid.UUID
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)
	args := []string{p.String(), "freeze", "--local"}
	if params.RequesterSid != nil {
		requesterSid = *params.RequesterSid
	}
	if sid, err := a.apiExec(ctx, p, requesterSid, args, log); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "", "%s", err)
	} else {
		return ctx.JSON(http.StatusOK, api.InstanceActionAccepted{SessionId: sid})
	}
}
