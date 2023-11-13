package daemonapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/pubsub"
)

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
func (a *DaemonApi) PostDaemonShutdown(ctx echo.Context, params api.PostDaemonShutdownParams) error {
	var (
		log                        = LogHandler(ctx, "PostDaemonShutdown")
		monitorLocalExpectShutdown = instance.MonitorLocalExpectShutdown
		orchestrationId            = uuid.New()
		shutdownCancel             context.CancelFunc
		shutdownCtx                = context.Background()
		toWait                     = make(map[naming.Path]instance.MonitorState)
	)
	if params.Duration != nil {
		if v, err := converters.Duration.Convert(*params.Duration); err != nil {
			log.Infof("Invalid parameter: field 'duration' with value '%s' validation error: %s", *params.Duration, err)
			return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "field 'duration' with value '%s' validation error: %s", *params.Duration, err)
		} else if timeout := *v.(*time.Duration); timeout > 0 {
			shutdownCtx, shutdownCancel = context.WithTimeout(shutdownCtx, timeout)
			defer shutdownCancel()
		}

	}

	a.announceNodeState(log, node.MonitorStateShutting)

	sub := a.EventBus.Sub(fmt.Sprintf("PostDaemonShutdown %s", ctx.Get("uuid")))
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

	setInstanceMonitor := func(p naming.Path, value instance.MonitorUpdate) error {
		errC := make(chan error)
		a.EventBus.Pub(&msgbus.SetInstanceMonitor{Path: p, Node: a.localhost, Value: value, Err: errC},
			pubsub.Label{"path", p.String()}, labelApi)
		return <-errC
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
			if len(waiting) > 0 {
				log.Infof("%s now shutdown, remaining: %s", e.Path, waiting)
			} else {
				log.Infof("%s now shutdown", e.Path)
			}
		} else {
			toWait[e.Path] = e.Value.State
		}
	}

	revertOnError := func() {
		idleState := instance.MonitorStateIdle
		for p := range toWait {
			waitingState := instance.MonitorData.Get(p, a.localhost).State
			if !waitingState.Is(instance.MonitorStateIdle, instance.MonitorStateShutting) {
				log.Infof("revert %s state %s to idle", p, waitingState)
				value := instance.MonitorUpdate{CandidateOrchestrationId: orchestrationId, State: &idleState}
				if err := setInstanceMonitor(p, value); err != nil {
					log.Warnf("can't revert %s state %s to idle: %s", p, waitingState, err)
				}
			}
		}
	}

	log.Infof("prepare objects shutdown")
	for p, state := range getMonitorStates() {
		if state.Is(instance.MonitorStateIdle) {
			sub.AddFilter(&msgbus.InstanceMonitorUpdated{}, a.LabelNode, pubsub.Label{"path", p.String()})
			toWait[p] = instance.MonitorData.Get(p, a.localhost).State
			log.Infof("ask shutdown for %s with state %s", p, state)
			value := instance.MonitorUpdate{
				CandidateOrchestrationId: orchestrationId,
				LocalExpect:              &monitorLocalExpectShutdown,
			}
			if err := setInstanceMonitor(p, value); err != nil {
				log.Errorf("failed: %s refused local expect shutdown: %s", p, err)
				a.announceNodeState(log, node.MonitorStateShutdownFailed)
				revertOnError()
				return JSONProblemf(ctx, http.StatusInternalServerError, "daemon shutdown failed",
					"%s refused local expect shutdown: %s", p, err)
			}
		}
	}

	log.Infof("wait for objects shutdown")
	for {
		select {
		case i := <-sub.C:
			switch e := i.(type) {
			case *msgbus.InstanceMonitorUpdated:
				onInstanceMonitorUpdated(e)
				if len(toWait) == 0 {
					log.Infof("all objects are now shutdown")
					a.announceNodeState(log, node.MonitorStateShutdown)
					log.Infof("ask daemon do stop")
					a.EventBus.Pub(&msgbus.DaemonCtl{Component: "daemon", Action: "stop"},
						pubsub.Label{"id", "daemon"}, labelApi, a.LabelNode)
					log.Infof("succeed")
					return JSONProblem(ctx, http.StatusOK, "all objects are now shutdown, daemon will stop", "")
				}
			}
		case <-shutdownCtx.Done():
			log.Errorf("failed: %s", shutdownCtx.Err())
			a.announceNodeState(log, node.MonitorStateShutdownFailed)
			revertOnError()
			return JSONProblemf(ctx, http.StatusInternalServerError, "daemon shutdown failed",
				"wait: %s", shutdownCtx.Err())
		}
	}
}
