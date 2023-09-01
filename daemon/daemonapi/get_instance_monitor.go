package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/api"
)

// GetNetworks returns network status list.
func (a *DaemonApi) GetInstanceMonitor(ctx echo.Context, params api.GetInstanceMonitorParams) error {
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
		Path:    params.Path,
	}
	if err := meta.Expand(); err != nil {
		log.Error().Err(err).Send()
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	data := instance.MonitorData.GetAll()
	l := make(api.GetInstanceMonitorArray, 0)
	for _, e := range data {
		if !meta.HasPath(e.Path.String()) {
			continue
		}
		if !meta.HasNode(e.Node) {
			continue
		}
		d := api.GetInstanceMonitorElement{
			Meta: api.InstanceMeta{
				Node:   e.Node,
				Object: e.Path.String(),
			},
			Data: api.InstanceMonitor(*e.Value),
		}
		l = append(l, d)
	}
	return ctx.JSON(http.StatusOK, l)
}
