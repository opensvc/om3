package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/key"
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
		conf := oc.Config()
		var keys key.L
		if params.Kw == nil {
			keys = conf.KeyList()
		} else {
			for _, s := range *params.Kw {
				keys = append(keys, key.Parse(s))
			}
		}
		for _, k := range keys {
			item := api.KeywordItem{
				Node:    nodename,
				Keyword: k.String(),
			}
			if s, err := conf.GetStrict(k); err != nil {
				continue
			} else {
				item.Value = s
			}
			if isEvaluated {
				if i, err := oc.MergedConfig().EvalAs(k, evaluatedAs); err != nil {
					return JSONProblemf(ctx, http.StatusInternalServerError, "EvalAs", "%s", err)
				} else {
					item.Evaluated = &i
					item.EvaluatedAs = evaluatedAs
				}
			}
			r.Items = append(r.Items, item)
		}
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
