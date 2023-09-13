package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetResourceConfig(ctx echo.Context, params api.GetResourceConfigParams) error {
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
	l := make(api.ResourceConfigArray, 0)
	for _, config := range configs {
		if !meta.HasPath(config.Path.String()) {
			continue
		}
		if !meta.HasNode(config.Node) {
			continue
		}
		for rid, resourceConfig := range config.Value.Resources {
			if params.Resource != nil && !resourceid.Match(rid, *params.Resource) {
				continue
			}
			d := api.ResourceConfigItem{
				Meta: api.ResourceMeta{
					Node:   config.Node,
					Object: config.Path.String(),
					Rid:    rid,
				},
				Data: resourceConfig,
			}
			l = append(l, d)
		}
	}
	return ctx.JSON(http.StatusOK, l)
}
