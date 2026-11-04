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
	"github.com/opensvc/om3/v3/daemon/session"
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
	instance.MonitorStatePreflightProgress,
	instance.MonitorStateProvisionProgress,
	instance.MonitorStateResizeProgress,
	instance.MonitorStateShutdownProgress,
	instance.MonitorStateStartProgress,
	instance.MonitorStateStopProgress,
	instance.MonitorStateUnprovisionProgress,
}

// JSONFromSetInstanceMonitorError sends a JSON response where status code depends
// on SetMonitorUpdate error value.
//   - StatusOK: expectation value accepted, or queued to an instance monitor
//     not reading it in time, as one running an action
//   - StatusRequestTimeout: the client gone
//   - StatusConflict: expectation value refused
func JSONFromSetInstanceMonitorError(eCtx echo.Context, p naming.Path, node string, value *instance.MonitorUpdate, err error) error {
	queued := func(unread bool) error {
		// Recorded before the id is answered, for the client to find it
		// if it asks at once: the monitor says it took it on, or refused
		// it, only after answering.
		o := session.Orchestration{
			OrchestrationID: value.CandidateOrchestrationID.String(),
			Node:            node,
			Path:            p.String(),
		}
		if value.GlobalExpect != nil {
			o.Expect = value.GlobalExpect.String()
		}
		session.AddOrchestrationIfUnknown(o)
		answer := api.OrchestrationQueued{OrchestrationID: value.CandidateOrchestrationID, Node: &node}
		if unread {
			answer.Queued = &unread
		}
		return eCtx.JSON(http.StatusOK, answer)
	}
	switch {
	case err == nil:
		return queued(false)
	case errors.Is(err, context.DeadlineExceeded):
		// The request is published: the instance monitor, busy, as
		// running an action it reads no request until the end of, reads
		// it later, and takes it on or refuses it under the id. A timeout
		// is no refusal.
		reason := "busy"
		if instMon := instance.MonitorData.GetByPathAndNode(p, node); instMon != nil && instMon.State.IsOneOf(monitorStatesRunningAction...) {
			reason = instMon.State.String()
		}
		LogHandler(eCtx, "set instance monitor").Infof("%s: the instance monitor, %s, did not read %s in time: answer it queued", p, reason, *value)
		return queued(true)
	case errors.Is(err, context.Canceled):
		return JSONProblemf(eCtx, http.StatusRequestTimeout, "set instance monitor", "client context canceled")
	default:
		// Refused by the instance monitor, which records the refusal
		// under the candidate id, with the reason: the client is handed
		// that id, for "daemon orchestration list" and "wait" to answer
		// about it. Recorded before the answer, as an accepted one is,
		// for a client asking at once.
		refused := api.OrchestrationRefused{
			Status: http.StatusConflict,
			Title:  "set instance monitor",
			Detail: err.Error(),
		}
		if value.CandidateOrchestrationID != uuid.Nil {
			id := value.CandidateOrchestrationID
			o := session.Orchestration{
				OrchestrationID: id.String(),
				Node:            node,
				Path:            p.String(),
			}
			if value.GlobalExpect != nil {
				o.Expect = value.GlobalExpect.String()
			}
			session.AddOrchestrationIfUnknown(o)
			session.EndOrchestration(id.String(), session.StateRefused, err.Error())
			refused.OrchestrationID = &id
			refused.Node = &node
		}
		return eCtx.JSON(http.StatusConflict, refused)
	}
}
