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
	if v, err := assertRoot(ctx); !v {
		return err
	}

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

// localPostDaemonShutdown shuts down all local instances and transitions the daemon
// to a stop state upon success.
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
		log             = LogHandler(eCtx, "PostDaemonShutdown")
		orchestrationID = uuid.New()
		shutdownCancel  context.CancelFunc
		shutdownCtx     = context.Background()

		// shutdownWaiting is a map of instance paths where shutdown has been requested but has not yet occurred
		shutdownWaiting = make(map[naming.Path]struct{})

		// shutdownFail is a list of local instances that failed to shut down.
		shutdownFail = make([]naming.Path, 0)

		// pathsToResetOnFailure is a list of local instance paths where "local expect"
		// was set to shut down. If the daemon fails to shut down, it won't be stopped,
		// so we need to reset the "local expect" of the local instance to "none".
		pathsToResetOnFailure = make([]naming.Path, 0)
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

	a.announceNodeState(log, node.MonitorStateShutdownProgress)

	sub := a.SubFactory.Sub(fmt.Sprintf("api.post_daemon_shutdown %s", eCtx.Get("uuid")))
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
		if _, ok := shutdownWaiting[e.Path]; !ok {
			// not waiting => skipped
			return
		}
		switch e.Value.State {
		case instance.MonitorStateShutdownSuccess:
			delete(shutdownWaiting, e.Path)
		case instance.MonitorStateShutdownFailure:
			delete(shutdownWaiting, e.Path)
			shutdownFail = append(shutdownFail, e.Path)
		default:
			return
		}
		var waiting []string
		for p := range shutdownWaiting {
			waiting = append(waiting, p.String())
		}
		logP := naming.LogWithPath(log, e.Path)
		if len(waiting) > 0 {
			logP.Infof("the local instance '%s' is %s. Remaining local instances waiting for shutdown: %s", e.Path, e.Value.State, waiting)
		} else {
			logP.Infof("the local instance '%s' is %s", e.Path, e.Value.State)
		}
	}

	resetLocalExpectOnError := func(l ...naming.Path) {
		localExpectNone := instance.MonitorLocalExpectNone
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		for _, p := range l {
			mon := instance.MonitorData.GetByPathAndNode(p, a.localhost)
			if mon == nil {
				continue
			}
			if mon.LocalExpect != instance.MonitorLocalExpectShutdown {
				continue
			}
			value := instance.MonitorUpdate{CandidateOrchestrationID: orchestrationID, LocalExpect: &localExpectNone}
			msg, setImonErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, a.localhost, value)

			naming.LogWithPath(log, p).Warnf("revert %s local expect %s to %s", p, mon.LocalExpect, localExpectNone)
			a.Publisher.Pub(msg, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()}, labelOriginAPI)

			if err := setImonErr.Receive(); err != nil {
				log.Warnf("can't revert %s local expect %s to %s: %s", p, mon.LocalExpect, localExpectNone, err)
			}
		}
	}

	log.Infof("prepare objects to accept local expect shutdown")
	for p, state := range getMonitorStates() {
		logP := naming.LogWithPath(log, p)
		if instance.ConfigData.GetByPathAndNode(p, a.localhost).IsDisabled {
			logP.Debugf("shutdown skipped on disabled local instance")
			continue
		}
		// TODO: perhaps here we should shutdown all local instance that are not shutdown ?
		//       if !state.Is(instance.MonitorStateShutdown)
		if state.Is(instance.MonitorStateIdle) || state.Is(instance.MonitorStatesFailure...) {
			logP.Infof("ask '%s' to shutdown (current state is %s)", p, state)

			ctx, cancel := context.WithTimeout(shutdownCtx, time.Second)

			localExpectShutdown := instance.MonitorLocalExpectShutdown
			stateIdle := instance.MonitorStateIdle
			value := instance.MonitorUpdate{
				CandidateOrchestrationID: orchestrationID,
				LocalExpect:              &localExpectShutdown,
				State:                    &stateIdle,
			}
			msg, setImonErr := msgbus.NewSetInstanceMonitorWithErr(ctx, p, a.localhost, value)

			a.Publisher.Pub(msg, pubsub.Label{"namespace", p.Namespace}, pubsub.Label{"path", p.String()}, labelOriginAPI)

			err := setImonErr.Receive()
			cancel()

			if err != nil {
				logP.Errorf("failure: %s refused local expect shutdown: %s", p, err)
				a.announceNodeState(log, node.MonitorStateShutdownFailure)
				resetLocalExpectOnError()
				return JSONProblemf(eCtx, http.StatusInternalServerError, "daemon shutdown failure",
					"%s refused local expect shutdown: %s", p, err)
			} else {
				pathsToResetOnFailure = append(pathsToResetOnFailure, p)
				shutdownWaiting[p] = struct{}{}
			}
		}
	}

	if len(shutdownWaiting) == 0 {
		log.Infof("no local instances pending shutdown: daemon will stop immediately")
		a.announceNodeState(log, node.MonitorStateShutdownSuccess)
		a.Publisher.Pub(&msgbus.DaemonCtl{Component: "daemon", Action: "stop"},
			pubsub.Label{"id", "daemon"}, a.LabelLocalhost, labelOriginAPI)
		log.Infof("succeed")
		return JSONProblem(eCtx, http.StatusOK, "no local instances pending shutdown: daemon will stop immediately", "")
	}
	log.Infof("waiting for local instances to shut down")
	for {
		select {
		case i := <-sub.C:
			switch e := i.(type) {
			case *msgbus.InstanceMonitorUpdated:
				onInstanceMonitorUpdated(e)
				if len(shutdownWaiting) > 0 {
					// some local instance shut down are still pending
					continue
				}
				if len(shutdownFail) == 0 { // all local instance shut down occurred and no failures.
					log.Infof("all local instances are in the shutdown state: daemon will stop immediately")
					a.announceNodeState(log, node.MonitorStateShutdownSuccess)
					a.Publisher.Pub(&msgbus.DaemonCtl{Component: "daemon", Action: "stop"},
						pubsub.Label{"id", "daemon"}, a.LabelLocalhost, labelOriginAPI)
					log.Infof("succeed")
					return JSONProblem(eCtx, http.StatusOK, "all local instances are in the shutdown state: daemon will stop immediately", "")
				}
				// all local instance shut down occurred but some has failed to shut down.
				log.Errorf("failed to shut down local instances: %v", shutdownFail)
				a.announceNodeState(log, node.MonitorStateShutdownFailure)
				resetLocalExpectOnError(pathsToResetOnFailure...)
				return JSONProblemf(eCtx, http.StatusInternalServerError, "daemon shutdown failure",
					"cannot stop daemon: failed to shut down local instances: %v", shutdownFail)
			}
		case <-shutdownCtx.Done():
			log.Errorf("failure: %s", shutdownCtx.Err())
			a.announceNodeState(log, node.MonitorStateShutdownFailure)
			resetLocalExpectOnError(pathsToResetOnFailure...)
			return JSONProblemf(eCtx, http.StatusInternalServerError, "daemon shutdown failure",
				"cannot stop daemon: waiting for local instances to shut down: %s", shutdownCtx.Err())
		}
	}
}
