package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
)

func (a *DaemonAPI) PostPeerActionAbort(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	if nodename == a.localhost {
		return a.localNodeActionAbort(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostPeerActionAbort(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) localNodeActionAbort(ctx echo.Context) error {
	v := node.MonitorLocalExpectNone
	msg := msgbus.SetNodeMonitor{
		Node: a.localhost,
		Value: node.MonitorUpdate{
			LocalExpect:              &v,
			CandidateOrchestrationID: uuid.New(),
		},
	}
	a.EventBus.Pub(&msg, labelOriginAPI)
	return ctx.JSON(http.StatusOK, api.OrchestrationQueued{OrchestrationID: msg.Value.CandidateOrchestrationID})
}
