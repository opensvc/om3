package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
)

func (a *DaemonAPI) GetObjectPaths(ctx echo.Context, params api.GetObjectPathsParams) error {
	log := LogHandler(ctx, "GetObjectPaths")
	log.Debugf("starting")
	paths := object.StatusData.GetPaths()
	selection := objectselector.New(
		params.Path,
		objectselector.WithPaths(paths),
		objectselector.WithLocal(true),
	)
	matchedPaths, err := selection.Expand()
	if err != nil {
		log.Errorf("expand selection from param selector %s: %s", params.Path, err)
		return JSONProblem(ctx, http.StatusInternalServerError, "Server error", "expand selection")
	}
	result := api.ObjectPaths{}
	hasRoot := grantsFromContext(ctx).HasRole(rbac.RoleRoot)
	userGrants := grantsFromContext(ctx)

	for _, path := range matchedPaths {
		if !hasRoot && !hasRoleGuestOn(userGrants, path.Namespace) {
			continue
		}
		result = append(result, path.String())
	}
	return ctx.JSON(http.StatusOK, result)
}
