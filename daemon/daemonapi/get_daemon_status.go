package daemonapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

type (
	daemonRefresh struct {
		*sync.Mutex
		updated time.Time
	}
)

const (
	// daemonRefreshInterval defines the maximum duration before next DaemonRefresh
	daemonRefreshInterval = 2 * time.Second
)

var (
	subRefreshed = daemonRefresh{Mutex: &sync.Mutex{}}
)

// GetDaemonStatus returns daemon data status
//
// Serve 2s cached data.
func (a *DaemonAPI) GetClusterStatus(ctx echo.Context, params api.GetClusterStatusParams) error {
	// Require at least "guest" on any namespace.
	if v, err := assertRole(ctx, rbac.RoleGuest, rbac.RoleOperator, rbac.RoleAdmin, rbac.RoleRoot, rbac.RoleJoin, rbac.RoleLeave); !v {
		return err
	}
	now := time.Now()
	subRefreshed.Lock()
	if now.After(subRefreshed.updated.Add(daemonRefreshInterval)) {
		a.Daemondata.DaemonRefresh()
		subRefreshed.updated = now
	}
	subRefreshed.Unlock()

	// A caller with a cluster wide role and no selector has nothing
	// filtered out of its answer, and gets the dataset as the daemondata
	// goroutine marshalled it. That is the whole of it: no deep copy, and
	// no second serialization here. The filtered paths below still need a
	// struct to filter, so they take the copy.
	userGrants := grantsFromContext(ctx)
	// The guest:ns1 grant is sufficient to see all objects in ns1, so a
	// grant on no namespace in particular is what means "everything".
	seesEverything := userGrants.HasRoleOn("", rbac.RoleRoot, rbac.RoleAdmin, rbac.RoleOperator, rbac.RoleGuest)
	if params.Selector == nil && params.Namespace == nil && seesEverything {
		b, err := a.Daemondata.ClusterDataJSON()
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Internal Server Error", "marshal cluster data: %s", err)
		}
		return ctx.JSONBlob(http.StatusOK, b)
	}

	status := a.Daemondata.ClusterData()
	if status == nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Internal Server Error", "get cluster data")
	}

	// The filters below return a new dataset instead of purging this one:
	// ClusterData is behind a singleflight, so concurrent callers share the
	// returned pointer and must not modify it.

	// Explicit object selector filtering
	if params.Selector != nil {
		status = status.WithSelector(*params.Selector)
	}

	// Explicit namespace filtering
	if params.Namespace != nil {
		status = status.WithNamespace(*params.Namespace)
	}

	// RBAC namespace filtering
	if !seesEverything {
		// Filter out all objects from namespaces the user has no role for.
		status = status.WithNamespace(userGrants.Namespaces()...)
	}

	return ctx.JSON(http.StatusOK, status)
}
