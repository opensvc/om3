package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/kind"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonApi) GetPoolVolume(ctx echo.Context, params api.GetPoolVolumeParams) error {
	n, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Server error", "Failed to allocate a new object.Node: %s", err)
	}
	volNames := make(map[string]any)
	poolNames := make(map[string]any)
	for _, name := range n.ListPools() {
		poolNames[name] = nil
	}
	l := make(api.PoolVolumeArray, 0)
	for _, instConfig := range instance.ConfigData.GetAll() {
		var (
			poolOk   bool
			poolName string
			size     int64
		)
		if instConfig.Path.Kind != kind.Vol {
			continue
		}
		p := instConfig.Path.String()
		if _, ok := volNames[p]; ok {
			continue
		} else {
			volNames[p] = nil
		}
		if instConfig.Value.Pool != nil {
			poolName = *instConfig.Value.Pool
			_, poolOk = poolNames[poolName]
		}
		if params.Name != nil && *params.Name != poolName {
			continue
		}
		if instConfig.Value.Size != nil {
			size = *instConfig.Value.Size
		}
		l = append(l, api.PoolVolume{
			Path:     p,
			Children: instConfig.Value.Children.StringSlice(),
			IsOrphan: !poolOk,
			Pool:     poolName,
			Size:     size,
		})
	}
	return ctx.JSON(http.StatusOK, l)
}
