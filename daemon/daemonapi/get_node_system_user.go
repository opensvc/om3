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

func (a *DaemonAPI) GetNodeSystemUser(ctx echo.Context, nodename api.InPathNodeName) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.getLocalNodeSystemUser(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.GetNodeSystemUser(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) getLocalNodeSystemUser(ctx echo.Context) error {
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
	items := make(api.UserItems, len(data.UIDS))
	for i := 0; i < len(data.UIDS); i++ {
		items[i] = api.UserItem{
			Kind: "UserItem",
			Data: api.User{
				ID:   data.UIDS[i].ID,
				Name: data.UIDS[i].Name,
			},
			Meta: api.NodeMeta{
				Node: a.localhost,
			},
		}
	}

	return ctx.JSON(http.StatusOK, api.UserList{Kind: "UserList", Items: items})
}
