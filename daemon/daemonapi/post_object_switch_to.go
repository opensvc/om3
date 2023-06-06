package daemonapi

import (
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

func (a *DaemonApi) PostObjectSwitchTo(ctx echo.Context) error {
	var (
		payload = api.PostObjectSwitchTo{}
		value   = instance.MonitorUpdate{}
		p       path.T
		err     error
	)
	if err := ctx.Bind(&payload); err != nil {
		return JSONProblem(ctx, http.StatusBadRequest, "Invalid Body", err.Error())
	}
	p, err = path.Parse(payload.Path)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid Body", "Invalid path: %s", payload.Path)
	}
	globalExpect := instance.MonitorGlobalExpectPlacedAt
	options := instance.MonitorGlobalExpectOptionsPlacedAt{}
	options.Destination = append(options.Destination, payload.Destination...)
	value = instance.MonitorUpdate{
		GlobalExpect:        &globalExpect,
		GlobalExpectOptions: options,
	}
	value.CandidateOrchestrationId = uuid.New()
	a.EventBus.Pub(&msgbus.SetInstanceMonitor{Path: p, Node: hostname.Hostname(), Value: value},
		pubsub.Label{"path", p.String()}, labelApi)
	return ctx.JSON(http.StatusOK, api.MonitorUpdateQueued{
		OrchestrationId: value.CandidateOrchestrationId,
	})
}
