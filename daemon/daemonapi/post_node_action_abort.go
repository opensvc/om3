package daemonapi

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
)

func (a *DaemonAPI) PostPeerActionAbort(ctx echo.Context, nodename string) error {
	if nodename == a.localhost {
		return a.localNodeActionAbort(ctx)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	c, err := newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}

	resp, err := c.PostPeerActionAbortWithResponse(ctx.Request().Context(), nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
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
	a.EventBus.Pub(&msg, labelAPI)
	return ctx.JSON(http.StatusOK, api.OrchestrationQueued{OrchestrationID: msg.Value.CandidateOrchestrationID})
}
