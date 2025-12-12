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

func (a *DaemonAPI) GetNodeSystemIPAddress(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.getLocalNodeSystemIPAddress(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSystemIPAddress(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodeSystemIPAddress(ctx echo.Context) error {
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
	items := make(api.IPAddressItems, 0)
	for key, value := range data.LAN {
		for i := 0; i < len(value); i++ {
			items = append(items, api.IPAddressItem{
				Kind: "IPAddressItem",
				Data: api.IPAddress{
					Mac:            key,
					Address:        value[i].Address,
					FlagDeprecated: value[i].FlagDeprecated,
					Intf:           value[i].Intf,
					Mask:           value[i].Mask,
					Type:           value[i].Type,
				},
				Meta: api.NodeMeta{
					Node: a.localhost,
				},
			})
		}
	}

	return ctx.JSON(http.StatusOK, api.IPAddressList{Kind: "IPAddressList", Items: items})
}
