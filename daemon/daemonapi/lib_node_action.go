package daemonapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
)

// setNodeMonitor sets new node monitor expectation under echo.Context api request
// that needs to be informed if expectation is accepted/refused.
// It publishes SetNodeMonitor with Err: ErrMessage.
// Then uses received error to send Json responses with following http status code:
//   - StatusOK: expectation value accepted
//   - StatusRequestTimeout: request context DeadlineExceeded or timeout reached
//   - StatusGone: request context Canceled
//   - StatusConflict: expectation value refused
//   - StatusNotFound: localhost is not found in node.MonitorData
func (a *DaemonAPI) setNodeMonitor(c echo.Context, value node.MonitorUpdate, timeout time.Duration) error {
	if mon := node.MonitorData.Get(a.localhost); mon == nil {
		return JSONProblemf(c, http.StatusNotFound, "Not found", "node monitor not found: %s", a.localhost)
	}
	errMsg := msgbus.ErrMessage{Err: make(chan error)}
	if timeout > 0 {
		ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
		defer cancel()
		errMsg.Ctx = ctx
	} else {
		errMsg.Ctx = c.Request().Context()
	}
	msg := msgbus.SetNodeMonitor{
		Node:  a.localhost,
		Value: value,
		Err:   &errMsg,
	}
	a.EventBus.Pub(&msg, labelAPI, a.LabelNode)
	err := errMsg.Receive()
	switch {
	case err == nil:
		return c.JSON(http.StatusOK, api.OrchestrationQueued{OrchestrationID: value.CandidateOrchestrationID})
	case errors.Is(err, context.DeadlineExceeded):
		return JSONProblemf(c, http.StatusRequestTimeout, "set node monitor", "timeout publishing the node %s expectation", value)
	case errors.Is(err, context.Canceled):
		return JSONProblemf(c, http.StatusGone, "set node monitor", "client context canceled")
	default:
		return JSONProblemf(c, http.StatusConflict, "set node monitor", "expectation %s: %s", value, err)
	}
}
