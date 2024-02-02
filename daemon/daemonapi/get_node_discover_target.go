package daemonapi

import (
	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"net/http"
)

func (a *DaemonAPI) GetNodeDiscoverTarget(ctx echo.Context, nodename api.InPathNodeName) error {
	if a.localhost == nodename {
		return a.getLocalNodeDiscoverTarget(ctx)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.getPeerNodeDiscoverTarget(ctx, nodename)
	}
}

func (a *DaemonAPI) getPeerNodeDiscoverTarget(ctx echo.Context, nodename string) error {
	c, err := newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	if resp, err := c.GetNodeDiscoverTargetWithResponse(ctx.Request().Context(), nodename); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonAPI) getLocalNodeDiscoverTarget(ctx echo.Context) error {
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	data, err := n.LoadAsset()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Load asset cache", "%s", err)
	}
	items := make(api.SANPathItems, len(data.Targets))
	for i := 0; i < len(data.Targets); i++ {
		items[i] = api.SANPath{
			Initiator: api.SANPathInitiator{
				Name: &data.Targets[i].Initiator.Name,
				Type: &data.Targets[i].Initiator.Type,
			},
			Target: api.SANPathTarget{
				Name: &data.Targets[i].Target.Name,
				Type: &data.Targets[i].Target.Type,
			},
		}
	}

	return ctx.JSON(http.StatusOK, api.SANPathList{Kind: "SANPathList", Items: items})
}
