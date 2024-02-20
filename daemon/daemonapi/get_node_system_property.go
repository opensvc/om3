package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodeSystemProperty(ctx echo.Context, nodename api.InPathNodeName) error {
	if a.localhost == nodename {
		return a.getLocalNodeSystemProperty(ctx)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.getPeerNodeSystemProperty(ctx, nodename)
	}
}

func (a *DaemonAPI) getPeerNodeSystemProperty(ctx echo.Context, nodename string) error {
	c, err := newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	if resp, err := c.GetNodeSystemPropertyWithResponse(ctx.Request().Context(), nodename); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonAPI) getLocalNodeSystemProperty(ctx echo.Context) error {
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	data, err := n.LoadSystem()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Load system cache", "%s", err)
	}
	items := make(api.PropertyItems, 0)

	for i := 0; i < len(data.Values()); i++ {

		value := data.Values()[i].Value
		if value == nil {
			value = ""
		}

		/* ou
		value := ""
		if data.Values()[i].Value != nil {
			value = data.Values()[i].Value
		}
		*/

		items = append(items, api.PropertyItem{
			Kind: "PropertyItem",
			Data: api.Property{
				Name:   data.Values()[i].Name,
				Source: data.Values()[i].Source,
				Title:  data.Values()[i].Title,
				Error:  data.Values()[i].Error,
				Value:  value,
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		})
	}

	return ctx.JSON(http.StatusOK, api.PropertyList{Kind: "PropertyList", Items: items})
}
