package daemonapi

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// storeReaderRoles are the roles on a namespace that let a requester see the
// stores its objects may use, and the runs of its objects.
var storeReaderRoles = []rbac.Role{rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin}

// GetStores lists the stores the requester may see: the stores of the
// cluster configuration, which every namespace may use, to a requester with
// a role on a namespace, and the stores of a sec to the ones holding a role
// on a namespace it shares with.
func (a *DaemonAPI) GetStores(ctx echo.Context) error {
	grants := grantsFromContext(ctx)
	if !mayReadStores(grants) {
		return JSONForbiddenMissingRole(ctx, append(storeReaderRoles, rbac.RoleRoot)...)
	}
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	clusterID, _ := n.Cluster()
	items := make([]api.StoreItem, 0)
	for _, item := range n.ListStores() {
		if !mayReadStore(grants, item.Share) {
			continue
		}
		i := api.StoreItem{
			Ref:   item.Ref,
			Type:  item.Type,
			Url:   item.URL,
			Share: item.Share,
		}
		e := cachedStoreStats(item.Ref, storeStatsFirstWait)
		if e.stats != nil {
			stats := apiStoreStats(storeStatsFor(grants, item.Ref, clusterID, *e.stats, e.objects))
			i.Stats = &stats
		}
		if len(e.nodeStats) > 0 {
			m := make(map[string]api.StoreStats, len(e.nodeStats))
			for node, ns := range e.nodeStats {
				m[node] = apiStoreStats(storeStatsFor(grants, item.Ref, clusterID, ns.Stats, ns.Objects))
			}
			i.NodeStats = &m
		}
		items = append(items, i)
	}
	return ctx.JSON(http.StatusOK, api.StoreList{Kind: "StoreList", Items: items})
}

// storeStatsFirstWait is how long a listing waits for the first figures of
// a store, the daemon knowing none yet.
const storeStatsFirstWait = 2 * time.Second

// mayReadStores says the requester holds a role letting it see a store.
func mayReadStores(grants rbac.Grants) bool {
	return grants.HasRole(rbac.RoleRoot) ||
		grants.HasRoleOn("", storeReaderRoles...) ||
		len(grants.Namespaces(storeReaderRoles...)) > 0
}

// mayReadStore says the requester may see a store whose objects of the
// namespaces share may use.
func mayReadStore(grants rbac.Grants, share []string) bool {
	if grants.HasRole(rbac.RoleRoot) || grants.HasRoleOn("", storeReaderRoles...) {
		return true
	}
	if slices.Contains(share, "*") {
		return len(grants.Namespaces(storeReaderRoles...)) > 0
	}
	for _, ns := range share {
		if grants.HasRoleOn(ns, storeReaderRoles...) {
			return true
		}
	}
	return false
}

// mayUseRun says the requester holds one of roles on the namespace of the
// object the run is of. A namespace is one of this cluster: the run of
// another cluster is one a requester sees through a sec store, whose owner
// shared it with its namespace, or as root.
func mayUseRun(grants rbac.Grants, ref string, run backup.Run, clusterID string, roles ...rbac.Role) bool {
	if grants.HasRole(rbac.RoleRoot) {
		return true
	}
	if run.ClusterID != clusterID && !isSecStoreRef(ref) {
		return false
	}
	p, err := naming.ParsePath(run.Path)
	if err != nil {
		return false
	}
	return grants.AssertRoleOn(p.Namespace, roles...)
}

// isSecStoreRef says ref names a store of a sec, from <sec> store#<name>.
func isSecStoreRef(ref string) bool {
	return len(strings.Fields(ref)) > 1
}
