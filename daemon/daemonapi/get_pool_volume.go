package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetPoolVolumes(ctx echo.Context, params api.GetPoolVolumesParams) error {
	if _, err := assertRoot(ctx); err != nil {
		return err
	}
	l := getPoolVolumes(params.Name)
	return ctx.JSON(http.StatusOK, api.PoolVolumeList{Kind: "PoolVolumeList", Items: l})
}

func getPoolVolumes(name *string) api.PoolVolumeItems {
	volNames := make(map[string]any)
	poolNames := make(map[string]any)
	for _, e := range pool.StatusData.GetAll() {
		poolNames[e.Name] = nil
	}

	l := make(api.PoolVolumeItems, 0)
	for _, instConfig := range instance.ConfigData.GetAll() {
		var (
			poolOk   bool
			poolName string
			size     int64
		)
		if instConfig.Path.Kind != naming.KindVol {
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
		if name != nil && *name != poolName {
			continue
		}
		if instConfig.Value.Size != nil {
			size = *instConfig.Value.Size
		}
		l = append(l, api.PoolVolume{
			Path:     p,
			Children: instConfig.Value.Children.Strings(),
			IsOrphan: !poolOk,
			Pool:     poolName,
			Size:     size,
		})
	}
	return l
}
