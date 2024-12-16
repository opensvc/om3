package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/util/key"
)

func (a *DaemonAPI) GetNodeConfigGet(ctx echo.Context, nodename string, params api.GetNodeConfigGetParams) error {
	//log := LogHandler(ctx, "GetNodeConfigGet")

	if v, err := assertGrant(ctx, rbac.GrantRoot); !v {
		return err
	}

	r := api.KeywordList{
		Kind:  "KeywordList",
		Items: make(api.KeywordItems, 0),
	}
	if params.Kw == nil {
		return ctx.JSON(http.StatusOK, r)
	}

	if nodename == a.localhost {
		oc, err := object.NewNode()
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewNode", "%s", err)
		}
		var (
			isEvaluated bool
			evaluatedAs string
		)
		if params.Evaluate != nil {
			isEvaluated = *params.Evaluate
		}
		if params.Impersonate != nil {
			evaluatedAs = *params.Impersonate
		} else if isEvaluated {
			evaluatedAs = a.localhost
		}
		if !isEvaluated && evaluatedAs != "" {
			return JSONProblemf(ctx, http.StatusBadRequest, "Bad request", "impersonate can only be specified with evaluate=true")
		}
		for _, s := range *params.Kw {
			kw := key.Parse(s)
			item := api.KeywordItem{
				Kind: "KeywordItem",
				Meta: api.KeywordMeta{
					Node:        a.localhost,
					Keyword:     s,
					IsEvaluated: isEvaluated,
					EvaluatedAs: evaluatedAs,
				},
			}

			if isEvaluated {
				if i, err := oc.MergedConfig().EvalAs(kw, evaluatedAs); err != nil {
					return JSONProblemf(ctx, http.StatusInternalServerError, "EvalAs", "%s", err)
				} else {
					item.Data.Value = i
					r.Items = append(r.Items, item)
				}
			} else {
				i := oc.Config().Get(kw)
				item.Data.Value = i
				r.Items = append(r.Items, item)
			}
		}
		return ctx.JSON(http.StatusOK, r)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.GetNodeConfigGetWithResponse(ctx.Request().Context(), nodename, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return ctx.JSON(http.StatusOK, r)
}
