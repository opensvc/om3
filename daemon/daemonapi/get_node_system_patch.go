package daemonapi

import (
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/object"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodeSystemPatch(ctx echo.Context, nodename api.InPathNodeName) error {
	if a.localhost == nodename {
		return a.getLocalNodeSystemPatch(ctx)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.getPeerNodeSystemPatch(ctx, nodename)
	}
}

func (a *DaemonAPI) getPeerNodeSystemPatch(ctx echo.Context, nodename string) error {
	c, err := newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	if resp, err := c.GetNodeSystemPatchWithResponse(ctx.Request().Context(), nodename); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonAPI) getLocalNodeSystemPatch(ctx echo.Context) error {
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	data, err := n.LoadPatch()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Load patch cache", "%s", err)
	}
	items := make(api.PatchItems, 0)

	for i := 0; i < len(data); i++ {

		items = append(items, api.PatchItem{
			Kind: "PatchItem",
			Data: api.Patch{
				Number:      data[i].Number,
				Revision:    data[i].Revision,
				InstalledAt: data[i].InstalledAt.String(),
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		})
	}

	return ctx.JSON(http.StatusOK, api.PatchList{Kind: "PatchList", Items: items})
}
