package daemonapi

import (
	"errors"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
)

func (a *DaemonAPI) GetNodeSystemDisk(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if a.localhost == nodename {
		return a.getLocalNodeSystemDisk(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSystemDisk(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodeSystemDisk(ctx echo.Context) error {
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	l, err := n.LoadDisks()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return JSONProblemf(ctx, http.StatusNotFound, "Load disk cache", "waiting for cached value: %s", err)
		} else {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Load disk cache", "%s", err)
		}
	}
	items := make(api.DiskItems, len(l))
	for i := 0; i < len(l); i++ {
		regions := make([]api.Region, len(l[i].Regions))

		for j := 0; j < len(l[i].Regions); j++ {
			regions[j] = api.Region{
				ID:      l[i].Regions[j].ID,
				Group:   l[i].Regions[j].Group,
				Object:  l[i].Regions[j].Object,
				Devpath: l[i].Regions[j].DevPath,
				Size:    l[i].Regions[j].Size,
			}
		}

		items[i] = api.DiskItem{
			Kind: "DiskItem",
			Data: api.Disk{
				ID:      l[i].ID,
				Size:    l[i].Size,
				Vendor:  l[i].Vendor,
				Model:   l[i].Model,
				Type:    l[i].Type,
				Devpath: l[i].DevPath,
				Regions: regions,
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		}
	}

	return ctx.JSON(http.StatusOK, api.DiskList{Kind: "DiskList", Items: items})
}
