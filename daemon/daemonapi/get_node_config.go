package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetNodeConfig(ctx echo.Context, params api.GetNodeConfigParams) error {
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
	}
	if err := meta.Expand(); err != nil {
		log.Error().Err(err).Send()
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	data := node.ConfigData.GetAll()
	l := make(api.NodeConfigArray, 0)
	for _, e := range data {
		if !meta.HasNode(e.Node) {
			continue
		}
		d := api.NodeConfigItem{
			Meta: api.NodeMeta{
				Node: e.Node,
			},
			Data: api.NodeConfig(*e.Value),
		}
		l = append(l, d)
	}
	return ctx.JSON(http.StatusOK, l)
}
