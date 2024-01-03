package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostDaemonStop(ctx echo.Context, nodename string) error {
	if nodename == a.localhost {
		return a.localPostDaemonStop(ctx)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	c, err := client.New(client.WithURL(nodename))
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	resp, err := c.PostDaemonStopWithResponse(ctx.Request().Context(), nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil

}

func (a *DaemonApi) localPostDaemonStop(ctx echo.Context) error {
	log := LogHandler(ctx, "PostDaemonStop")
	log.Debugf("starting")

	a.announceNodeState(log, node.MonitorStateMaintenance)

	a.EventBus.Pub(&msgbus.DaemonCtl{Component: "daemon", Action: "stop"},
		pubsub.Label{"id", "daemon"}, labelApi, a.LabelNode)
	return JSONProblem(ctx, http.StatusOK, "announce maintenance state and ask daemon to stop", "")
}
