package daemonapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonApi) PostObjectMonitor(ctx echo.Context) error {
	var (
		payload api.PostObjectMonitor
		update  instance.MonitorUpdate
		p       path.T
		err     error
	)
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "%s", err)
	}
	p, err = path.Parse(payload.Path)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid body", "Error parsing path %s: %s", payload.Path, err)
	}
	if payload.GlobalExpect != nil {
		i := instance.MonitorGlobalExpectValues[*payload.GlobalExpect]
		update.GlobalExpect = &i
	}
	if payload.LocalExpect != nil {
		i := instance.MonitorLocalExpectValues[*payload.LocalExpect]
		update.LocalExpect = &i
	}
	if payload.State != nil {
		i := instance.MonitorStateValues[*payload.State]
		update.State = &i
	}
	update.CandidateOrchestrationId = uuid.New()
	msg := msgbus.SetInstanceMonitor{
		Path:  p,
		Node:  hostname.Hostname(),
		Value: update,
		Err:   make(chan error),
	}
	a.EventBus.Pub(&msg, pubsub.Label{"path", p.String()}, labelApi)
	var errs error
	for {
		select {
		case err := <-msg.Err:
			if err != nil {
				errs = errors.Join(errs, err)
			} else if errs != nil {
				return JSONProblemf(ctx, http.StatusBadRequest, "set monitor", "%s", errs)
			} else {
				return ctx.JSON(http.StatusOK, api.MonitorUpdateQueued{
					OrchestrationId: update.CandidateOrchestrationId,
				})
			}
		case <-ctx.Request().Context().Done():
			return JSONProblemf(ctx, http.StatusGone, "set monitor", "")
		}
	}
}
