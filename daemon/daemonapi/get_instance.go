package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonAPI) GetInstances(ctx echo.Context, params api.GetInstancesParams) error {
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
		Path:    params.Path,
	}
	name := "GetInstances"
	log := LogHandler(ctx, name)
	if err := meta.Expand(); err != nil {
		log.Errorf("%s: %s", name, err)
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	configs := instance.ConfigData.GetAll()
	l := make(api.InstanceItems, 0)
	hasRoot := grantsFromContext(ctx).HasRole(rbac.RoleRoot)
	userGrants := grantsFromContext(ctx)

	for _, config := range configs {
		if !meta.HasPath(config.Path.String()) {
			continue
		}
		if !meta.HasNode(config.Node) {
			continue
		}
		if !hasRoot && !hasRoleGuestOn(userGrants, config.Path.Namespace) {
			continue
		}

		monitor := instance.MonitorData.GetByPathAndNode(config.Path, config.Node)
		status := instance.StatusData.GetByPathAndNode(config.Path, config.Node)
		d := api.InstanceItem{
			Kind: "InstanceItem",
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
	return ctx.JSON(http.StatusOK, api.InstanceList{Kind: "InstanceList", Items: l})
}

func (a *DaemonAPI) GetInstance(ctx echo.Context, nodename string, namespace string, kind naming.Kind, name string) error {
	log := LogHandler(ctx, "GetInstance")
	path, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		log.Errorf("GetInstance: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "New path", "%s", err)
	}
	config := instance.ConfigData.GetByPathAndNode(path, nodename)
	if config == nil {
		return ctx.NoContent(http.StatusNotFound)
	}
	monitor := instance.MonitorData.GetByPathAndNode(path, nodename)
	status := instance.StatusData.GetByPathAndNode(path, nodename)
	item := api.InstanceItem{
		Kind: "InstanceItem",
		Meta: api.InstanceMeta{
			Node:   nodename,
			Object: path.String(),
		},
		Data: api.Instance{
			Config:  config,
			Monitor: monitor,
			Status:  status,
		},
	}
	return ctx.JSON(http.StatusOK, item)
}
