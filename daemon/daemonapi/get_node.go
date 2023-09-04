package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetNode(ctx echo.Context, params api.GetNodeParams) error {
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
	}
	if err := meta.Expand(); err != nil {
		log.Error().Err(err).Send()
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	configs := node.ConfigData.GetAll()
	l := make(api.NodeArray, 0)
	for _, config := range configs {
		if !meta.HasNode(config.Node) {
			continue
		}
		monitor := node.MonitorData.Get(config.Node)
		status := node.StatusData.Get(config.Node)
		d := api.NodeItem{
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
	return ctx.JSON(http.StatusOK, l)
}
