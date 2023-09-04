package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
)

// GetNetworks returns network status list.
func (a *DaemonApi) GetNodeMonitor(ctx echo.Context, params api.GetNodeMonitorParams) error {
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
	}
	if err := meta.Expand(); err != nil {
		log.Error().Err(err).Send()
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	data := node.MonitorData.GetAll()
	l := make(api.NodeMonitorArray, 0)
	for _, e := range data {
		if !meta.HasNode(e.Node) {
			continue
		}
		d := api.NodeMonitorItem{
			Meta: api.NodeMeta{
				Node: e.Node,
			},
			Data: api.NodeMonitor(*e.Value),
		}
		l = append(l, d)
	}
	return ctx.JSON(http.StatusOK, l)
}
