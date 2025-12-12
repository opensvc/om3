package daemonapi

import (
	"errors"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetNodeSystemHardware(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.getLocalNodeSystemHardware(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSystemHardware(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodeSystemHardware(ctx echo.Context) error {
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	data, err := n.LoadSystem()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return JSONProblemf(ctx, http.StatusNotFound, "Load system cache", "waiting for cached value: %s", err)
		} else {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Load system cache", "%s", err)
		}
	}
	items := make(api.HardwareItems, len(data.Hardware))
	for i := 0; i < len(data.Hardware); i++ {
		items[i] = api.HardwareItem{
			Kind: "HardwareItem",
			Data: api.Hardware{
				Class:       data.Hardware[i].Class,
				Type:        data.Hardware[i].Type,
				Driver:      data.Hardware[i].Driver,
				Path:        data.Hardware[i].Path,
				Description: data.Hardware[i].Description,
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		}
	}

	return ctx.JSON(http.StatusOK, api.HardwareList{Kind: "HardwareList", Items: items})
}
