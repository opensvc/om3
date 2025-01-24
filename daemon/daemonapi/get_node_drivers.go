package daemonapi

import (
	"net/http"
	"sort"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodeDriver(ctx echo.Context, nodename api.InPathNodeName) error {
	if _, err := assertRoot(ctx); err != nil {
		return err
	}
	if a.localhost == nodename {
		return a.getLocalNodeDriver(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeDriver(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodeDriver(ctx echo.Context) error {
	ids := driver.List()
	sort.Sort(ids)

	items := make(api.DriverItems, len(ids))
	for i, id := range ids {
		items[i] = api.DriverItem{
			Kind: "DriverItem",
			Data: api.Driver{
				Name: id.String(),
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		}
	}

	return ctx.JSON(http.StatusOK, api.DriverList{Kind: "DriverList", Items: items})
}
