package daemonapi

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

func (a *DaemonAPI) PostPeerActionDrain(ctx echo.Context, nodename string) error {
	if v, err := assertRoot(ctx); !v {
		return err
	}
	nodename = a.parseNodename(nodename)
	if nodename == a.localhost {
		return a.localNodeActionDrain(ctx)
	}
	return a.proxy(ctx, nodename, func(c *client.T) (*http.Response, error) {
		return c.PostPeerActionDrain(ctx.Request().Context(), nodename)
	})
}

func (a *DaemonAPI) localNodeActionDrain(eCtx echo.Context) error {
	if mon := node.MonitorData.GetByNode(a.localhost); mon == nil {
		return JSONProblemf(eCtx, http.StatusNotFound, "Not found", "node monitor not found: %s", a.localhost)
	}

	ctx, cancel := context.WithTimeout(eCtx.Request().Context(), 300*time.Millisecond)
	defer cancel()

	localExpect := node.MonitorLocalExpectDrained
	value := node.MonitorUpdate{
		LocalExpect:              &localExpect,
		CandidateOrchestrationID: uuid.New(),
	}

	msg, errReceiver := msgbus.NewSetNodeMonitorWithErr(ctx, a.localhost, value)
	a.Publisher.Pub(msg, a.LabelLocalhost, labelOriginAPI)

	return JSONFromSetNodeMonitorError(eCtx, &value, errReceiver.Receive())
}
