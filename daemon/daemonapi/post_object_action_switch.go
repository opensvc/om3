package daemonapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonAPI) PostObjectActionSwitch(ctx echo.Context, namespace string, kind naming.Kind, name string) error {
	var (
		payload = api.PostObjectActionSwitch{}
		value   = instance.MonitorUpdate{}
		p       naming.Path
		err     error
	)
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid Body", err.Error())
	}
	p, err = naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	if instMon := instance.MonitorData.Get(p, a.localhost); instMon == nil {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "Object does not exist: %s", p)
	}
	globalExpect := instance.MonitorGlobalExpectPlacedAt
	options := instance.MonitorGlobalExpectOptionsPlacedAt{}
	options.Destination = append(options.Destination, payload.Destination...)
	value = instance.MonitorUpdate{
		GlobalExpect:             &globalExpect,
		GlobalExpectOptions:      options,
		CandidateOrchestrationID: uuid.New(),
	}
	timeout := 300 * time.Millisecond
	msg := msgbus.SetInstanceMonitor{
		Path:    p,
		Node:    a.localhost,
		Value:   value,
		Err:     make(chan error),
		Timeout: timeout,
	}
	a.EventBus.Pub(&msg, pubsub.Label{"path", p.String()}, labelAPI)
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()
	select {
	case <-ticker.C:
		return JSONProblemf(ctx, http.StatusRequestTimeout, "set monitor", "timeout publishing the %s switch expectation", kind)
	case err := <-msg.Err:
		if err != nil {
			return JSONProblemf(ctx, http.StatusConflict, "set monitor", "%s", err)
		} else {
			return ctx.JSON(http.StatusOK, api.OrchestrationQueued{
				OrchestrationID: value.CandidateOrchestrationID,
			})
		}
	case <-ctx.Request().Context().Done():
		return JSONProblemf(ctx, http.StatusGone, "set monitor", "")
	}
}
