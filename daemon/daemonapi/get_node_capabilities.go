package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/capabilities"
)

func (a *DaemonApi) GetNodeCapabilities(ctx echo.Context, nodename string) error {
	if a.localhost == nodename {
		return a.getLocalCapabilities(ctx)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.getPeerCapabilities(ctx, nodename)
	}
}

func (a *DaemonApi) getPeerCapabilities(ctx echo.Context, nodename string) error {
	c, err := newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	if resp, err := c.GetNodeCapabilitiesWithResponse(ctx.Request().Context(), nodename); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonApi) getLocalCapabilities(ctx echo.Context) error {
	caps, err := capabilities.Load()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Load capabilities", "%s", err)
	}
	resp := api.CapabilityList{
		Kind: "CapabilityList",
	}
	for _, e := range caps {
		item := api.CapabilityItem{
			Kind: "CapabilityItem",
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
			Data: api.Capability{
				Name: e,
			},
		}
		resp.Items = append(resp.Items, item)
	}
	return ctx.JSON(http.StatusOK, resp)
}
