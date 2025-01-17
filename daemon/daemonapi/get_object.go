package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetObjects(ctx echo.Context, params api.GetObjectsParams) error {
	logName := "GetObjects"
	log := LogHandler(ctx, logName)
	if l, err := a.getObjects(ctx, params.Path); err != nil {
		log.Errorf("%s: %s", logName, err)
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	} else {
		return ctx.JSON(http.StatusOK, api.ObjectList{Kind: "ObjectList", Items: l})
	}
}

func (a *DaemonAPI) GetObject(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	logName := "GetObject"
	log := LogHandler(ctx, logName)
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return err
	}
	s := p.FQN()
	if l, err := a.getObjects(ctx, &s); err != nil {
		log.Errorf("%s: %s", logName, err)
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	} else if len(l) == 0 {
		return JSONProblem(ctx, http.StatusNotFound, "", "")
	} else {
		return ctx.JSON(http.StatusOK, l[0])
	}
}

func (a *DaemonAPI) getObjects(ctx echo.Context, pathSelector *string) (api.ObjectItems, error) {
	meta := Meta{
		Context: ctx,
		Path:    pathSelector,
	}
	if err := meta.Expand(); err != nil {
		return nil, err
	}
	l := make(api.ObjectItems, 0)
	for _, p := range meta.Paths() {
		ostat := object.StatusData.GetByPath(p)
		if ostat == nil {
			continue
		}
		d := api.ObjectItem{
			Kind: "ObjectItem",
			Meta: api.ObjectMeta{
				Object: p.String(),
			},
			Data: api.ObjectData{
				Avail:            api.Status(ostat.Avail.String()),
				FlexMax:          ostat.FlexMax,
				FlexMin:          ostat.FlexMin,
				FlexTarget:       ostat.FlexTarget,
				Frozen:           ostat.Frozen,
				Instances:        make(map[string]api.Instance),
				Orchestrate:      api.Orchestrate(ostat.Orchestrate),
				Overall:          api.Status(ostat.Overall.String()),
				PlacementPolicy:  api.PlacementPolicy(ostat.PlacementPolicy.String()),
				PlacementState:   api.PlacementState(ostat.PlacementState.String()),
				Pool:             ostat.Pool,
				Priority:         int(ostat.Priority),
				Provisioned:      api.Provisioned(ostat.Provisioned.String()),
				Scope:            append([]string{}, ostat.Scope...),
				Size:             ostat.Size,
				Topology:         api.Topology(ostat.Topology.String()),
				UpInstancesCount: ostat.UpInstancesCount,
				UpdatedAt:        ostat.UpdatedAt.String(),
			},
		}
		for nodename, config := range instance.ConfigData.GetByPath(p) {
			monitor := instance.MonitorData.GetByPathAndNode(p, nodename)
			status := instance.StatusData.GetByPathAndNode(p, nodename)
			d.Data.Instances[nodename] = api.Instance{
				Config:  config,
				Monitor: monitor,
				Status:  status,
			}
		}
		l = append(l, d)
	}
	return l, nil
}
