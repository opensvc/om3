package daemonapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
)

func (a *DaemonAPI) postObjectAction(eCtx echo.Context, namespace string, kind naming.Kind, name string, globalExpect instance.MonitorGlobalExpect, fn func(c *client.T) (*http.Response, error), options ...any) error {
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
		if len(options) > 0 {
			value.GlobalExpectOptions = options[0]
		}
		msg, setImonErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, a.localhost, value)

		a.Bus.Pub(msg, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()}, labelOriginAPI)

		return JSONFromSetInstanceMonitorError(eCtx, p, a.localhost, &value, setImonErr.Receive())
	}
	for nodename := range instance.MonitorData.GetByPath(p) {
		if nodename == a.localhost {
			continue
		}
		return a.proxy(eCtx, nodename, fn)
	}
	return JSONProblem(eCtx, http.StatusNotFound, "object not found", "")
}

// monitorStatesRunningAction are the states of an instance monitor running
// an action it waits for the end of, as a stop: it reads no request until the
// action ends, so a request sent meanwhile times out.
var monitorStatesRunningAction = []instance.MonitorState{
	instance.MonitorStateBootProgress,
	instance.MonitorStateCapProgress,
	instance.MonitorStateDeleteProgress,
	instance.MonitorStateProvisionProgress,
	instance.MonitorStateResizeProgress,
	instance.MonitorStateShutdownProgress,
	instance.MonitorStateStartProgress,
	instance.MonitorStateStopProgress,
	instance.MonitorStateUnprovisionProgress,
}

// JSONFromSetInstanceMonitorError sends a JSON response where status code depends
// on SetMonitorUpdate error value.
//   - StatusOK: expectation value accepted, or an abort queued to an
//     instance monitor running an action
//   - StatusRequestTimeout: request context DeadlineExceeded or timeout reached
//   - StatusConflict: expectation value refused
func JSONFromSetInstanceMonitorError(eCtx echo.Context, p naming.Path, node string, value *instance.MonitorUpdate, err error) error {
	switch {
	case err == nil:
		return eCtx.JSON(http.StatusOK, api.OrchestrationQueued{OrchestrationID: value.CandidateOrchestrationID})
	case errors.Is(err, context.DeadlineExceeded):
		// The instance monitor did not answer. Running an action, it
		// reads no request until the action ends, which can be long, and
		// reads the request then: say so, rather than a timeout naming
		// the request.
		if instMon := instance.MonitorData.GetByPathAndNode(p, node); instMon != nil && instMon.State.IsOneOf(monitorStatesRunningAction...) {
			if value.GlobalExpect != nil && *value.GlobalExpect == instance.MonitorGlobalExpectAborted {
				// An abort is accepted whatever the orchestration in
				// progress: it is queued, not refused.
				return eCtx.JSON(http.StatusOK, api.OrchestrationQueued{OrchestrationID: value.CandidateOrchestrationID})
			}
			return JSONProblemf(eCtx, http.StatusRequestTimeout, "set instance monitor", "%s is %s on %s: the instance monitor reads the request when the action ends, and may refuse it then", p, instMon.State, node)
		}
		return JSONProblemf(eCtx, http.StatusRequestTimeout, "set instance monitor", "timeout publishing %s", *value)
	case errors.Is(err, context.Canceled):
		return JSONProblemf(eCtx, http.StatusRequestTimeout, "set instance monitor", "client context canceled")
	default:
		return JSONProblemf(eCtx, http.StatusConflict, "set instance monitor", "%s", err)
	}
}
