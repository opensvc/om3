package daemonapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
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

func (a *DaemonAPI) PostClusterAction(ctx echo.Context, globalExpect node.MonitorGlobalExpect) error {
	var (
		value = node.MonitorUpdate{}
	)
	if mon := node.MonitorData.Get(a.localhost); mon == nil {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "node monitor not found: %s", a.localhost)
	}
	value = node.MonitorUpdate{
		GlobalExpect:             &globalExpect,
		CandidateOrchestrationID: uuid.New(),
	}
	msg := msgbus.SetNodeMonitor{
		Node:  a.localhost,
		Value: value,
		Err:   make(chan error),
	}
	a.EventBus.Pub(&msg, labelAPI, a.LabelNode)
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	var errs error
	for {
		select {
		case <-ticker.C:
			return JSONProblemf(ctx, http.StatusRequestTimeout, "set monitor", "timeout publishing the node %s expectation", globalExpect)
		case err := <-msg.Err:
			if err != nil {
				errs = errors.Join(errs, err)
			} else if errs != nil {
				return JSONProblemf(ctx, http.StatusConflict, "set monitor", "%s", errs)
			} else {
				return ctx.JSON(http.StatusOK, api.OrchestrationQueued{
					OrchestrationID: value.CandidateOrchestrationID,
				})
			}
		case <-ctx.Request().Context().Done():
			return JSONProblemf(ctx, http.StatusGone, "set monitor", "")
		}
	}
}
