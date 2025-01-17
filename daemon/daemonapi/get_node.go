package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodes(ctx echo.Context, params api.GetNodesParams) error {
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
	}
	name := "GetNodes"
	log := LogHandler(ctx, name)
	if err := meta.Expand(); err != nil {
		log.Errorf("%s: %s", name, err)
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	configs := node.ConfigData.GetAll()
	l := make(api.NodeItems, 0)
	for _, config := range configs {
		if !meta.HasNode(config.Node) {
			continue
		}
		monitor := node.MonitorData.GetByNode(config.Node)
		status := node.StatusData.GetByNode(config.Node)
		d := api.NodeItem{
			Kind: "NodeItem",
			Meta: api.NodeMeta{
				Node: config.Node,
			},
			Data: api.Node{
				Config:  config.Value,
				Monitor: monitor,
				Status:  status,
			},
		}
		l = append(l, d)
	}
	return ctx.JSON(http.StatusOK, api.NodeList{Kind: "NodeList", Items: l})
}
