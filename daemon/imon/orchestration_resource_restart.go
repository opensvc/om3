package imon

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/kind"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/toc"
)

type (
	todoMap map[string]bool
)

func (t todoMap) Add(rid string) {
	t[rid] = true
}

func (t todoMap) Del(rid string) {
	delete(t, rid)
}

func (t todoMap) IsEmpty() bool {
	return len(t) == 0
}

func newTodoMap() todoMap {
	m := make(todoMap)
	return m
}

func (o *imon) orchestrateResourceRestart() {
	todoRestart := newTodoMap()
	todoStandby := newTodoMap()

	pubMonitorAction := func(rid string) {
		o.pubsubBus.Pub(
			&msgbus.InstanceMonitorAction{
				Path:   o.path,
				Node:   o.localhost,
				Action: o.instConfig.MonitorAction,
				RID:    rid,
			},
			o.labelPath,
			o.labelLocalhost)
	}

	// doPreMonitorAction executes a user-defined command before imon
	// runs the MonitorAction. This command can detect a situation where
	// the MonitorAction can not succeed, and decide to do another action.
	doPreMonitorAction := func() error {
		if o.instConfig.PreMonitorAction == "" {
			return nil
		}
		o.log.Info().Msgf("execute pre monitor action: %s", o.instConfig.PreMonitorAction)
		cmdArgs, err := command.CmdArgsFromString(o.instConfig.PreMonitorAction)
		if err != nil {
			return err
		}
		if len(cmdArgs) == 0 {
			return nil
		}
		cmd := command.New(
			command.WithName(cmdArgs[0]),
			command.WithVarArgs(cmdArgs[1:]...),
			command.WithLogger(&o.log),
			command.WithStdoutLogLevel(zerolog.InfoLevel),
			command.WithStderrLogLevel(zerolog.ErrorLevel),
			command.WithTimeout(60*time.Second),
		)
		return cmd.Run()
	}

	doMonitorAction := func(rid string) {
		switch o.instConfig.MonitorAction {
		case instance.MonitorActionCrash:
		case instance.MonitorActionFreezeStop:
		case instance.MonitorActionReboot:
		case instance.MonitorActionSwitch:
		case instance.MonitorActionNone:
			o.log.Error().Msgf("skip monitor action: not configured")
			return
		default:
			o.log.Error().Msgf("skip monitor action: not supported: %s", o.instConfig.MonitorAction)
			return
		}

		if err := doPreMonitorAction(); err != nil {
			o.log.Error().Err(err).Msg("pre monitor action")
		}

		o.log.Info().Msgf("do %s monitor action", o.instConfig.MonitorAction)
		pubMonitorAction(rid)

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

	resetTimer := func(rmon *instance.ResourceMonitor) {
		todoRestart.Del(rmon.Rid)
		todoStandby.Del(rmon.Rid)
		if rmon.Restart.Timer != nil {
			o.log.Info().Msgf("resource %s is up, reset delayed restart", rmon.Rid)
			o.change = rmon.StopRestartTimer()
			o.state.Resources.Set(*rmon)
		}
	}

	resetRemaining := func(rcfg *instance.ResourceConfig, rmon *instance.ResourceMonitor) {
		if rmon.Restart.Remaining != rcfg.Restart {
			o.log.Info().Msgf("resource %s is up, reset restart count to the max (%d -> %d)", rcfg.Rid, rmon.Restart.Remaining, rcfg.Restart)
			o.state.MonitorActionExecutedAt = time.Time{}
			rmon.Restart.Remaining = rcfg.Restart
			o.state.Resources.Set(*rmon)
			o.change = true
		}
	}

	resetRemainingAndTimer := func(rcfg *instance.ResourceConfig, rmon *instance.ResourceMonitor) {
		resetRemaining(rcfg, rmon)
		resetTimer(rmon)
	}

	resetTimers := func() {
		for _, rmon := range o.state.Resources {
			resetTimer(&rmon)
		}
	}

	planFor := func(rid string, resStatus status.T, started bool) {
		rcfg := o.instConfig.Resources.Get(rid)
		rmon := o.state.Resources.Get(rid)
		switch {
		case rcfg == nil:
			return
		case rmon == nil:
			return
		case rcfg.IsDisabled:
			o.log.Debug().Msgf("resource %s restart skip: disable=%v", rid, rcfg.IsDisabled)
			resetRemainingAndTimer(rcfg, rmon)
		case resStatus.Is(status.NotApplicable, status.Undef):
			o.log.Debug().Msgf("resource %s restart skip: status=%s", rid, resStatus)
			resetRemainingAndTimer(rcfg, rmon)
		case resStatus.Is(status.Up, status.StandbyUp):
			o.log.Debug().Msgf("resource %s restart skip: status=%s", rid, resStatus)
			resetRemainingAndTimer(rcfg, rmon)
		case rmon.Restart.Timer != nil:
			o.log.Debug().Msgf("resource %s restart skip: already has a delay timer", rid)
		case !o.state.MonitorActionExecutedAt.IsZero():
			o.log.Debug().Msgf("resource %s restart skip: already ran the monitor action", rid)
		case started:
			o.log.Info().Msgf("resource %s status %s, restart remaining %d out of %d", rid, resStatus, rmon.Restart.Remaining, rcfg.Restart)
			if rmon.Restart.Remaining == 0 {
				o.state.MonitorActionExecutedAt = time.Now()
				o.change = true
				doMonitorAction(rid)
			} else {
				todoRestart.Add(rid)
			}
		case rcfg.IsStandby:
			o.log.Info().Msgf("resource %s status %s, standby restart remaining %d out of %d", rid, resStatus, rmon.Restart.Remaining, rcfg.Restart)
			todoStandby.Add(rid)
		default:
			o.log.Debug().Msgf("resource %s restart skip: instance not started", rid)
			resetTimer(rmon)
		}
	}

	getRidsAndDelay := func(todo todoMap) ([]string, time.Duration) {
		var maxDelay time.Duration
		rids := make([]string, 0)
		now := time.Now()
		for rid, _ := range todo {
			rcfg := o.instConfig.Resources.Get(rid)
			if rcfg == nil {
				continue
			}
			rmon := o.state.Resources.Get(rid)
			if rmon == nil {
				continue
			}
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
		}
		return rids, maxDelay
	}

	doRestart := func() {
		rids, delay := getRidsAndDelay(todoRestart)
		if len(rids) == 0 {
			return
		}
		timer := time.AfterFunc(delay, func() {
			now := time.Now()
			for _, rid := range rids {
				rmon := o.state.Resources.Get(rid)
				if rmon == nil {
					continue
				}
				rmon.Restart.LastAt = now
				rmon.Restart.Timer = nil
				o.state.Resources.Set(*rmon)
				o.change = true
			}
			action := func() error {
				return o.crmResourceStart(rids)
			}
			o.doTransitionAction(action, instance.MonitorStateStarting, instance.MonitorStateIdle, instance.MonitorStateStartFailed)
		})
		for _, rid := range rids {
			rmon := o.state.Resources.Get(rid)
			if rmon == nil {
				continue
			}
			rmon.DecRestartRemaining()
			rmon.Restart.Timer = timer
			o.state.Resources.Set(*rmon)
			o.change = true
		}
	}

	doStandby := func() {
		rids, delay := getRidsAndDelay(todoStandby)
		if len(rids) == 0 {
			return
		}
		timer := time.AfterFunc(delay, func() {
			now := time.Now()
			for _, rid := range rids {
				rmon := o.state.Resources.Get(rid)
				if rmon == nil {
					continue
				}
				rmon.Restart.LastAt = now
				rmon.Restart.Timer = nil
				o.state.Resources.Set(*rmon)
				o.change = true
			}
			action := func() error {
				return o.crmResourceStartStandby(rids)
			}
			o.doTransitionAction(action, instance.MonitorStateStarting, instance.MonitorStateIdle, instance.MonitorStateStartFailed)
		})
		for _, rid := range rids {
			rmon := o.state.Resources.Get(rid)
			if rmon == nil {
				continue
			}
			rmon.DecRestartRemaining()
			rmon.Restart.Timer = timer
			o.state.Resources.Set(*rmon)
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
	if o.nodeMonitor[o.localhost].State != node.MonitorStateIdle {
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

	// discard if the instance has no monitor data
	instMonitor, ok := o.GetInstanceMonitor(o.localhost)
	if !ok {
		o.log.Debug().Msgf("skip restart: no instance monitor")
		resetTimers()
		return
	}

	// discard if the instance is not idle nor start failed
	switch instMonitor.State {
	case instance.MonitorStateIdle, instance.MonitorStateStartFailed:
		// pass
	default:
		o.log.Debug().Msgf("skip restart: state=%s", instMonitor.State)
		return
	}

	started := instMonitor.LocalExpect == instance.MonitorLocalExpectStarted

	for _, rstat := range o.instStatus[o.localhost].Resources {
		planFor(rstat.Rid, rstat.Status, started)
	}
	doStandby()
	doRestart()
}
