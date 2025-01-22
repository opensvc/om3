package daemonapi

import (
	"context"
	"errors"
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

func (a *DaemonAPI) postObjectAction(eCtx echo.Context, namespace string, kind naming.Kind, name string, globalExpect instance.MonitorGlobalExpect, fn func(c *client.T) (*http.Response, error)) error {
	p, err := naming.NewPath(namespace, kind, name)
	if err != nil {
		return JSONProblem(eCtx, http.StatusBadRequest, "Invalid parameters", err.Error())
	}
	if instMon := instance.MonitorData.GetByPathAndNode(p, a.localhost); instMon != nil {
		ctx, cancel := context.WithTimeout(eCtx.Request().Context(), 500*time.Millisecond)
		defer cancel()

		value := instance.MonitorUpdate{
			GlobalExpect:             &globalExpect,
			CandidateOrchestrationID: uuid.New(),
		}
		msg, setImonErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, a.localhost, value)

		a.EventBus.Pub(msg, pubsub.Label{"path", p.String()}, labelAPI)

		return JSONFromSetInstanceMonitorError(eCtx, &value, setImonErr.Receive())
	}
	for nodename, _ := range instance.MonitorData.GetByPath(p) {
		if nodename == a.localhost {
			continue
		}
		return a.proxy(eCtx, nodename, fn)
	}
	return JSONProblem(eCtx, http.StatusNotFound, "object not found", "")
}

// JSONFromSetInstanceMonitorError sends a JSON response where status code depends
// on SetMonitorUpdate error value.
//   - StatusOK: expectation value accepted
//   - StatusRequestTimeout: request context DeadlineExceeded or timeout reached
//   - StatusConflict: expectation value refused
func JSONFromSetInstanceMonitorError(eCtx echo.Context, value *instance.MonitorUpdate, err error) error {
	// TODO: is 408 Request Timeout correct ? it may caused from slow imon
	switch {
	case err == nil:
		return eCtx.JSON(http.StatusOK, api.OrchestrationQueued{OrchestrationID: value.CandidateOrchestrationID})
	case errors.Is(err, context.DeadlineExceeded):
		return JSONProblemf(eCtx, http.StatusRequestTimeout, "set instance monitor", "timeout publishing %s", *value)
	case errors.Is(err, context.Canceled):
		return JSONProblemf(eCtx, http.StatusRequestTimeout, "set instance monitor", "client context canceled")
	default:
		return JSONProblemf(eCtx, http.StatusConflict, "set instance monitor", "%s", err)
	}
}
