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

	getCore := func(p naming.Path, ostat *object.Status) api.ObjectCore {
		core := api.ObjectCore{
			Scope:     append([]string{}, ostat.Scope...),
			Instances: make(map[string]api.Instance),
			Priority:  int(ostat.Priority),
			UpdatedAt: ostat.UpdatedAt,
		}
		for nodename, config := range instance.ConfigData.GetByPath(p) {
			monitor := instance.MonitorData.GetByPathAndNode(p, nodename)
			status := instance.StatusData.GetByPathAndNode(p, nodename)
			core.Instances[nodename] = api.Instance{
				Config:  config,
				Monitor: monitor,
				Status:  status,
			}
		}
		return core
	}
	getActor := func(ostat *object.Status) api.ObjectActor {
		actor := api.ObjectActor{
			Avail:            api.Status(ostat.Avail.String()),
			Frozen:           api.ObjectFrozen(ostat.Frozen),
			Orchestrate:      api.Orchestrate(ostat.Orchestrate),
			Overall:          api.Status(ostat.Overall.String()),
			PlacementPolicy:  api.PlacementPolicy(ostat.PlacementPolicy.String()),
			PlacementState:   api.PlacementState(ostat.PlacementState.String()),
			Provisioned:      api.Provisioned(ostat.Provisioned.String()),
			Topology:         api.Topology(ostat.Topology.String()),
			UpInstancesCount: ostat.UpInstancesCount,
		}
		if ostat.Flex != nil {
			actor.Flex = &api.FlexConfig{
				Max:    ostat.Flex.Max,
				Min:    ostat.Flex.Min,
				Target: ostat.Flex.Target,
			}
		}
		return actor
	}

	getObjectVolConfig := func(ostat *object.Status) api.ObjectVolConfig {
		return api.ObjectVolConfig{
			Pool: ostat.Pool,
			Size: ostat.Size,
		}
	}

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
			Data: api.ObjectData{},
		}
		switch p.Kind {
		case naming.KindCcfg:
			d.Data.MergeObjectCore(getCore(p, ostat))
		case naming.KindCfg:
			d.Data.MergeObjectCore(getCore(p, ostat))
		case naming.KindSec:
			d.Data.MergeObjectCore(getCore(p, ostat))
		case naming.KindSvc:
			d.Data.MergeObjectCore(getCore(p, ostat))
			d.Data.MergeObjectActor(getActor(ostat))
		case naming.KindVol:
			d.Data.MergeObjectCore(getCore(p, ostat))
			d.Data.MergeObjectActor(getActor(ostat))
			d.Data.MergeObjectVolConfig(getObjectVolConfig(ostat))
		case naming.KindUsr:
			d.Data.MergeObjectCore(getCore(p, ostat))
		}

		l = append(l, d)
	}
	return l, nil
}
