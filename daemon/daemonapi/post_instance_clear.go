package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostInstanceClear(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	if a.localhost == nodename {
		return a.postLocalInstanceClear(ctx, namespace, kind, name)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		return a.postPeerInstanceClear(ctx, nodename, namespace, kind, name)
	}
}

func (a *DaemonApi) postPeerInstanceClear(ctx echo.Context, nodename, namespace string, kind naming.Kind, name string) error {
	c, err := newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	if resp, err := c.PostInstanceClearWithResponse(ctx.Request().Context(), nodename, namespace, kind, name); err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

func (a *DaemonApi) postLocalInstanceClear(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	state := instance.MonitorStateIdle
	instMonitor := instance.MonitorUpdate{
		State: &state,
	}
	msg := msgbus.SetInstanceMonitor{
		Path:  p,
		Node:  a.localhost,
		Value: instMonitor,
	}
	a.EventBus.Pub(&msg, pubsub.Label{"path", p.String()}, labelApi)
	return ctx.JSON(http.StatusOK, nil)
}
