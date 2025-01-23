package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/capabilities"
)

func (a *DaemonAPI) GetNodeCapabilities(ctx echo.Context, nodename string) error {
	if _, err := assertRoot(ctx); err != nil {
		return err
	}
	if a.localhost == nodename {
		return a.getLocalCapabilities(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeCapabilities(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalCapabilities(ctx echo.Context) error {
	caps, err := capabilities.Load()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Load capabilities", "%s", err)
	}
	resp := api.CapabilityList{
		Kind: "CapabilityList",
	}
	for _, e := range caps {
		item := api.CapabilityItem{
			Kind: "CapabilityItem",
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
			Data: api.Capability{
				Name: e,
			},
		}
		resp.Items = append(resp.Items, item)
	}
	return ctx.JSON(http.StatusOK, resp)
}
