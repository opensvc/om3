package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/util/key"
)

func (a *DaemonAPI) PostNodeConfigUpdate(ctx echo.Context, nodename string, params api.PostNodeConfigUpdateParams) error {
	//log := LogHandler(ctx, "PostObjectConfigUpdate")

	if v, err := assertGrant(ctx, rbac.GrantRoot); !v {
		return err
	}
	if nodename == a.localhost {
		return a.postLocalNodeConfigUpdate(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeConfigUpdate(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) postLocalNodeConfigUpdate(ctx echo.Context, params api.PostNodeConfigUpdateParams) error {
	sets := make(keyop.L, 0)
	unsets := make(key.L, 0)
	deletes := make([]string, 0)

	if params.Set != nil {
		sets = keyop.ParseOps(*params.Set)
	}
	if params.Unset != nil {
		unsets = key.ParseStrings(*params.Unset)
	}
	if params.Delete != nil {
		deletes = *params.Delete
	}
	if len(sets)+len(unsets)+len(deletes) == 0 {
		return JSONProblemf(ctx, http.StatusBadRequest, "No valid update requested", "")
	}

	oc, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "NewNode", "%s", err)
	}
	isChanged, err := oc.Config().UpdateAndReportIsChanged(deletes, unsets, sets)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Config Update", "%s", err)
	}

	item := api.IsChangedItem{
		Data: api.IsChanged{
			Ischanged: isChanged,
		},
		Meta: api.NodeMeta{
			Node: a.localhost,
		},
	}

	return ctx.JSON(http.StatusOK, item)
}
