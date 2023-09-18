package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetPool(ctx echo.Context, params api.GetPoolParams) error {
	var l pool.StatusList
	for _, e := range pool.StatusData.GetAll() {
		if params.Name != nil && *params.Name != e.Name {
			continue
		}
		stat := *e.Value
		stat.VolumeCount = len(getPoolVolumes(&e.Name))
		l = append(l, stat)
	}
	return ctx.JSON(http.StatusOK, l)
}
