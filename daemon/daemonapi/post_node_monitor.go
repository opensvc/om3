package daemonapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
)

func (a *DaemonApi) PostNodeMonitor(ctx echo.Context) error {
	var (
		payload      api.PostNodeMonitor
		validRequest bool
		update       node.MonitorUpdate
	)
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid Body", err.Error())
	}
	if payload.LocalExpect != nil {
		validRequest = true
		i := node.MonitorLocalExpectValues[*payload.LocalExpect]
		update.LocalExpect = &i
	}
	if payload.GlobalExpect != nil {
		validRequest = true
		i := node.MonitorGlobalExpectValues[*payload.GlobalExpect]
		update.GlobalExpect = &i
	}
	if payload.State != nil {
		validRequest = true
		i := node.MonitorStateValues[*payload.State]
		update.State = &i
	}
	update.CandidateOrchestrationId = uuid.New()
	if !validRequest {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid Body", "Need at least 'state', 'local_expect' or 'global_expect'")
	}
	msg := msgbus.SetNodeMonitor{
		Node:  hostname.Hostname(),
		Value: update,
		Err:   make(chan error),
	}
	a.EventBus.Pub(&msg, labelApi)
	var errs error
	for {
		select {
		case err := <-msg.Err:
			if err != nil {
				errs = errors.Join(errs, err)
			} else if errs != nil {
				return JSONProblemf(ctx, http.StatusConflict, "set monitor", "%s", errs)
			} else {
				return ctx.JSON(http.StatusOK, api.OrchestrationQueued{
					OrchestrationId: update.CandidateOrchestrationId,
				})
			}
		case <-ctx.Request().Context().Done():
			return JSONProblemf(ctx, http.StatusGone, "set monitor", "")
		}
	}
}
