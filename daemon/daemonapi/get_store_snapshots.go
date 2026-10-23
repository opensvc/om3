package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// GetStoreSnapshots lists the runs the stores the requester may see hold, of
// the objects of the namespaces it holds a role on. A store keeping its runs
// in a directory of each node is read on every node, over ssh.
//
// A store not readable is logged and skipped, so one store down does not hide
// the runs of the others, unless it is the one store asked.
func (a *DaemonAPI) GetStoreSnapshots(ctx echo.Context, params api.GetStoreSnapshotsParams) error {
	log := LogHandler(ctx, "GetStoreSnapshots")
	grants := grantsFromContext(ctx)
	if !mayReadStores(grants) {
		return JSONForbiddenMissingRole(ctx, append(storeReaderRoles, rbac.RoleRoot)...)
	}
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	clusterID, _ := n.Cluster()
	refs := make([]string, 0)
	for _, item := range n.ListStores() {
		if params.Store != nil && *params.Store != item.Ref {
			continue
		}
		if mayReadStore(grants, item.Share) {
			refs = append(refs, item.Ref)
		}
	}
	if params.Store != nil && len(refs) == 0 {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "store %s: not declared, or not one a namespace you hold a role on may use", *params.Store)
	}
	filter := backup.RunFilter{}
	if params.Path != nil {
		filter.Path = *params.Path
	}
	if params.Cluster != nil {
		filter.Cluster = *params.Cluster
	}
	reqCtx := ctx.Request().Context()
	items := make([]api.StoreSnapshot, 0)
	for _, ref := range refs {
		s, err := n.Store(ref, naming.Path{})
		if err != nil {
			if params.Store != nil {
				return JSONProblemf(ctx, http.StatusInternalServerError, "Store", "%s", err)
			}
			log.Warnf("%s", err)
			continue
		}
		runs, err := s.Runs(reqCtx, filter)
		_ = s.Close()
		if err != nil {
			if params.Store != nil {
				return JSONProblemf(ctx, http.StatusInternalServerError, "Store", "%s", err)
			}
			log.Warnf("%s", err)
		}
		for _, run := range runs {
			if !mayUseRun(grants, ref, run, clusterID, storeReaderRoles...) {
				continue
			}
			items = append(items, api.StoreSnapshot{
				Store:       s.Ref(),
				Path:        run.Path,
				Id:          run.ID,
				CreatedAt:   run.CreatedAt,
				ClusterId:   run.ClusterID,
				ClusterName: run.ClusterName,
				Node:        run.Node,
				Members:     run.MemberNames(),
				Protected:   run.Protected,
				Note:        run.Note,
			})
		}
	}
	return ctx.JSON(http.StatusOK, api.StoreSnapshotList{Kind: "StoreSnapshotList", Items: items})
}
