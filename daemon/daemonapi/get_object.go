package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
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

	hasRoot := grantsFromContext(ctx).HasRole(rbac.RoleRoot)
	userGrants := grantsFromContext(ctx)

	l := make(api.ObjectItems, 0)
	for _, p := range meta.Paths() {
		if !hasRoot && !hasRoleGuestOn(userGrants, p.Namespace) {
			continue
		}
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
				Scope:     append([]string{}, ostat.Scope...),
				Instances: make(map[string]api.Instance),
				Priority:  int(ostat.Priority),
				UpdatedAt: ostat.UpdatedAt.String(),
			},
		}
		if ostat.ActorStatus != nil {
			d.Data.Avail = api.Status(ostat.Avail.String())
			d.Data.Frozen = ostat.Frozen
			d.Data.Orchestrate = api.Orchestrate(ostat.Orchestrate)
			d.Data.Overall = api.Status(ostat.Overall.String())
			d.Data.PlacementPolicy = api.PlacementPolicy(ostat.PlacementPolicy.String())
			d.Data.PlacementState = api.PlacementState(ostat.PlacementState.String())
			d.Data.Provisioned = api.Provisioned(ostat.Provisioned.String())
			d.Data.Topology = api.Topology(ostat.Topology.String())
			d.Data.UpInstancesCount = ostat.UpInstancesCount
		}
		if ostat.FlexStatus != nil {
			d.Data.FlexMax = ostat.FlexMax
			d.Data.FlexMin = ostat.FlexMin
			d.Data.FlexTarget = ostat.FlexTarget
		}
		if ostat.VolStatus != nil {
			d.Data.Pool = &ostat.Pool
			d.Data.Size = &ostat.Size
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
