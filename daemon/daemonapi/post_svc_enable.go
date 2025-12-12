package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/actioncontext"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostSvcEnable(ctx echo.Context, namespace string, name string, params api.PostSvcEnableParams) error {
	log := LogHandler(ctx, "PostSvcEnable")

	if v, err := assertAdmin(ctx, namespace); !v {
		return err
	}

	p, err := naming.NewPath(namespace, naming.KindSvc, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)

	instanceConfigData := instance.ConfigData.GetByPath(p)

	if _, ok := instanceConfigData[a.localhost]; ok {
		c := ctx.Request().Context()
		if params.Subset != nil {
			c = actioncontext.WithSubset(c, *params.Subset)
		}
		if params.Tag != nil {
			c = actioncontext.WithTag(c, *params.Tag)
		}
		if params.Rid != nil {
			c = actioncontext.WithRID(c, *params.Rid)
		}
		oc, err := object.NewSvc(p)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "NewSvc", "%s", err)
		}
		if err := oc.Enable(c); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Enable", "%s", err)
		}
		return ctx.NoContent(http.StatusNoContent)
	}

	for nodename := range instanceConfigData {
		c, err := a.newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.PostSvcEnableWithResponse(ctx.Request().Context(), namespace, name, &params); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}

	return nil
}
