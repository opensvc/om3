package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func (a *DaemonAPI) PostNodeDRBDSecondary(ctx echo.Context, nodename api.InPathNodeName, params api.PostNodeDRBDSecondaryParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.postLocalDRBDSecondary(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeDRBDSecondary(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) postLocalDRBDSecondary(ctx echo.Context, params api.PostNodeDRBDSecondaryParams) error {
	c := ctx.Request().Context()
	log := LogHandler(ctx, "PostNodeDRBDSecondary")
	p, ok := a.objPathHandlingDRBDRes(params.Name)
	if !ok {
		return JSONProblemf(ctx, http.StatusForbidden, "No resource found managing the drbd resource", "%s", params.Name)
	}
	log = naming.LogWithPath(log, p)
	res, err := newDrbd(c, params.Name, log)
	if err != nil {
		log.Warnf("can't create internal drbd object: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "New drbd", "%s", err)
	}
	if err := res.Secondary(c); err != nil {
		log.Warnf("unexpected error during primary: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "Secondary", "%s", err)
	}
	return ctx.NoContent(http.StatusNoContent)
}
