package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/pool"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetPools(ctx echo.Context, params api.GetPoolsParams) error {
	if _, err := assertRoot(ctx); err != nil {
		return err
	}
	var items api.PoolItems
	for _, e := range pool.StatusData.GetAll() {
		if params.Name != nil && *params.Name != e.Name {
			continue
		}
		stat := *e.Value
		item := api.Pool{
			Capabilities: append([]string{}, stat.Capabilities...),
			Free:         stat.Free,
			Head:         stat.Head,
			Name:         stat.Name,
			Size:         stat.Size,
			Type:         stat.Type,
			Used:         stat.Used,
			VolumeCount:  len(getPoolVolumes(&e.Name)),
		}
		if len(stat.Errors) > 0 {
			l := append([]string{}, stat.Errors...)
			item.Errors = &l
		}
		items = append(items, item)
	}
	return ctx.JSON(http.StatusOK, api.PoolList{Kind: "PoolList", Items: items})
}
