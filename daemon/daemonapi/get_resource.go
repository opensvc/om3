package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetResources(ctx echo.Context, params api.GetResourcesParams) error {
	name := "GetResources"
	log := LogHandler(ctx, name)
	meta := Meta{
		Context: ctx,
		Node:    params.Node,
		Path:    params.Path,
	}
	if err := meta.Expand(); err != nil {
		log.Errorf("%s: %s", name, err)
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	configs := instance.ConfigData.GetAll()
	items := make(api.ResourceItems, 0)
	for _, config := range configs {
		if _, err := assertGuest(ctx, config.Path.Namespace); err != nil {
			continue
		}
		if !meta.HasPath(config.Path.String()) {
			continue
		}
		if !meta.HasNode(config.Node) {
			continue
		}
		monitor := instance.MonitorData.GetByPathAndNode(config.Path, config.Node)
		status := instance.StatusData.GetByPathAndNode(config.Path, config.Node)
		for rid, resourceConfig := range config.Value.Resources {
			if id, err := resourceid.Parse(rid); err != nil {
				continue
			} else if id.DriverGroup() == driver.GroupUnknown {
				continue
			}
			if params.Resource != nil && !resourceid.Match(rid, *params.Resource) {
				continue
			}
			item := api.ResourceItem{
				Kind: "ResourceItem",
				Meta: api.ResourceMeta{
					Node:   config.Node,
					Object: config.Path.String(),
					RID:    rid,
				},
				Data: api.Resource{
					Config: &resourceConfig,
				},
			}
			if e, ok := monitor.Resources[rid]; ok {
				item.Data.Monitor = &e
			}
			if e, ok := status.Resources[rid]; ok {
				item.Data.Status = &e
			}
			items = append(items, item)
		}
	}
	return ctx.JSON(http.StatusOK, api.ResourceList{Kind: "ResourceList", Items: items})
}
