package daemonapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/clusternode"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/pubsub"
)

func (a *DaemonAPI) PostDaemonShutdown(ctx echo.Context, nodename string, params api.PostDaemonShutdownParams) error {
	if nodename == a.localhost {
		return a.localPostDaemonShutdown(ctx, params)
	} else if !clusternode.Has(nodename) {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid nodename", "field 'nodename' with value '%s' is not a cluster node", nodename)
	}
	c, err := a.newProxyClient(ctx, nodename)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "New client", "%s: %s", nodename, err)
	}
	resp, err := c.PostDaemonShutdownWithResponse(ctx.Request().Context(), nodename, &params)
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Request peer", "%s: %s", nodename, err)
	} else if len(resp.Body) > 0 {
		return ctx.JSONBlob(resp.StatusCode(), resp.Body)
	}
	return nil
}

// PostDaemonShutdown is the daemon shutdown handler.
//
// It shuts down vol and svc objects then stop the daemon with following steps:
//   - announces node monitor state shutting
//   - sets local expect shutdown for vol and svc objects
//   - waits for vol and svc objects to reach monitor state shutdown
//   - announces node monitor state shutdown
//   - publishes DaemonCtl stop
//
// On unexpected errors it reverts pending local expect, and announces node monitor state shutdown failed
func (a *DaemonAPI) localPostDaemonShutdown(eCtx echo.Context, params api.PostDaemonShutdownParams) error {
	var (
		log                        = LogHandler(eCtx, "PostDaemonShutdown")
		monitorLocalExpectShutdown = instance.MonitorLocalExpectShutdown
		orchestrationID            = uuid.New()
		shutdownCancel             context.CancelFunc
		shutdownCtx                = context.Background()
		toWait                     = make(map[naming.Path]instance.MonitorState)
	)
	if params.Duration != nil {
		if v, err := converters.Duration.Convert(*params.Duration); err != nil {
			log.Infof("Invalid parameter: field 'duration' with value '%s' validation error: %s", *params.Duration, err)
			return JSONProblemf(eCtx, http.StatusBadRequest, "Invalid parameter", "field 'duration' with value '%s' validation error: %s", *params.Duration, err)
		} else if timeout := *v.(*time.Duration); timeout > 0 {
			shutdownCtx, shutdownCancel = context.WithTimeout(shutdownCtx, timeout)
			defer shutdownCancel()
		}

	}

	a.announceNodeState(log, node.MonitorStateShutting)

	sub := a.EventBus.Sub(fmt.Sprintf("api.post_daemon_shutdown %s", eCtx.Get("uuid")))
	sub.AddFilter(&msgbus.InstanceMonitorUpdated{}, a.LabelLocalhost)
	sub.Start()
	defer func() {
		if err := sub.Stop(); err != nil {
			log.Errorf("sub stop %s", err)
		}
	}()

	getMonitorStates := func() map[naming.Path]instance.MonitorState {
		result := make(map[naming.Path]instance.MonitorState)
		for p, instanceMonitor := range instance.MonitorData.GetByNode(a.localhost) {
			switch p.Kind {
			case naming.KindSvc, naming.KindVol:
				result[p] = instanceMonitor.State
			default:
				// skipped (not svc or vol)
				continue
			}
		}
		return result
	}

	onInstanceMonitorUpdated := func(e *msgbus.InstanceMonitorUpdated) {
		if waitedState, ok := toWait[e.Path]; !ok {
			// not waiting => skip
			return
		} else if e.Value.State.Is(instance.MonitorStateShutdown) && !waitedState.Is(instance.MonitorStateShutdown) {
			delete(toWait, e.Path)
			var waiting []string
			for k := range toWait {
				waiting = append(waiting, k.String())
			}
			logP := naming.LogWithPath(log, e.Path)
			if len(waiting) > 0 {
				logP.Infof("object '%s' has now state shutdown, remaining objects to wait: %s", e.Path, waiting)
			} else {
				logP.Infof("object '%s' has now state shutdown", e.Path)
			}
		} else {
			toWait[e.Path] = e.Value.State
		}
	}

	revertOnError := func() {
		idleState := instance.MonitorStateIdle

		revertState := func(p naming.Path, currentState instance.MonitorState) {
			log.Infof("revert %s state %s to idle", p, currentState)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			value := instance.MonitorUpdate{CandidateOrchestrationID: orchestrationID, State: &idleState}
			msg, setImonErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, a.localhost, value)

			a.EventBus.Pub(msg, pubsub.Label{"path", p.String()}, labelAPI)

			if err := setImonErr.Receive(); err != nil {
				log.Warnf("can't revert %s state %s to idle: %s", p, currentState, err)
			}
			cancel()
		}

		for p := range toWait {
			currentState := instance.MonitorData.Get(p, a.localhost).State
			if !currentState.Is(instance.MonitorStateIdle, instance.MonitorStateShutting) {
				revertState(p, currentState)
			}
		}
	}

	log.Infof("prepare objects to accept local expect shutdown")
	for p, state := range getMonitorStates() {
		if state.Is(instance.MonitorStateIdle) {
			logP := naming.LogWithPath(log, p)
			toWait[p] = instance.MonitorData.Get(p, a.localhost).State
			logP.Infof("ask '%s' to shutdown (current state is %s)", p, state)

			ctx, cancel := context.WithTimeout(shutdownCtx, time.Second)

			value := instance.MonitorUpdate{
				CandidateOrchestrationID: orchestrationID,
				LocalExpect:              &monitorLocalExpectShutdown,
			}
			msg, setImonErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, a.localhost, value)

			a.EventBus.Pub(msg, pubsub.Label{"path", p.String()}, labelAPI)

			err := setImonErr.Receive()
			cancel()

			if err != nil {
				logP.Errorf("failed: %s refused local expect shutdown: %s", p, err)
				a.announceNodeState(log, node.MonitorStateShutdownFailed)
				revertOnError()
				return JSONProblemf(eCtx, http.StatusInternalServerError, "daemon shutdown failed",
					"%s refused local expect shutdown: %s", p, err)
			}
		}
	}

	log.Infof("wait for objects to reach state shutdown")
	for {
		select {
		case i := <-sub.C:
			switch e := i.(type) {
			case *msgbus.InstanceMonitorUpdated:
				onInstanceMonitorUpdated(e)
				if len(toWait) == 0 {
					log.Infof("all objects have state shutdown")
					a.announceNodeState(log, node.MonitorStateShutdown)
					log.Infof("ask daemon do stop")
					a.EventBus.Pub(&msgbus.DaemonCtl{Component: "daemon", Action: "stop"},
						pubsub.Label{"id", "daemon"}, a.LabelLocalhost, labelAPI)
					log.Infof("succeed")
					return JSONProblem(eCtx, http.StatusOK, "all objects are now shutdown, daemon will stop", "")
				}
			}
		case <-shutdownCtx.Done():
			log.Errorf("failed: %s", shutdownCtx.Err())
			a.announceNodeState(log, node.MonitorStateShutdownFailed)
			revertOnError()
			return JSONProblemf(eCtx, http.StatusInternalServerError, "daemon shutdown failed",
				"wait: %s", shutdownCtx.Err())
		}
	}
}
