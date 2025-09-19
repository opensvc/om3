package daemonapi

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/drbd"
	"github.com/opensvc/om3/util/plog"
)

func (a *DaemonAPI) PostNodeDRBDConnect(ctx echo.Context, nodename api.InPathNodeName, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.PostNodeDRBDConnectParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.postLocalDRBDConnect(ctx, namespace, kind, name, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeDRBDConnect(ctx.Request().Context(), nodename, namespace, kind, name, &params)
	})
}

func (a *DaemonAPI) postLocalDRBDConnect(ctx echo.Context, namespace api.InPathNamespace, kind api.InPathKind, name api.InPathName, params api.PostNodeDRBDConnectParams) error {
	var res *drbd.T
	c := ctx.Request().Context()
	log := LogHandler(ctx, "PostNodeDRBDConnect")
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	log = naming.LogWithPath(log, p)
	if instance.ConfigData.GetByPathAndNode(p, a.localhost) == nil {
		log.Infof("skipped: no local instance")
		return ctx.NoContent(http.StatusNoContent)
	}
	if res, err = newDrbd(c, params.Name, log); err != nil {
		log.Warnf("can't create internal drbd object: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "New drbd", "%s", err)
	}
	if err := res.TryStartConnection(c); err != nil {
		log.Warnf("unexpected error during try start connection: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "TryStartConnection", "%s", err)
	}
	return ctx.NoContent(http.StatusNoContent)
}

func newDrbd(ctx context.Context, res string, log *plog.Logger) (*drbd.T, error) {
	d := drbd.New(
		res,
		drbd.WithLogger(log),
	)
	return d, d.ModProbe(ctx)
}
