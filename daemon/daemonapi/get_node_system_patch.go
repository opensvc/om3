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

func (a *DaemonAPI) GetNodeSystemPatch(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.getLocalNodeSystemPatch(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSystemPatch(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodeSystemPatch(ctx echo.Context) error {
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	data, err := n.LoadPatch()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return JSONProblemf(ctx, http.StatusNotFound, "Load patch cache", "waiting for cached value: %s", err)
		} else {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Load patch cache", "%s", err)
		}
	}
	items := make(api.PatchItems, 0)

	for i := 0; i < len(data); i++ {

		items = append(items, api.PatchItem{
			Kind: "PatchItem",
			Data: api.Patch{
				Number:      data[i].Number,
				Revision:    data[i].Revision,
				InstalledAt: data[i].InstalledAt,
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		})
	}

	return ctx.JSON(http.StatusOK, api.PatchList{Kind: "PatchList", Items: items})
}
