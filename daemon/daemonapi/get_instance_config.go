package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetInstanceConfig(ctx echo.Context, params api.GetInstanceConfigParams) error {
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
		Path:    params.Path,
	}
	if err := meta.Expand(); err != nil {
		log.Error().Err(err).Send()
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	data := instance.ConfigData.GetAll()
	l := make(api.InstanceConfigArray, 0)
	for _, e := range data {
		if !meta.HasPath(e.Path.String()) {
			continue
		}
		if !meta.HasNode(e.Node) {
			continue
		}
		d := api.InstanceConfigItem{
			Meta: api.InstanceMeta{
				Node:   e.Node,
				Object: e.Path.String(),
			},
			Data: api.InstanceConfig(*e.Value),
		}
		l = append(l, d)
	}
	return ctx.JSON(http.StatusOK, l)
}
