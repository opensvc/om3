package daemonapi

import (
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
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

func (a *DaemonAPI) PostClusterAction(c echo.Context, globalExpect node.MonitorGlobalExpect) error {
	value := node.MonitorUpdate{
		GlobalExpect:             &globalExpect,
		CandidateOrchestrationID: uuid.New(),
	}
	return a.setNodeMonitor(c, value, 300*time.Millisecond)
}
