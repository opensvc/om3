package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/daemon/session"
)

func (a *DaemonAPI) PostPeerActionAbort(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
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
	// Recorded before the id is answered, for the client to find it if it
	// asks at once: the monitor says it took it on, or refused it, when it
	// reads the request.
	session.AddOrchestrationIfUnknown(session.Orchestration{
		OrchestrationID: msg.Value.CandidateOrchestrationID.String(),
		Node:            a.localhost,
	})
	a.Bus.Pub(&msg, labelOriginAPI)
	return ctx.JSON(http.StatusOK, api.OrchestrationQueued{OrchestrationID: msg.Value.CandidateOrchestrationID})
}
