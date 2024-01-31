package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
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
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		c, err := newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.PostNodeConfigUpdateWithResponse(ctx.Request().Context(), nodename, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
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
	if err := oc.Config().PrepareUpdate(deletes, unsets, sets); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Update config", "%s", err)
	}

	alerts, err := oc.Config().Validate()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Validate config", "%s", err)
	}
	if alerts.HasError() {
		return JSONProblemf(ctx, http.StatusBadRequest, "Validate config", "%s", alerts)
	}

	err = oc.Config().CommitInvalid()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Commit", "%s", err)
	}

	return ctx.NoContent(http.StatusNoContent)
}
