package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetResourceStatus(ctx echo.Context, params api.GetResourceStatusParams) error {
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
		Path:    params.Path,
	}
	if err := meta.Expand(); err != nil {
		log.Error().Err(err).Send()
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	statuses := instance.StatusData.GetAll()
	l := make(api.ResourceStatusArray, 0)
	for _, status := range statuses {
		if !meta.HasPath(status.Path.String()) {
			continue
		}
		if !meta.HasNode(status.Node) {
			continue
		}
		for rid, resourceStatus := range status.Value.Resources {
			if params.Resource != nil && rid != *params.Resource {
				continue
			}
			d := api.ResourceStatusItem{
				Meta: api.ResourceMeta{
					Node:   status.Node,
					Object: status.Path.String(),
					Rid:    rid,
				},
				Data: resourceStatus,
			}
			l = append(l, d)
		}
	}
	return ctx.JSON(http.StatusOK, l)
}
