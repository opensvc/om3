package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetResource(ctx echo.Context, params api.GetResourceParams) error {
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
		Path:    params.Path,
	}
	if err := meta.Expand(); err != nil {
		log.Error().Err(err).Send()
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	configs := instance.ConfigData.GetAll()
	l := make(api.ResourceArray, 0)
	for _, config := range configs {
		if !meta.HasPath(config.Path.String()) {
			continue
		}
		if !meta.HasNode(config.Node) {
			continue
		}
		monitor := instance.MonitorData.Get(config.Path, config.Node)
		status := instance.StatusData.Get(config.Path, config.Node)
		for rid, resourceConfig := range config.Value.Resources {
			if params.Resource != nil && rid != *params.Resource {
				continue
			}
			d := api.ResourceItem{
				Meta: api.ResourceMeta{
					Node:   config.Node,
					Object: config.Path.String(),
					Rid:    rid,
				},
				Data: api.Resource{
					Config: &resourceConfig,
				},
			}
			if e, ok := monitor.Resources[rid]; ok {
				d.Data.Monitor = &e
			}
			if e, ok := status.Resources[rid]; ok {
				d.Data.Status = &e
			}
			l = append(l, d)
		}
	}
	return ctx.JSON(http.StatusOK, l)
}
