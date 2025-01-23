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

func (a *DaemonAPI) GetNodeSystemSANPath(ctx echo.Context, nodename api.InPathNodeName) error {
	if _, err := assertRoot(ctx); err != nil {
		return err
	}
	if a.localhost == nodename {
		return a.getLocalNodeSystemSANPath(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSystemSANPath(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodeSystemSANPath(ctx echo.Context) error {
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
	items := make(api.SANPathItems, len(data.Targets))
	for i := 0; i < len(data.Targets); i++ {
		items[i] = api.SANPath{
			Initiator: api.SANPathInitiator{
				Name: data.Targets[i].Initiator.Name,
				Type: data.Targets[i].Initiator.Type,
			},
			Target: api.SANPathTarget{
				Name: data.Targets[i].Target.Name,
				Type: data.Targets[i].Target.Type,
			},
		}
	}

	return ctx.JSON(http.StatusOK, api.SANPathList{Kind: "SANPathList", Items: items})
}
