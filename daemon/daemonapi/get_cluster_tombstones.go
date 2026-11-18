package daemonapi

import (
	"net/http"
	"slices"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/daemon/tombmon"
)

// GetClusterTombstones returns the tombstones the nodes publish, a record per
// node deleting its instance, every node holding them all, with the nodes
// still publishing the version of the config deleted. A user other than root
// sees the ones of the namespaces it is guest of.
func (a *DaemonAPI) GetClusterTombstones(ctx echo.Context) error {
	grants := grantsFromContext(ctx)
	hasRoot := grants.HasRole(rbac.RoleRoot)
	isGuestOn := guestOn(grants)
	items := make([]api.Tombstone, 0)
	for _, e := range tombmon.List() {
		p, err := naming.ParsePath(e.Record.Path)
		if err != nil {
			continue
		}
		if !hasRoot && !isGuestOn(p.Namespace) {
			continue
		}
		holders := make([]string, 0)
		for n, cfg := range instance.ConfigData.GetByPath(p) {
			if e.Record.Kills(e.Record.Path, cfg.UpdatedAt) {
				holders = append(holders, n)
			}
		}
		slices.Sort(holders)
		item := api.Tombstone{
			OrchestrationID: e.Record.OrchestrationID,
			Node:            e.Node,
			Path:            e.Record.Path,
			Kind:            api.TombstoneKind(e.Record.Kind),
			Version:         e.Record.Version,
			DeletedAt:       e.Record.DeletedAt,
			Holders:         holders,
		}
		if e.Record.ObjectID != "" {
			id := e.Record.ObjectID
			item.ObjectID = &id
		}
		items = append(items, item)
	}
	return ctx.JSON(http.StatusOK, api.TombstoneList{Kind: api.TombstoneListKindTombstoneList, Items: items})
}
