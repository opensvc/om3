package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
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
	hasRoot := grantsFromContext(ctx).HasRole(rbac.RoleRoot)
	userGrants := grantsFromContext(ctx)

	for _, config := range configs {
		if !hasRoot && !hasRoleGuestOn(userGrants, config.Path.Namespace) {
			continue
		}
		if !meta.HasPath(config.Path.String()) {
			continue
		}
		if !meta.HasNode(config.Node) {
			continue
		}
		items = appendResourceItems(items, config, params.Resource)
	}
	return ctx.JSON(http.StatusOK, api.ResourceList{Kind: "ResourceList", Items: items})
}

// appendResourceItems appends the resources of the instance whose
// configuration is config, selected by resourceSelector when set.
func appendResourceItems(items api.ResourceItems, config instance.DataElement[instance.Config], resourceSelector *string) api.ResourceItems {
	// A configuration is known before the status and the monitor of its
	// instance are, as right after the daemon starts: without a status,
	// the instance has no resource to list yet.
	monitor := instance.MonitorData.GetByPathAndNode(config.Path, config.Node)
	status := instance.StatusData.GetByPathAndNode(config.Path, config.Node)
	if status == nil {
		return items
	}
	// A sec, a cfg, a usr, a nscfg or the cluster configuration has
	// a status, and no resource: its configuration has none of the
	// object part a resource lives in.
	if config.Value == nil || config.Value.ActorConfig == nil {
		return items
	}
	for rid, resourceConfig := range config.Value.Resources {
		if id, err := resourceid.Parse(rid); err != nil {
			continue
		} else if id.DriverGroup() == driver.GroupUnknown {
			continue
		}
		if resourceSelector != nil && !resourceid.Match(rid, *resourceSelector) {
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
		if monitor != nil {
			if e, ok := monitor.Resources[rid]; ok {
				item.Data.Monitor = &e
			}
		}
		if e, ok := status.Resources[rid]; ok {
			item.Data.Status = &e
			items = append(items, item)
		} else {
			for encapNode, encapStatus := range status.Encap {
				if e, ok := encapStatus.Resources[rid]; ok {
					encapItem := item
					encapItem.Meta.EncapNode = encapNode
					encapItem.Data.Status = &e
					items = append(items, encapItem)
				}
			}
		}
	}
	return items
}
