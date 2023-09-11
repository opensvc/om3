package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
)

// GetNetworks returns network status list.
func (a *DaemonApi) GetNodeStatus(ctx echo.Context, params api.GetNodeStatusParams) error {
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
	}
	if err := meta.Expand(); err != nil {
		log.Error().Err(err).Send()
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	data := node.StatusData.GetAll()
	l := make(api.NodeStatusArray, 0)
	for _, e := range data {
		if !meta.HasNode(e.Node) {
			continue
		}
		d := api.NodeStatusItem{
			Meta: api.NodeMeta{
				Node: e.Node,
			},
			Data: api.NodeStatus(*e.Value),
		}
		l = append(l, d)
	}
	return ctx.JSON(http.StatusOK, l)
}
