package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/key"
)

func (a *DaemonAPI) GetObjectConfig(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetObjectConfigParams) error {
	if v, err := assertGuest(ctx, namespace); !v {
		return err
	}
	log := LogHandler(ctx, "GetObjectConfig")
	r := api.KeywordList{
		Kind:  "KeywordList",
		Items: make(api.KeywordItems, 0),
	}

	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	instanceConfigData := instance.ConfigData.GetByPath(p)

	if _, ok := instanceConfigData[a.localhost]; ok {
		oc, err := object.NewCore(p)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewCore", "%s", err)
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
				Object:  p.String(),
				Keyword: k.String(),
			}
			if s, err := conf.GetStrict(k); err != nil {
				item.Value = ""
			} else {
				item.Value = s
			}

			if isEvaluated {
				if i, err := oc.EvalAs(k, evaluatedAs); err != nil {
					return JSONProblemf(ctx, http.StatusInternalServerError, "EvalAs", "%s", err)
				} else {
					item.Evaluated = &i
					item.EvaluatedAs = evaluatedAs
				}
			}
			r.Items = append(r.Items, item)
		}
		return ctx.JSON(http.StatusOK, r)
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.GetObjectConfigWithResponse(ctx.Request().Context(), namespace, kind, name, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return ctx.JSON(http.StatusOK, r)
}
