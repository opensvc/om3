package daemonapi

import (
	"net/http"
	"slices"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/tombstone"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/daemon/tombmon"
)

// GetClusterTombstones returns the tombstones the nodes publish, the copies
// merged per orchestration: every node holds them all, so the node asked
// answers. A user other than root sees the ones of the namespaces it is
// guest of.
func (a *DaemonAPI) GetClusterTombstones(ctx echo.Context) error {
	grants := grantsFromContext(ctx)
	hasRoot := grants.HasRole(rbac.RoleRoot)
	isGuestOn := guestOn(grants)
	items := make([]api.Tombstone, 0)
	for _, rec := range tombmon.List() {
		if !hasRoot {
			p, err := naming.ParsePath(rec.Path)
			if err != nil || !isGuestOn(p.Namespace) {
				continue
			}
		}
		items = append(items, tombstoneToAPI(rec, tombstone.Data.Publishers(rec.OrchestrationID)))
	}
	return ctx.JSON(http.StatusOK, api.TombstoneList{Kind: api.TombstoneListKindTombstoneList, Items: items})
}

func tombstoneToAPI(rec tombstone.Record, publishers []string) api.Tombstone {
	t := api.Tombstone{
		OrchestrationID: rec.OrchestrationID,
		Path:            rec.Path,
		Kind:            api.TombstoneKind(rec.Kind),
		PurgedAt:        rec.PurgedAt,
		Nodes:           publishers,
		Holders:         make([]api.TombstoneHolder, 0, len(rec.Holders)),
	}
	for _, n := range tombstoneHolderNodes(rec.Holders) {
		h := rec.Holders[n]
		item := api.TombstoneHolder{
			Node:  n,
			State: api.TombstoneHolderState(h.State),
			Mode:  api.TombstoneHolderMode(h.Mode),
		}
		if h.ClearedBy != "" {
			s := string(h.ClearedBy)
			item.ClearedBy = &s
		}
		t.Holders = append(t.Holders, item)
	}
	return t
}

func tombstoneHolderNodes(m map[string]tombstone.Holder) []string {
	l := make([]string, 0, len(m))
	for k := range m {
		l = append(l, k)
	}
	slices.Sort(l)
	return l
}
