package daemonapi

import (
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
	a.EventBus.Pub(&msgbus.SetNodeMonitor{Node: hostname.Hostname(), Value: update}, labelApi)
	return ctx.JSON(http.StatusOK, api.MonitorUpdateQueued{
		OrchestrationId: update.CandidateOrchestrationId,
	})
}
