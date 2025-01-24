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

func (a *DaemonAPI) GetNodeSystemPackage(ctx echo.Context, nodename api.InPathNodeName) error {
	if _, err := assertRoot(ctx); err != nil {
		return err
	}
	if a.localhost == nodename {
		return a.getLocalNodeSystemPackage(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSystemPackage(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodeSystemPackage(ctx echo.Context) error {
	n, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New node", "%s", err)
	}
	data, err := n.LoadPkg()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return JSONProblemf(ctx, http.StatusNotFound, "Load package cache", "waiting for cached value: %s", err)
		} else {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Load package cache", "%s", err)
		}
	}
	items := make(api.PackageItems, len(data))
	for i := 0; i < len(data); i++ {
		items[i] = api.PackageItem{
			Kind: "PackageItem",
			Data: api.Package{
				Version:     data[i].Version,
				Name:        data[i].Name,
				Arch:        data[i].Arch,
				Type:        data[i].Type,
				InstalledAt: data[i].InstalledAt,
				Sig:         data[i].Sig,
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		}
	}

	return ctx.JSON(http.StatusOK, api.PackageList{Kind: "PackageList", Items: items})
}
