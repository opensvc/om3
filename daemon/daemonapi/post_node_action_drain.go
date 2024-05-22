package daemonapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/node"
)

func (a *DaemonAPI) PostPeerActionDrain(ctx echo.Context, nodename string) error {
	if nodename == a.localhost {
		return a.localNodeActionDrain(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostPeerActionDrain(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) localNodeActionDrain(c echo.Context) error {
	localExpect := node.MonitorLocalExpectDrained
	value := node.MonitorUpdate{
		LocalExpect:              &localExpect,
		CandidateOrchestrationID: uuid.New(),
	}

	return a.setNodeMonitor(c, value, 300*time.Millisecond)
}
