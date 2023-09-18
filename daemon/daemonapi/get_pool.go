package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetPool(ctx echo.Context, params api.GetPoolParams) error {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "Failed to allocate a new object.Node: %s", err)
	}
	var l pool.StatusList
	if params.Name != nil {
		l = n.ShowPoolsByName(*params.Name)
	} else {
		l = n.ShowPools()
	}
	return ctx.JSON(http.StatusOK, l)
}
