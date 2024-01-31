package daemonapi

import (
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/object"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodeDiscoverGroup(ctx echo.Context, nodename api.InPathNodeName) error {
	if a.localhost == nodename {
		return a.getLocalNodeDiscoverGroup(ctx)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.getPeerNodeDiscoverGroup(ctx, nodename)
	}
}

func (a *DaemonAPI) getPeerNodeDiscoverGroup(ctx echo.Context, nodename string) error {
	c, err := newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	if resp, err := c.GetNodeDiscoverGroupWithResponse(ctx.Request().Context(), nodename); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonAPI) getLocalNodeDiscoverGroup(ctx echo.Context) error {
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	data, err := n.LoadAsset()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Load asset cache", "%s", err)
	}
	items := make(api.GroupItems, len(data.GIDS))
	for i := 0; i < len(data.GIDS); i++ {
		items[i] = api.GroupItem{
			Kind: "GroupItem",
			Data: api.Group{
				ID:   data.GIDS[i].ID,
				Name: data.GIDS[i].Name,
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		}
	}

	return ctx.JSON(http.StatusOK, api.GroupList{Kind: "GroupList", Items: items})
}
