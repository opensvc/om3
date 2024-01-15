package daemonapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
)

func (a *DaemonApi) PostPeerActionDrain(ctx echo.Context, nodename string) error {
	if nodename == a.localhost {
		return a.localNodeActionDrain(ctx)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameters", "%s is not a cluster node", nodename)
	} else {
		c, err := newProxyClient(ctx, nodename)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
		}
		if resp, err := c.PostPeerActionDrainWithResponse(ctx.Request().Context(), nodename); err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
		} else if len(resp.Body) > 0 {
			return ctx.JSONBlob(resp.StatusCode(), resp.Body)
		}
	}
	return nil
}

func (a *DaemonApi) localNodeActionDrain(ctx echo.Context) error {
	var (
		value = node.MonitorUpdate{}
	)
	if mon := node.MonitorData.Get(a.localhost); mon == nil {
		return JSONProblemf(ctx, http.StatusNotFound, "Not found", "node monitor not found: %s", a.localhost)
	}
	localExpect := node.MonitorLocalExpectDrained
	value = node.MonitorUpdate{
		LocalExpect:              &localExpect,
		CandidateOrchestrationId: uuid.New(),
	}
	msg := msgbus.SetNodeMonitor{
		Node:  a.localhost,
		Value: value,
		Err:   make(chan error),
	}
	a.EventBus.Pub(&msg, labelAPI, a.LabelNode)
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	var errs error
	for {
		select {
		case <-ticker.C:
			return JSONProblemf(ctx, http.StatusRequestTimeout, "set monitor", "timeout waiting for monitor commit")
		case err := <-msg.Err:
			if err != nil {
				errs = errors.Join(errs, err)
			} else if errs != nil {
				return JSONProblemf(ctx, http.StatusConflict, "set monitor", "%s", errs)
			} else {
				return ctx.JSON(http.StatusOK, api.OrchestrationQueued{
					OrchestrationId: value.CandidateOrchestrationId,
				})
			}
		case <-ctx.Request().Context().Done():
			return JSONProblemf(ctx, http.StatusGone, "set monitor", "")
		}
	}
}
