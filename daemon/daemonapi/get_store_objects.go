package daemonapi

import (
	"cmp"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// storeObjectsFirstWait is how long a listing of the objects of a store
// waits for its first figures, the daemon knowing none yet.
const storeObjectsFirstWait = 10 * time.Second

// GetStoreObjects lists the objects a store holds runs of, with their
// figures, from the stats the daemon keeps of the store: the objects of the
// namespaces the requester holds a role on, as for the snapshots.
func (a *DaemonAPI) GetStoreObjects(ctx echo.Context, params api.GetStoreObjectsParams) error {
	grants := grantsFromContext(ctx)
	if !mayReadStores(grants) {
		return JSONForbiddenMissingRole(ctx, append(storeReaderRoles, rbac.RoleRoot)...)
	}
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	i := slices.IndexFunc(n.ListStores(), func(item object.StoreItem) bool { return item.Ref == params.Store })
	if i < 0 || !mayReadStore(grants, n.ListStores()[i].Share) {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "store %s: not declared, or not one a namespace you hold a role on may use", params.Store)
	}
	clusterID, _ := n.Cluster()
	e := cachedStoreStats(params.Store, storeObjectsFirstWait)
	items := make([]api.StoreObjectStats, 0, len(e.objects))
	for _, o := range e.objects {
		if o.Runs+o.Incomplete == 0 {
			continue
		}
		run := backup.Run{Path: o.Path, ClusterID: o.ClusterID}
		if !mayUseRun(grants, params.Store, run, clusterID, storeReaderRoles...) {
			continue
		}
		items = append(items, apiStoreObjectStats(o))
	}
	slices.SortFunc(items, func(a, b api.StoreObjectStats) int {
		if c := compareTimePtr(b.LastCreatedAt, a.LastCreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})
	return ctx.JSON(http.StatusOK, api.StoreObjectStatsList{Kind: "StoreObjectStatsList", Items: items})
}

// compareTimePtr orders two times, a nil one first.
func compareTimePtr(a, b *time.Time) int {
	switch {
	case a == nil && b == nil:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	default:
		return cmp.Compare(a.UnixNano(), b.UnixNano())
	}
}
