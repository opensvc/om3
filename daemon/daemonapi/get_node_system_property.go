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

func (a *DaemonAPI) GetNodeSystemProperty(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.getLocalNodeSystemProperty(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSystemProperty(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodeSystemProperty(ctx echo.Context) error {
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
	items := make(api.PropertyItems, 0)

	for _, p := range data.Values() {

		value := p.Value
		if value == nil {
			value = ""
		}

		items = append(items, api.PropertyItem{
			Kind: "PropertyItem",
			Data: api.Property{
				Name:   p.Name,
				Source: p.Source,
				Title:  p.Title,
				Error:  p.Error,
				Value:  value,
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		})
	}

	return ctx.JSON(http.StatusOK, api.PropertyList{Kind: "PropertyList", Items: items})
}
