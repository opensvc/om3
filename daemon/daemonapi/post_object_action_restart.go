package daemonapi

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonAPI) PostObjectActionRestart(eCtx echo.Context, namespace string, kind naming.Kind, name string) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(eCtx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}
	if instMon := instance.MonitorData.Get(p, a.localhost); instMon != nil {
		var payload api.PostObjectActionRestart
		if err := eCtx.Bind(&payload); err != nil {
			return JSONProblem(eCtx, http.StatusBadRequest, "Invalid Body", err.Error())
		}

		ctx, cancel := context.WithTimeout(eCtx.Request().Context(), 300*time.Millisecond)
		defer cancel()

		globalExpect := instance.MonitorGlobalExpectRestarted
		options := instance.MonitorGlobalExpectOptionsRestarted{}
		if payload.Force != nil && *payload.Force {
			options.Force = true
		}
		value := instance.MonitorUpdate{
			GlobalExpect:             &globalExpect,
			GlobalExpectOptions:      options,
			CandidateOrchestrationID: uuid.New(),
		}

		msg, setInstanceMonitorErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, a.localhost, value)

		a.EventBus.Pub(msg, pubsub.Label{"path", p.String()}, labelAPI)

		return JSONFromSetInstanceMonitorError(eCtx, &value, setInstanceMonitorErr.Receive())
	}
	for nodename, _ := range instance.MonitorData.GetByPath(p) {
		return a.proxy(eCtx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostObjectActionRestartWithBody(eCtx.Request().Context(), namespace, kind, name, eCtx.Request().Header.Get("Content-Type"), eCtx.Request().Body)
		})
	}
	return JSONProblemf(eCtx, http.StatusNotFound, "Not found", "Object does not exist: %s", p)
}
