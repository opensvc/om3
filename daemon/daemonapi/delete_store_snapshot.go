package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// DeleteStoreSnapshot removes a run from a store, protected or not: the
// retention never removes a protected one. The run is one of an object of a
// namespace the requester is an administrator of.
func (a *DaemonAPI) DeleteStoreSnapshot(ctx echo.Context, params api.DeleteStoreSnapshotParams) error {
	log := LogHandler(ctx, "DeleteStoreSnapshot")
	grants := grantsFromContext(ctx)
	if !grants.HasRole(rbac.RoleRoot, rbac.RoleAdmin) {
		return JSONForbiddenMissingRole(ctx, rbac.RoleAdmin, rbac.RoleRoot)
	}
	if params.Id == "" {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request", "no snapshot id")
	}
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	var share []string
	for _, item := range n.ListStores() {
		if item.Ref == params.Store {
			share = item.Share
			break
		}
	}
	if share == nil || !mayReadStore(grants, share) {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "store %s: not declared, or not one a namespace you hold a role on may use", params.Store)
	}
	s, err := n.Store(params.Store, naming.Path{})
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Store", "%s", err)
	}
	defer s.Close()
	pruner, ok := s.(backup.Pruner)
	if !ok {
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request", "store %s removes no run: its runs go as its server decides", s.Ref())
	}
	reqCtx := ctx.Request().Context()
	run, err := backup.FindRun(reqCtx, s, backup.RunFilter{ID: params.Id, Incomplete: true})
	switch {
	case errors.Is(err, backup.ErrNotFound):
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "%s", err)
	case errors.Is(err, backup.ErrAmbiguous):
		return JSONProblemf(ctx, http.StatusBadRequest, "Bad request", "%s", err)
	case err != nil:
		return JSONProblemf(ctx, http.StatusInternalServerError, "Store", "%s", err)
	}
	clusterID, _ := n.Cluster()
	if !mayUseRun(grants, params.Store, run, clusterID, rbac.RoleAdmin) {
		return JSONProblemf(ctx, http.StatusForbidden, "Forbidden", "snapshot %s is of %s on the cluster %s: removing it needs the admin grant of its namespace on this cluster, or root", run.ShortID(), run.Path, run.ClusterName)
	}
	if err := pruner.Delete(reqCtx, []backup.Run{run}); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Delete", "%s", err)
	}
	log.Infof("store %s: removed the run %s of %s, from %s (protected=%v)", s.Ref(), run.ID, run.Path, run.ClusterName, run.Protected)
	return ctx.NoContent(http.StatusNoContent)
}
