package daemonapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
)

// JSONFromSetNodeMonitorError sends a JSON response where status code depends
// on SetNodeMonitor error value.
//   - StatusOK: expectation value accepted
//   - StatusRequestTimeout: request context DeadlineExceeded or timeout reached
//   - StatusGone: request context Canceled
//   - StatusConflict: expectation value refused
func JSONFromSetNodeMonitorError(eCtx echo.Context, value *node.MonitorUpdate, err error) error {
	switch {
	case err == nil:
		return eCtx.JSON(http.StatusOK, api.OrchestrationQueued{OrchestrationID: value.CandidateOrchestrationID})
	case errors.Is(err, context.DeadlineExceeded):
		return JSONProblemf(eCtx, http.StatusRequestTimeout, "set node monitor", "timeout publishing the node %s expectation", *value)
	case errors.Is(err, context.Canceled):
		return JSONProblemf(eCtx, http.StatusGone, "set node monitor", "client context canceled")
	default:
		return JSONProblemf(eCtx, http.StatusConflict, "set node monitor", "expectation %s: %s", *value, err)
	}
}
