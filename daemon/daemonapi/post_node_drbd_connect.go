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

func (a *DaemonAPI) PostNodeDRBDConnect(ctx echo.Context, nodename api.InPathNodeName, params api.PostNodeDRBDConnectParams) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if a.localhost == nodename {
		return a.postLocalDRBDConnect(ctx, params)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostNodeDRBDConnect(ctx.Request().Context(), nodename, &params)
	})
}

func (a *DaemonAPI) objPathHandlingDRBDRes(res string) (naming.Path, bool) {
	for p, instanceStatus := range instance.StatusData.GetByNode(a.localhost) {
		for _, resourceStatus := range instanceStatus.Resources {
			if v, ok := resourceStatus.Info["res"]; ok && v == res {
				return p, true
			}
		}
	}
	return naming.Path{}, false
}

func (a *DaemonAPI) postLocalDRBDConnect(ctx echo.Context, params api.PostNodeDRBDConnectParams) error {
	c := ctx.Request().Context()
	log := LogHandler(ctx, "PostNodeDRBDConnect")
	p, ok := a.objPathHandlingDRBDRes(params.Name)
	if !ok {
		return JSONProblemf(ctx, http.StatusForbidden, "No object found managing the drbd resource", "%s", params.Name)
	}
	log = naming.LogWithPath(log, p)
	res, err := newDrbd(c, params.Name, log)
	if err != nil {
		log.Warnf("can't create internal drbd object: %s", err)
		return JSONProblemf(ctx, http.StatusInternalServerError, "New drbd", "%s", err)
	}
	var nodeID string
	if params.NodeId != nil {
		nodeID = *params.NodeId
	}
	if err := res.TryStartConnection(c, nodeID); err != nil {
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
