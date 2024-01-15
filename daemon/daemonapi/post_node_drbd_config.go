package daemonapi

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonApi) PostNodeDRBDConfig(ctx echo.Context, nodename string, params api.PostNodeDRBDConfigParams) error {
	if v, err := assertGrant(ctx, rbac.GrantRoot); !v {
		return err
	}
	payload := api.PostNodeDRBDConfigRequest{}
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	if a.localhost == nodename {
		return a.postLocalDRBDConfig(ctx, params, payload)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.postPeerDRBDConfig(ctx, nodename, params, payload)
	}
}

func (a *DaemonApi) postPeerDRBDConfig(ctx echo.Context, nodename string, params api.PostNodeDRBDConfigParams, payload api.PostNodeDRBDConfigRequest) error {
	c, err := newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	if resp, err := c.PostNodeDRBDConfigWithResponse(ctx.Request().Context(), nodename, &params, payload); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonApi) postLocalDRBDConfig(ctx echo.Context, params api.PostNodeDRBDConfigParams, payload api.PostNodeDRBDConfigRequest) error {
	if a, ok := pendingDRBDAllocations.get(payload.AllocationID); !ok || time.Now().After(a.ExpiredAt) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "drbd allocation expired: %#v", a)
	}
	if strings.Contains(params.Name, "..") || strings.HasPrefix(params.Name, "/") {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid body", "The 'name' parameter must be a basename.")
	}
	cf := fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	if err := os.WriteFile(cf, payload.Data, 0644); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Error writing drbd res file", "%s", err)
	}
	return ctx.JSON(http.StatusOK, nil)
}
