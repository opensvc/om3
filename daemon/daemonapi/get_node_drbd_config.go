package daemonapi

import (
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonApi) GetNodeDRBDConfig(ctx echo.Context, nodename string, params api.GetNodeDRBDConfigParams) error {
	log := LogHandler(ctx, "GetNodeDRBDConfig")
	log.Debugf("starting")
	if params.Name == "" {
		log.Warnf("invalid file name: %s", params.Name)
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "invalid file name: %s", params.Name)
	}
	if v, err := assertGrant(ctx, rbac.GrantRoot); !v {
		return err
	}
	if a.localhost == nodename {
		return a.getLocalDRBDConfig(ctx, params)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.getPeerDRBDConfig(ctx, nodename, params)
	}
}

func (a *DaemonApi) getPeerDRBDConfig(ctx echo.Context, nodename string, params api.GetNodeDRBDConfigParams) error {
	c, err := client.New(client.WithURL(nodename))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	if resp, err := c.GetNodeDRBDConfigWithResponse(ctx.Request().Context(), nodename, &params); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonApi) getLocalDRBDConfig(ctx echo.Context, params api.GetNodeDRBDConfigParams) error {
	filename := fmt.Sprintf("/etc/drbd.d/%s.res", params.Name)
	resp := api.DRBDConfig{}

	if data, err := os.ReadFile(filename); err != nil {
		log.Infof("ReadFile %s (may be deleted): %s", filename, err)
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "ReadFile %s (may be deleted)", filename)
	} else {
		resp.Data = data
	}

	return ctx.JSON(http.StatusOK, resp)
}
