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

func (a *DaemonAPI) PostObjectActionSwitch(eCtx echo.Context, namespace string, kind naming.Kind, name string) error {
	if v, err := assertOperator(eCtx, namespace); !v {
		return err
	}
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblemf(eCtx, http.StatusBadRequest, "Invalid parameters", "%s", err)
	}

	if instMon := instance.MonitorData.GetByPathAndNode(p, a.localhost); instMon != nil {
		var payload api.PostObjectActionSwitch
		if err := eCtx.Bind(&payload); err != nil {
			return JSONProblem(eCtx, http.StatusBadRequest, "Invalid Body", err.Error())
		}

		ctx, cancel := context.WithTimeout(eCtx.Request().Context(), 300*time.Millisecond)
		defer cancel()

		globalExpect := instance.MonitorGlobalExpectPlacedAt
		value := instance.MonitorUpdate{
			GlobalExpect: &globalExpect,
			GlobalExpectOptions: instance.MonitorGlobalExpectOptionsPlacedAt{
				Destination: payload.Destination,
			},
			CandidateOrchestrationID: uuid.New(),
		}

		msg, setInstanceMonitorErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, a.localhost, value)

		a.Pub.Pub(msg, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()}, labelOriginAPI)

		return JSONFromSetInstanceMonitorError(eCtx, &value, setInstanceMonitorErr.Receive())
	}
	for nodename := range instance.MonitorData.GetByPath(p) {
		return a.proxy(eCtx, nodename, func(c *client.T) (*http.Response, error) {
			return c.PostObjectActionSwitchWithBody(eCtx.Request().Context(), namespace, kind, name, eCtx.Request().Header.Get("Content-Type"), eCtx.Request().Body)
		})
	}
	return JSONProblemf(eCtx, http.StatusNotFound, "Not found", "Object not found: %s", p)
}
