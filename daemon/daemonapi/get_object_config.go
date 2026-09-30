package daemonapi

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/configkeywords"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) GetObjectConfig(ctx echo.Context, namespace string, kind naming.Kind, name string, params api.GetObjectConfigParams) error {
	log := LogHandler(ctx, "GetObjectConfig")
	r := api.KeywordList{
		Kind:  "KeywordList",
		Items: make(api.KeywordItems, 0),
	}

	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	redact, ok, err := configReadAccess(ctx, p)
	if !ok {
		return err
	}
	log = naming.LogWithPath(log, p)

	instanceConfigData := instance.ConfigData.GetByPath(p)

	if _, ok := instanceConfigData[a.localhost]; ok {
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
			Redact:      redact,
		}
		if params.Kw != nil {
			o.Keywords = *params.Kw
		}
		items, err := configkeywords.Object(p, o)
		switch {
		case errors.Is(err, xconfig.ErrNoKeyword):
			return JSONProblemf(ctx, http.StatusBadRequest, "EvalAs", "%s", err)
		case err != nil:
			return JSONProblemf(ctx, http.StatusInternalServerError, "EvalAs", "%s", err)
		}
		r.Items = items
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
