package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetInstance(ctx echo.Context, params api.GetInstanceParams) error {
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
	l := make(api.InstanceArray, 0)
	for _, config := range configs {
		if !meta.HasPath(config.Path.String()) {
			continue
		}
		if !meta.HasNode(config.Node) {
			continue
		}
		monitor := instance.MonitorData.Get(config.Path, config.Node)
		status := instance.StatusData.Get(config.Path, config.Node)
		d := api.InstanceItem{
			Meta: api.InstanceMeta{
				Node:   config.Node,
				Object: config.Path.String(),
			},
			Data: api.Instance{
				Config:  config.Value,
				Monitor: monitor,
				Status:  status,
			},
		}
		l = append(l, d)
	}
	return ctx.JSON(http.StatusOK, l)
}
