package daemonapi

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
)

func (a *DaemonAPI) PostClusterActionAbort(ctx echo.Context) error {
	return a.PostClusterAction(ctx, node.MonitorGlobalExpectAborted)
}

func (a *DaemonAPI) PostClusterActionFreeze(ctx echo.Context) error {
	return a.PostClusterAction(ctx, node.MonitorGlobalExpectFrozen)
}

func (a *DaemonAPI) PostClusterActionUnfreeze(ctx echo.Context) error {
	return a.PostClusterAction(ctx, node.MonitorGlobalExpectThawed)
}

func (a *DaemonAPI) PostClusterAction(eCtx echo.Context, globalExpect node.MonitorGlobalExpect) error {
	if mon := node.MonitorData.Get(a.localhost); mon == nil {
		return JSONProblemf(eCtx, http.StatusNotFound, "Not found", "node monitor not found: %s", a.localhost)
	}

	ctx, cancel := context.WithTimeout(eCtx.Request().Context(), 300*time.Millisecond)
	defer cancel()

	value := node.MonitorUpdate{
		GlobalExpect:             &globalExpect,
		CandidateOrchestrationID: uuid.New(),
	}
	msg, err := msgbus.NewSetNodeMonitorWithErr(ctx, a.localhost, value)

	a.EventBus.Pub(msg, a.LabelNode, labelAPI)
	return JSONFromSetNodeMonitorError(eCtx, &value, err.Receive())
}
