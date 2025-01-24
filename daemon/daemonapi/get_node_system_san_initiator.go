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

func (a *DaemonAPI) GetNodeSystemSANInitiator(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if a.localhost == nodename {
		return a.getLocalNodeSystemSANInitiator(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSystemSANInitiator(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodeSystemSANInitiator(ctx echo.Context) error {
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
	items := make(api.SANPathInitiatorItems, len(data.HBA))
	for i := 0; i < len(data.HBA); i++ {
		items[i] = api.SANPathInitiatorItem{
			Kind: "SANPathInitiatorItem",
			Data: api.SANPathInitiator{
				Name: data.HBA[i].Name,
				Type: data.HBA[i].Type,
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		}
	}

	return ctx.JSON(http.StatusOK, api.SANPathInitiatorList{Kind: "SANPathInitiatorList", Items: items})
}
