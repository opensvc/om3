package imon

import (
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
	"opensvc.com/opensvc/util/toc"
)

type (
	restartTodoMap map[string]bool
)

func (t restartTodoMap) Add(rid string) {
	t[rid] = true
}

func (t restartTodoMap) Del(rid string) {
	delete(t, rid)
}

func (t restartTodoMap) IsEmpty() bool {
	return len(t) == 0
}

func newRestartTodoMap() restartTodoMap {
	m := make(restartTodoMap)
	return m
}

func (o *imon) orchestrateResourceRestart() {
	todo := newRestartTodoMap()
	pubMonitorAction := func(rid string) {
		bus := pubsub.BusFromContext(o.ctx)
		bus.Pub(msgbus.InstanceMonitorAction{
			Path:   o.path,
			Node:   hostname.Hostname(),
			Action: o.instConfig.MonitorAction,
			RID:    rid,
		}, pubsub.Label{"path", o.path.String()}, pubsub.Label{"node", hostname.Hostname()})
	}
	doMonitorAction := func(rid string) {
		if o.instConfig.MonitorAction != "" {
			o.log.Info().Msgf("do %s monitor action", o.instConfig.MonitorAction)
			pubMonitorAction(rid)
		}
		switch o.instConfig.MonitorAction {
		case instance.MonitorActionCrash:
			if err := toc.Crash(); err != nil {
				o.log.Error().Err(err).Msg("monitor action")
			}
		case instance.MonitorActionFreezeStop:
			o.doFreezeStop()
			o.doStop()
		case instance.MonitorActionReboot:
			if err := toc.Reboot(); err != nil {
				o.log.Error().Err(err).Msg("monitor action")
			}
		case instance.MonitorActionSwitch:
			o.createPendingWithDuration(stopDuration)
			o.doAction(o.crmStop, instance.MonitorStateStopping, instance.MonitorStateStartFailed, instance.MonitorStateStopFailed)
		}
	}
	resetTimer := func(rid string) {
		todo.Del(rid)
		if timer, ok := o.state.Resources.GetRestartTimer(rid); ok && timer != nil {
			o.log.Info().Msgf("resource %s is up, reset delayed restart", rid)
			timer.Stop()
			o.state.Resources.SetRestartTimer(rid, nil)
			o.change = true
		}
	}
	resetRemaining := func(rid string) {
		rcfg, ok := o.instConfig.Resources[rid]
		if !ok {
			return
		}
		if remaining, ok := o.state.Resources.GetRestartRemaining(rid); ok && remaining != rcfg.Restart {
			o.log.Info().Msgf("resource %s is up, reset restart count to the max (%d -> %d)", rid, remaining, rcfg.Restart)
			o.state.MonitorActionExecutedAt = time.Time{}
			o.state.Resources.SetRestartRemaining(rid, rcfg.Restart)
			o.change = true
		}
	}
	resetRemainingAndTimer := func(rid string) {
		resetRemaining(rid)
		resetTimer(rid)
	}
	resetTimers := func() {
		for _, res := range o.instStatus[o.localhost].Resources {
			resetTimer(res.Rid)
		}
	}
	planFor := func(rid string, resStatus status.T) {
		rcfg, ok := o.instConfig.Resources[rid]
		if !ok {
			return
		}
		switch {
		case rcfg.IsDisabled == true:
			o.log.Debug().Msgf("resource %s restart skip: disable=%v", rid, rcfg.IsDisabled)
			resetRemainingAndTimer(rid)
		case resStatus.Is(status.NotApplicable, status.Undef):
			o.log.Debug().Msgf("resource %s restart skip: status=%s", rid, resStatus)
			resetRemainingAndTimer(rid)
		case resStatus.Is(status.Up, status.StandbyUp):
			o.log.Debug().Msgf("resource %s restart skip: status=%s", rid, resStatus)
			resetRemainingAndTimer(rid)
		case o.state.Resources.HasRestartTimer(rid):
			o.log.Debug().Msgf("resource %s restart skip: already has a delay timer", rid)
		case !o.state.MonitorActionExecutedAt.IsZero():
			o.log.Debug().Msgf("resource %s restart skip: already ran the monitor action", rid)
		default:
			rmon := o.state.Resources[rid]
			o.log.Info().Msgf("resource %s status %s, restart remaining %d out of %d", rid, resStatus, rmon.Restart.Remaining, rcfg.Restart)
			if rmon.Restart.Remaining == 0 {
				o.state.MonitorActionExecutedAt = time.Now()
				o.change = true
				doMonitorAction(rid)
			} else {
				todo.Add(rid)
			}
		}
	}
	do := func() {
		if todo.IsEmpty() {
			return
		}
		rids := make([]string, 0)
		now := time.Now()
		var maxDelay time.Duration
		for rid, _ := range todo {
			rcfg := o.instConfig.Resources[rid]
			rmon := o.state.Resources[rid]
			if rcfg.RestartDelay != nil {
				notBefore := rmon.Restart.LastAt.Add(*rcfg.RestartDelay)
				if now.Before(notBefore) {
					delay := notBefore.Sub(now)
					if delay > maxDelay {
						maxDelay = delay
					}
				}
			}
			rids = append(rids, rid)
			o.state.Resources.DecRestartRemaining(rid)
			o.change = true
		}
		timer := time.AfterFunc(maxDelay, func() {
			now := time.Now()
			for _, rid := range rids {
				o.state.Resources.SetRestartLastAt(rid, now)
				o.state.Resources.SetRestartTimer(rid, nil)
				o.change = true
			}
			action := func() error {
				return o.crmResourceStart(rids)
			}
			o.doTransitionAction(action, instance.MonitorStateStarting, instance.MonitorStateIdle, instance.MonitorStateStartFailed)
		})
		for _, rid := range rids {
			o.state.Resources.SetRestartTimer(rid, timer)
			o.change = true
		}
	}

	// discard the cluster object
	if o.path.String() == "cluster" {
		return
	}

	// discard all execpt svc and vol
	switch o.path.Kind {
	case kind.Svc, kind.Vol:
	default:
		return
	}

	// discard if the instance status does not exist
	if _, ok := o.instStatus[o.localhost]; !ok {
		resetTimers()
		return
	}

	// don't run on frozen nodes
	if o.nodeStatus[o.localhost].IsFrozen() {
		resetTimers()
		return
	}

	// don't run when the node is not idle
	if o.nodeMonitor[o.localhost].State != cluster.NodeMonitorStateIdle {
		resetTimers()
		return
	}

	// don't run on frozen instances
	if o.instStatus[o.localhost].IsFrozen() {
		resetTimers()
		return
	}

	// discard not provisioned
	if instanceStatus := o.instStatus[o.localhost]; instanceStatus.Provisioned.IsOneOf(provisioned.False, provisioned.Mixed, provisioned.Undef) {
		o.log.Debug().Msgf("skip restart: provisioned=%s", instanceStatus.Provisioned)
		resetTimers()
		return
	}

	// discard if the instance is not idle,started
	if instMonitor, ok := o.GetInstanceMonitor(o.localhost); !ok {
		o.log.Debug().Msgf("skip restart: no instance monitor")
		resetTimers()
		return
	} else {
		switch instMonitor.State {
		case instance.MonitorStateIdle, instance.MonitorStateStartFailed:
			// pass
		default:
			o.log.Debug().Msgf("skip restart: state=%s", instMonitor.State)
			return
		}
		if instMonitor.LocalExpect != instance.MonitorLocalExpectStarted {
			o.log.Debug().Msgf("skip restart: local_expect=%s", instMonitor.LocalExpect)
			resetTimers()
			return
		}
	}

	for _, res := range o.instStatus[o.localhost].Resources {
		planFor(res.Rid, res.Status)
	}
	do()
}
