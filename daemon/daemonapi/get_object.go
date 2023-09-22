package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetObjects(ctx echo.Context, params api.GetObjectsParams) error {
	meta := Meta{
		Context: ctx,
		Path:    params.Path,
	}
	if err := meta.Expand(); err != nil {
		log.Error().Err(err).Send()
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	ostats := object.StatusData.GetAll()
	l := make(api.ObjectItems, 0)
	for _, ostat := range ostats {
		if !meta.HasPath(ostat.Path.String()) {
			continue
		}

		d := api.ObjectItem{
			Meta: api.ObjectMeta{
				Object: ostat.Path.String(),
			},
			Data: api.ObjectData{
				Avail:            api.Status(ostat.Value.Avail.String()),
				FlexMax:          ostat.Value.FlexMax,
				FlexMin:          ostat.Value.FlexMin,
				FlexTarget:       ostat.Value.FlexTarget,
				Frozen:           ostat.Value.Frozen,
				Instances:        make(map[string]api.Instance),
				Orchestrate:      api.Orchestrate(ostat.Value.Orchestrate),
				Overall:          api.Status(ostat.Value.Overall.String()),
				PlacementPolicy:  api.PlacementPolicy(ostat.Value.PlacementPolicy.String()),
				PlacementState:   api.PlacementState(ostat.Value.PlacementState.String()),
				Pool:             ostat.Value.Pool,
				Priority:         int(ostat.Value.Priority),
				Provisioned:      api.Provisioned(ostat.Value.Provisioned.String()),
				Scope:            append([]string{}, ostat.Value.Scope...),
				Size:             ostat.Value.Size,
				Topology:         api.Topology(ostat.Value.Topology.String()),
				UpInstancesCount: ostat.Value.UpInstancesCount,
				UpdatedAt:        ostat.Value.UpdatedAt.String(),
			},
		}
		for nodename, config := range instance.ConfigData.GetByPath(ostat.Path) {
			monitor := instance.MonitorData.Get(ostat.Path, nodename)
			status := instance.StatusData.Get(ostat.Path, nodename)
			d.Data.Instances[nodename] = api.Instance{
				Config:  config,
				Monitor: monitor,
				Status:  status,
			}
		}
		l = append(l, d)
	}
	return ctx.JSON(http.StatusOK, api.ObjectList{Kind: "ObjectList", Items: l})
}
