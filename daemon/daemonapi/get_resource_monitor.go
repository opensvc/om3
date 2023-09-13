package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetResourceMonitor(ctx echo.Context, params api.GetResourceMonitorParams) error {
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
		Path:    params.Path,
	}
	if err := meta.Expand(); err != nil {
		log.Error().Err(err).Send()
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	monitors := instance.MonitorData.GetAll()
	l := make(api.ResourceMonitorArray, 0)
	for _, monitor := range monitors {
		if !meta.HasPath(monitor.Path.String()) {
			continue
		}
		if !meta.HasNode(monitor.Node) {
			continue
		}
		for rid, resourceMonitor := range monitor.Value.Resources {
			if params.Resource != nil && !resourceid.Match(rid, *params.Resource) {
				continue
			}
			d := api.ResourceMonitorItem{
				Meta: api.ResourceMeta{
					Node:   monitor.Node,
					Object: monitor.Path.String(),
					Rid:    rid,
				},
				Data: resourceMonitor,
			}
			l = append(l, d)
		}
	}
	return ctx.JSON(http.StatusOK, l)
}
