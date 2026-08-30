package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/key"
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

		// A key selection is a user input, so a key it can not evaluate is an
		// error. The whole config is not: it can hold keys no keyword declares,
		// and failing the request on the first one would hide all the others.
		isWholeConfig := params.Kw == nil

		if isWholeConfig {
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
				i, err := oc.MergedConfig().EvalAs(k, evaluatedAs)
				switch {
				case err != nil && isWholeConfig:
					s := err.Error()
					item.Error = &s
					item.EvaluatedAs = evaluatedAs
				case errors.Is(err, xconfig.ErrNoKeyword):
					return JSONProblemf(ctx, http.StatusBadRequest, "EvalAs", "%s", err)
				case err != nil:
					return JSONProblemf(ctx, http.StatusInternalServerError, "EvalAs", "%s", err)
				default:
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
