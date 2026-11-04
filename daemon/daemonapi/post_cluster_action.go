package daemonapi

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/clusternode"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

func (a *DaemonAPI) PostClusterActionAbort(ctx echo.Context) error {
	return a.PostClusterAction(ctx, node.MonitorGlobalExpectAborted)
}

func (a *DaemonAPI) PostClusterActionFreeze(ctx echo.Context) error {
	return a.PostClusterAction(ctx, node.MonitorGlobalExpectFrozen)
}

func (a *DaemonAPI) PostClusterActionUnfreeze(ctx echo.Context) error {
	return a.PostClusterAction(ctx, node.MonitorGlobalExpectUnfrozen)
}

// PostClusterActionUpdatePkg asks the nodes to upgrade their opensvc
// packages, one at a time. Every node of the cluster has to be heard first:
// the nodes take turns, each waiting for the whole cluster to be live and
// idle, so one not heard would hold the upgrade from the start, and an
// upgrade run with a node down is not one to start by itself.
func (a *DaemonAPI) PostClusterActionUpdatePkg(ctx echo.Context) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	for _, nodename := range clusternode.Get() {
		if node.MonitorData.GetByNode(nodename) == nil {
			return JSONProblemf(ctx, http.StatusConflict, "Node not live",
				"node %s is not live: the nodes upgrade their packages in turn, each waiting for every node of the cluster to be live and idle", nodename)
		}
	}
	return a.PostClusterAction(ctx, node.MonitorGlobalExpectUpgraded)
}

func (a *DaemonAPI) PostClusterAction(eCtx echo.Context, globalExpect node.MonitorGlobalExpect) error {
	if v, err := assertRoot(eCtx); !v {
		return err
	}

	if mon := node.MonitorData.GetByNode(a.localhost); mon == nil {
		return JSONProblemf(eCtx, http.StatusNotFound, "Not found", "node monitor not found: %s", a.localhost)
	}

	ctx, cancel := context.WithTimeout(eCtx.Request().Context(), 300*time.Millisecond)
	defer cancel()

	value := node.MonitorUpdate{
		GlobalExpect:             &globalExpect,
		CandidateOrchestrationID: uuid.New(),
	}
	msg, err := msgbus.NewSetNodeMonitorWithErr(ctx, a.localhost, value)

	a.Bus.Pub(msg, a.LabelLocalhost, labelOriginAPI)
	return JSONFromSetNodeMonitorError(eCtx, a.localhost, &value, err.Receive())
}
