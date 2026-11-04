package daemonapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/session"
)

// JSONFromSetNodeMonitorError sends a JSON response where status code depends
// on SetNodeMonitor error value.
//   - StatusOK: expectation value accepted, or queued to a node monitor
//     not reading it in time
//   - StatusRequestTimeout: the client gone
//   - StatusConflict: expectation value refused
func JSONFromSetNodeMonitorError(eCtx echo.Context, nodename string, value *node.MonitorUpdate, err error) error {
	queued := func(unread bool) error {
		// Recorded before the id is answered, for the client to find it
		// if it asks at once: the monitor says it took it on, or refused
		// it, only after answering.
		session.AddOrchestrationIfUnknown(session.Orchestration{
			OrchestrationID: value.CandidateOrchestrationID.String(),
			Node:            nodename,
		})
		answer := api.OrchestrationQueued{OrchestrationID: value.CandidateOrchestrationID, Node: &nodename}
		if unread {
			answer.Queued = &unread
		}
		return eCtx.JSON(http.StatusOK, answer)
	}
	switch {
	case err == nil:
		return queued(false)
	case errors.Is(err, context.DeadlineExceeded):
		// The request is published: the node monitor, busy, as setting
		// the networks up, reads it later, and takes it on or refuses
		// it under the id. A timeout is no refusal.
		LogHandler(eCtx, "set node monitor").Warnf("the node monitor did not read %s in time: answer it queued", *value)
		return queued(true)
	case errors.Is(err, context.Canceled):
		return JSONProblemf(eCtx, http.StatusRequestTimeout, "set node monitor", "client context canceled")
	default:
		return JSONProblemf(eCtx, http.StatusConflict, "set node monitor", "%s", err)
	}
}
