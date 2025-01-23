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

func (a *DaemonAPI) GetObjectConfigGet(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetObjectConfigGetParams) error {
	if _, err := assertGuest(ctx, namespace); err != nil {
		return err
	}
	log := LogHandler(ctx, "GetObjectConfigGet")
	r := api.KeywordList{
		Kind:  "KeywordList",
		Items: make(api.KeywordItems, 0),
	}
	if params.Kw == nil {
		return ctx.JSON(http.StatusOK, r)
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
		for _, s := range *params.Kw {
			kw := key.Parse(s)
			item := api.KeywordItem{
				Kind: "KeywordItem",
				Meta: api.KeywordMeta{
					Node:        a.localhost,
					Object:      p.String(),
					Keyword:     s,
					IsEvaluated: isEvaluated,
					EvaluatedAs: evaluatedAs,
				},
			}

			if isEvaluated {
				if i, err := oc.EvalAs(kw, evaluatedAs); err != nil {
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
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.GetObjectConfigGetWithResponse(ctx.Request().Context(), namespace, kind, name, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return ctx.JSON(http.StatusOK, r)
}
