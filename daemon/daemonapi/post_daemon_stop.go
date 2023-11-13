package daemonapi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostDaemonStop(ctx echo.Context) error {
	log := LogHandler(ctx, "PostDaemonStop")
	log.Debugf("starting")

	a.announceNodeState(log, node.MonitorStateMaintenance)

	a.EventBus.Pub(&msgbus.DaemonCtl{Component: "daemon", Action: "stop"},
		pubsub.Label{"id", "daemon"}, labelApi, a.LabelNode)
	return JSONProblem(ctx, http.StatusOK, "announce maintenance state and ask daemon to stop", "")
}
