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
//   - StatusOK: expectation value accepted
//   - StatusRequestTimeout: request context DeadlineExceeded or timeout reached
//   - StatusConflict: expectation value refused
func JSONFromSetNodeMonitorError(eCtx echo.Context, nodename string, value *node.MonitorUpdate, err error) error {
	// TODO: is 408 Request Timeout correct ? it may caused from slow nmon
	switch {
	case err == nil:
		// Recorded before the id is answered, for the client to find it
		// if it asks at once: the monitor says it took it on, or refused
		// it, only after answering.
		session.AddOrchestrationIfUnknown(session.Orchestration{
			OrchestrationID: value.CandidateOrchestrationID.String(),
			Node:            nodename,
		})
		return eCtx.JSON(http.StatusOK, api.OrchestrationQueued{OrchestrationID: value.CandidateOrchestrationID})
	case errors.Is(err, context.DeadlineExceeded):
		return JSONProblemf(eCtx, http.StatusRequestTimeout, "set node monitor", "timeout publishing %s", *value)
	case errors.Is(err, context.Canceled):
		return JSONProblemf(eCtx, http.StatusRequestTimeout, "set node monitor", "client context canceled")
	default:
		return JSONProblemf(eCtx, http.StatusConflict, "set node monitor", "%s", err)
	}
}
