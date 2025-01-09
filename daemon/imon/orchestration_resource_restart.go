package imon

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
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

const (
	enableMonitorMsg  = "enable resource restart and monitoring"
	disableMonitorMsg = "disable resource restart and monitoring"
)

// disableMonitor disables the resource restart and monitoring by setting
// the local expectation to "none".
// format is used to log changing reason, format == "" => no logging.
func (t *Manager) disableMonitor(format string, a ...any) bool {
	if format != "" {
		format = format + ": %s"
		a = append(a, disableMonitorMsg)
	}
	return t.setLocalExpect(instance.MonitorLocalExpectNone, format, a...)
}

// enableMonitor resets the monitor action execution time and sets the
// local expected state to "Started" with a message.
// It resets the MonitorActionExecutedAt on each call to always rearm the
// next monitor action.
// format is used to log changing reason, format == "" => no logging.
func (t *Manager) enableMonitor(format string, a ...any) bool {
	if format != "" {
		format = format + ": %s"
		a = append(a, enableMonitorMsg)
	}
	// reset the last monitor action execution time, to rearm the next monitor action
	t.state.MonitorActionExecutedAt = time.Time{}
	return t.setLocalExpect(instance.MonitorLocalExpectStarted, format, a...)
}

// setLocalExpect sets the local expect value for monitoring.
// format is used to log changing reason, format == "" => no logging.
func (t *Manager) setLocalExpect(localExpect instance.MonitorLocalExpect, format string, a ...any) bool {
	if t.state.LocalExpect != localExpect {
		t.change = true
		if format != "" {
			t.loggerWithState().Infof(format, a...)
		}
		t.state.LocalExpect = localExpect
		return true
	} else {
		if format != "" {
			msg := fmt.Sprintf(format, a...)
			t.loggerWithState().Debugf("%s: local expect is already %s", msg, localExpect)
		}
		return false
	}
}

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

func (t *Manager) monitorActionCalled() bool {
	return !t.state.MonitorActionExecutedAt.IsZero()
}

func (t *Manager) doMonitorAction(rid string, action instance.MonitorAction) {
	t.state.MonitorActionExecutedAt = time.Now()
	if !t.isValidMonitorAction(action) {
		return
	}
	if err := t.doPreMonitorAction(); err != nil {
		t.log.Errorf("pre monitor action: %s", err)
	}

	t.log.Infof("do monitor action: %s", action)
	t.pubMonitorAction(rid, action)

	switch action {
	case instance.MonitorActionCrash:
		if err := toc.Crash(); err != nil {
			t.log.Errorf("monitor action %s: %s", action, err)
		}
	case instance.MonitorActionFreezeStop:
		t.doFreezeStop()
		t.doStop()
	case instance.MonitorActionReboot:
		if err := toc.Reboot(); err != nil {
			t.log.Errorf("monitor action %s: %s", action, err)
		}
	case instance.MonitorActionSwitch:
		t.createPendingWithDuration(stopDuration)
		t.disableMonitor("monitor action switch stopping")
		t.queueAction(t.crmStop, instance.MonitorStateStopping, instance.MonitorStateStartFailed, instance.MonitorStateStopFailed)
	}
}

// doPreMonitorAction executes a user-defined command before imon runs the
// MonitorAction. This command can detect a situation where the MonitorAction
// can not succeed, and decide to do another action.
func (t *Manager) doPreMonitorAction() error {
	if t.instConfig.PreMonitorAction == "" {
		return nil
	}
	t.log.Infof("execute pre monitor action: %s", t.instConfig.PreMonitorAction)
	cmdArgs, err := command.CmdArgsFromString(t.instConfig.PreMonitorAction)
	if err != nil {
		return err
	}
	if len(cmdArgs) == 0 {
		return nil
	}
	cmd := command.New(
		command.WithName(cmdArgs[0]),
		command.WithVarArgs(cmdArgs[1:]...),
		command.WithLogger(t.log),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithTimeout(60*time.Second),
	)
	return cmd.Run()
}

func (t *Manager) pubMonitorAction(rid string, action instance.MonitorAction) {
	t.pubsubBus.Pub(
		&msgbus.InstanceMonitorAction{
			Path:   t.path,
			Node:   t.localhost,
			Action: action,
			RID:    rid,
		},
		t.labelPath,
		t.labelLocalhost)
}

// orchestrateResourceRestart manages the restart orchestration process for resources,
// handling delays, timers, and retries.
func (t *Manager) orchestrateResourceRestart() {
	todoRestart := newTodoMap()
	todoStandby := newTodoMap()

	dropScheduled := func(rid string, standby bool, reason string) {
		var scheduled map[string]bool
		var msg string
		if standby {
			scheduled = t.resourceStandbyWithRestartScheduled
			msg = "delayed restart standby"
		} else {
			scheduled = t.resourceWithRestartScheduled
			msg = "delayed restart"
		}
		if _, ok := scheduled[rid]; ok {
			delete(scheduled, rid)
			t.change = true
			t.log.Infof("resource %s %s, reset %s", rid, reason, msg)
			if len(scheduled) == 0 {
				t.resetResourceMonitorTimer(standby)
			}
		}
	}

	resetRemaining := func(rid string, rcfg *instance.ResourceConfig, rmon *instance.ResourceMonitor, reason string) {
		if rmon.Restart.Remaining != rcfg.Restart {
			t.log.Infof("resource %s %s: reset restart count to the max (%d -> %d)", rid, reason, rmon.Restart.Remaining, rcfg.Restart)
			rmon.Restart.Remaining = rcfg.Restart
			// reset the last monitor action execution time, to rearm the next monitor action
			t.state.MonitorActionExecutedAt = time.Time{}
			t.state.Resources.Set(rid, *rmon)
			t.change = true
		}
	}

	resetRemainingAndTimer := func(rid string, rcfg *instance.ResourceConfig, rmon *instance.ResourceMonitor, reason string) {
		resetRemaining(rid, rcfg, rmon, reason)
		if rcfg != nil {
			dropScheduled(rid, rcfg.IsStandby, reason)
		}
	}

	planFor := func(rid string, resStatus status.T, started bool) {
		rcfg := t.instConfig.Resources.Get(rid)
		rmon := t.state.Resources.Get(rid)
		_, aleadyScheduled := t.resourceWithRestartScheduled[rid]
		_, aleadyScheduledStandby := t.resourceStandbyWithRestartScheduled[rid]
		switch {
		case rcfg == nil:
			return
		case rmon == nil:
			return
		case rcfg.IsDisabled:
			t.log.Debugf("resource %s restart skip: is disabled", rid, rcfg.IsDisabled)
			resetRemainingAndTimer(rid, rcfg, rmon, "is disabled")
		case resStatus.Is(status.NotApplicable, status.Undef, status.Up, status.StandbyUp):
			t.log.Debugf("resource %s restart skip: status is %s", rid, resStatus)
			resetRemainingAndTimer(rid, rcfg, rmon, fmt.Sprintf("status is %s", resStatus))
		case aleadyScheduledStandby && rcfg.IsStandby:
			t.log.Debugf("resource %s restart skipped: already registered for restart standby", rid)
		case aleadyScheduled && !rcfg.IsStandby:
			t.log.Debugf("resource %s restart skipped: already registered for restart", rid)
		case t.monitorActionCalled():
			t.log.Debugf("resource %s restart skip: already ran the monitor action", rid)
		case rcfg.IsStandby || started:
			if rmon.Restart.Remaining == 0 && rcfg.IsMonitored && t.initialMonitorAction != instance.MonitorActionNone {
				t.log.Infof("resource %s status %s, restart remaining %d out of %d", rid, resStatus, rmon.Restart.Remaining, rcfg.Restart)
				t.setLocalExpect(instance.MonitorLocalExpectEvicted, "monitor action evicting: %s", disableMonitorMsg)
				t.doMonitorAction(rid, t.initialMonitorAction)
			} else if rmon.Restart.Remaining > 0 {
				if rcfg.IsStandby {
					t.log.Infof("resource %s status %s, standby restart remaining %d out of %d", rid, resStatus, rmon.Restart.Remaining, rcfg.Restart)
					todoStandby.Add(rid)
				} else {
					t.log.Infof("resource %s status %s, restart remaining %d out of %d", rid, resStatus, rmon.Restart.Remaining, rcfg.Restart)
					todoRestart.Add(rid)
				}
			}
		default:
			dropScheduled(rid, rcfg.IsStandby, "not standby or instance is not started")
		}
	}

	// discard the cluster object
	if t.path.String() == "cluster" {
		return
	}

	// discard all except svc and vol
	switch t.path.Kind {
	case naming.KindSvc, naming.KindVol:
	default:
		return
	}

	// discard if the instance status does not exist
	if _, ok := t.instStatus[t.localhost]; !ok {
		t.log.Errorf("skip restart: missing instance status")
		t.resetResourceMonitorTimers()
		return
	}

	// don't run on frozen nodes
	if t.nodeStatus[t.localhost].IsFrozen() {
		t.log.Errorf("skip restart: node is frozen")
		t.resetResourceMonitorTimers()
		return
	}

	// don't run when the node is not idle
	if t.nodeMonitor[t.localhost].State != node.MonitorStateIdle {
		t.log.Errorf("skip restart: node is %s", t.nodeMonitor[t.localhost].State)
		t.resetResourceMonitorTimers()
		return
	}

	if t.state.LocalExpect == instance.MonitorLocalExpectEvicted && t.state.State == instance.MonitorStateStopFailed {
		if action, ok := t.getValidMonitorAction(1); ok {
			t.disableMonitor("initial monitor action failed, try alternate monitor action %s", action)
			t.doMonitorAction("", action)
		} else {
			t.disableMonitor("initial monitor action failed, no alternate monitor action")
		}
	}

	// don't run on frozen instances
	if t.instStatus[t.localhost].IsFrozen() {
		t.log.Errorf("skip restart: instance is frozen")
		t.resetResourceMonitorTimers()
		return
	}

	// discard not provisioned
	if instanceStatus := t.instStatus[t.localhost]; instanceStatus.Provisioned.IsOneOf(provisioned.False, provisioned.Mixed, provisioned.Undef) {
		t.log.Errorf("skip restart: provisioned is %s", instanceStatus.Provisioned)
		t.resetResourceMonitorTimers()
		return
	}

	// discard if the instance has no monitor data
	instMonitor, ok := t.GetInstanceMonitor(t.localhost)
	if !ok {
		t.log.Errorf("skip restart: no instance monitor")
		t.resetResourceMonitorTimers()
		return
	}

	// discard if the instance is not idle, start failed or stop failed.
	switch instMonitor.State {
	case instance.MonitorStateIdle, instance.MonitorStateStartFailed, instance.MonitorStateStopFailed:
		// pass
	default:
		t.log.Debugf("skip restart: state=%s", instMonitor.State)
		return
	}

	started := instMonitor.LocalExpect == instance.MonitorLocalExpectStarted

	for rid, rstat := range t.instStatus[t.localhost].Resources {
		planFor(rid, rstat.Status, started)
	}

	// Prepare scheduled resource restart
	if len(todoStandby) > 0 {
		t.resourceRestartSchedule(todoStandby, true)
	}
	if len(todoRestart) > 0 {
		t.resourceRestartSchedule(todoRestart, false)
	}
}

// resourceRestartSchedule schedules a restart for resources based on the provided resource map and standby mode.
// It updates the state of resources with associated restart timers and logs the operation.
func (t *Manager) resourceRestartSchedule(todo todoMap, standby bool) {
	var scheduled map[string]bool
	rids, delay := t.getRidsAndDelay(todo)
	if len(rids) == 0 {
		return
	}
	onTimer := func() {
		t.cmdC <- cmdResourceRestart{
			rids:    rids,
			standby: standby,
		}
	}
	if standby {
		t.log.Infof("schedule restart standby resources %v in %s", rids, delay)
		t.resourceStandbyRestartTimer = time.AfterFunc(delay, onTimer)
		scheduled = t.resourceStandbyWithRestartScheduled
	} else {
		t.log.Infof("schedule restart resources %v in %s", rids, delay)
		t.resourceRestartTimer = time.AfterFunc(delay, onTimer)
		scheduled = t.resourceWithRestartScheduled
	}
	for _, rid := range rids {
		rmon := t.state.Resources.Get(rid)
		if rmon == nil {
			continue
		}
		rmon.DecRestartRemaining()
		t.state.Resources.Set(rid, *rmon)
		t.change = true
		scheduled[rid] = true
	}

}

// resourceRestart restarts the specified resources and updates their state in the resource monitor.
// Accepts a list of resource IDs and a boolean indicating if standby mode should be used.
// Queues the appropriate start operation and initiates a state transition.
func (t *Manager) resourceRestart(resourceRids []string, standby bool) {
	now := time.Now()
	rids := make([]string, 0, len(resourceRids))
	var scheduled map[string]bool
	var skipMessage string
	if standby {
		skipMessage = "skip resource restart standby"
		scheduled = t.resourceStandbyWithRestartScheduled
	} else {
		skipMessage = "skip resource restart"
		scheduled = t.resourceWithRestartScheduled
	}
	for _, rid := range resourceRids {
		rmon := t.state.Resources.Get(rid)
		if rmon == nil {
			continue
		}

		if _, ok := scheduled[rid]; !ok {
			t.log.Infof("%s %s: not anymore candidate", skipMessage, rid)
			continue
		}
		rids = append(rids, rid)
		rmon.Restart.LastAt = now
		t.state.Resources.Set(rid, *rmon)
		t.change = true
		delete(scheduled, rid)
	}
	if len(rids) == 0 {
		t.log.Infof("%s: no more candidates", skipMessage)
		return
	}
	queueFunc := t.queueResourceStart
	if standby {
		queueFunc = t.queueResourceStartStandby
	}
	action := func() error {
		return queueFunc(rids)
	}
	t.doTransitionAction(action, instance.MonitorStateStarting, instance.MonitorStateIdle, instance.MonitorStateStartFailed)
}

// getRidsAndDelay processes a todoMap to retrieve resource IDs and calculates the maximum required restart delay.
func (t *Manager) getRidsAndDelay(todo todoMap) ([]string, time.Duration) {
	var maxDelay time.Duration
	rids := make([]string, 0)
	now := time.Now()
	for rid := range todo {
		rcfg := t.instConfig.Resources.Get(rid)
		if rcfg == nil {
			continue
		}
		rmon := t.state.Resources.Get(rid)
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

func (t *Manager) getValidMonitorAction(stage int) (action instance.MonitorAction, ok bool) {
	if stage >= len(t.instConfig.MonitorAction) {
		return
	}
	action = t.instConfig.MonitorAction[stage]
	ok = t.isValidMonitorAction(action)
	return
}

func (t *Manager) isValidMonitorAction(action instance.MonitorAction) bool {
	switch action {
	case instance.MonitorActionCrash,
		instance.MonitorActionFreezeStop,
		instance.MonitorActionReboot,
		instance.MonitorActionSwitch,
		instance.MonitorActionNoOp:
		return true
	case instance.MonitorActionNone:
		return false
	default:
		t.log.Infof("unsupported monitor action: %s", action)
		return false
	}
}

func (t *Manager) resetResourceMonitorTimers() {
	t.resetResourceMonitorTimer(true)
	t.resetResourceMonitorTimer(false)
}

func (t *Manager) resetResourceMonitorTimer(standby bool) {
	if standby && t.resourceStandbyRestartTimer != nil {
		t.log.Infof("reset scheduled restart standby resources")
		t.resourceStandbyRestartTimer.Stop()
		t.resourceStandbyRestartTimer = nil
	} else if !standby && t.resourceRestartTimer != nil {
		t.log.Infof("reset scheduled restart resources")
		t.resourceRestartTimer.Stop()
		t.resourceRestartTimer = nil
	}
}
