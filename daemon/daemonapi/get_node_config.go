package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/configkeywords"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetNodeConfig(ctx echo.Context, nodename string, params api.GetNodeConfigParams) error {
	//log := LogHandler(ctx, "GetNodeConfig")

	if v, err := assertRoot(ctx); !v {
		return err
	}

	r := api.KeywordList{
		Kind:  "KeywordList",
		Items: make(api.KeywordItems, 0),
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		var (
			isEvaluated bool
			impersonate string
		)
		if params.Evaluate != nil {
			isEvaluated = *params.Evaluate
		}
		if params.Impersonate != nil {
			impersonate = *params.Impersonate
		}
		if !isEvaluated && impersonate != "" {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad request", "impersonate can only be specified with evaluate=true")
		}
		o := configkeywords.Options{
			Evaluate:    isEvaluated,
			Impersonate: impersonate,
		}
		if params.Kw != nil {
			o.Keywords = *params.Kw
		}
		items, err := configkeywords.Node(nodename, o)
		switch {
		case errors.Is(err, xconfig.ErrNoKeyword):
			return JSONProblemf(ctx, http.StatusBadRequest, "EvalAs", "%s", err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "EvalAs", "%s", err)
		}
		r.Items = items
		return ctx.JSON(http.StatusOK, r)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.GetNodeConfigWithResponse(ctx.Request().Context(), nodename, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return ctx.JSON(http.StatusOK, r)
}
