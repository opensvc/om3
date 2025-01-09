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

func (t *Manager) doMonitorAction(rid string, stage int) {
	t.state.MonitorActionExecutedAt = time.Now()
	monitorActionCount := len(t.instConfig.MonitorAction)
	if monitorActionCount < stage+1 {
		t.log.Errorf("skip monitor action: stage %d action no longer configured", stage+1)
		return
	}

	monitorAction := t.instConfig.MonitorAction[stage]

	switch monitorAction {
	case instance.MonitorActionCrash:
	case instance.MonitorActionFreezeStop:
	case instance.MonitorActionReboot:
	case instance.MonitorActionSwitch:
	case instance.MonitorActionNone:
		t.log.Infof("skip monitor action: not configured")
		return
	default:
		t.log.Errorf("skip monitor action: not supported: %s", monitorAction)
		return
	}

	if err := t.doPreMonitorAction(); err != nil {
		t.log.Errorf("pre monitor action: %s", err)
	}

	t.log.Infof("do monitor action %d/%d: %s", stage+1, len(t.instConfig.MonitorAction), monitorAction)
	t.pubMonitorAction(rid, monitorAction)

	switch monitorAction {
	case instance.MonitorActionCrash:
		if err := toc.Crash(); err != nil {
			t.log.Errorf("monitor action: %s", err)
		}
	case instance.MonitorActionFreezeStop:
		t.doFreezeStop()
		t.doStop()
	case instance.MonitorActionReboot:
		if err := toc.Reboot(); err != nil {
			t.log.Errorf("monitor action: %s", err)
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

	resetTimer := func(rid string, rmon *instance.ResourceMonitor) {
		todoRestart.Del(rid)
		todoStandby.Del(rid)
		if rmon.Restart.Timer != nil {
			t.log.Infof("resource %s is up, reset delayed restart", rid)
			t.change = rmon.StopRestartTimer()
			t.state.Resources.Set(rid, *rmon)
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
		resetTimer(rid, rmon)
	}

	resetTimers := func() {
		for rid, rmon := range t.state.Resources {
			resetTimer(rid, &rmon)
		}
	}

	planFor := func(rid string, resStatus status.T, started bool) {
		rcfg := t.instConfig.Resources.Get(rid)
		rmon := t.state.Resources.Get(rid)
		switch {
		case rcfg == nil:
			return
		case rmon == nil:
			return
		case rcfg.IsDisabled:
			t.log.Debugf("resource %s restart skip: disable=%v", rid, rcfg.IsDisabled)
			resetRemainingAndTimer(rid, rcfg, rmon, "is disabled")
		case resStatus.Is(status.NotApplicable, status.Undef, status.Up, status.StandbyUp):
			t.log.Debugf("resource %s restart skip: status=%s", rid, resStatus)
			resetRemainingAndTimer(rid, rcfg, rmon, fmt.Sprintf("status is %s", resStatus))
		case rmon.Restart.Timer != nil:
			t.log.Debugf("resource %s restart skip: already has a delay timer", rid)
		case t.monitorActionCalled():
			t.log.Debugf("resource %s restart skip: already ran the monitor action", rid)
		case rcfg.IsStandby:
			t.log.Infof("resource %s status %s, standby restart remaining %d out of %d", rid, resStatus, rmon.Restart.Remaining, rcfg.Restart)
			todoStandby.Add(rid)
		case started:
			if rmon.Restart.Remaining == 0 && rcfg.IsMonitored {
				t.log.Infof("resource %s status %s, restart remaining %d out of %d", rid, resStatus, rmon.Restart.Remaining, rcfg.Restart)
				t.setLocalExpect(instance.MonitorLocalExpectEvicted, "monitor action evicting: %s", disableMonitorMsg)
				t.doMonitorAction(rid, 0)
			} else if rmon.Restart.Remaining > 0 {
				t.log.Infof("resource %s status %s, restart remaining %d out of %d", rid, resStatus, rmon.Restart.Remaining, rcfg.Restart)
				todoRestart.Add(rid)
			}
		default:
			t.log.Debugf("resource %s restart skip: instance not started", rid)
			resetTimer(rid, rmon)
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
		resetTimers()
		return
	}

	// don't run on frozen nodes
	if t.nodeStatus[t.localhost].IsFrozen() {
		resetTimers()
		return
	}

	// don't run when the node is not idle
	if t.nodeMonitor[t.localhost].State != node.MonitorStateIdle {
		resetTimers()
		return
	}

	if t.state.LocalExpect == instance.MonitorLocalExpectEvicted && t.state.State == instance.MonitorStateStopFailed {
		t.disableMonitor("orchestrate resource restart recover from evicted and stop failed")
		t.doMonitorAction("", 1)
	}

	// don't run on frozen instances
	if t.instStatus[t.localhost].IsFrozen() {
		resetTimers()
		return
	}

	// discard not provisioned
	if instanceStatus := t.instStatus[t.localhost]; instanceStatus.Provisioned.IsOneOf(provisioned.False, provisioned.Mixed, provisioned.Undef) {
		t.log.Debugf("skip restart: provisioned=%s", instanceStatus.Provisioned)
		resetTimers()
		return
	}

	// discard if the instance has no monitor data
	instMonitor, ok := t.GetInstanceMonitor(t.localhost)
	if !ok {
		t.log.Debugf("skip restart: no instance monitor")
		resetTimers()
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
	rids, delay := t.getRidsAndDelay(todo)
	if len(rids) == 0 {
		return
	}
	if standby {
		t.log.Infof("schedule restart standby resources %v in %s", rids, delay)
	} else {
		t.log.Infof("schedule restart resources %v in %s", rids, delay)
	}
	timer := time.AfterFunc(delay, func() {
		t.cmdC <- cmdResourceRestart{
			rids:    rids,
			standby: standby,
		}
	})
	for _, rid := range rids {
		rmon := t.state.Resources.Get(rid)
		if rmon == nil {
			continue
		}
		rmon.DecRestartRemaining()
		rmon.Restart.Timer = timer
		t.state.Resources.Set(rid, *rmon)
		t.change = true
	}
}

// resourceRestart restarts the specified resources and updates their state in the resource monitor.
// Accepts a list of resource IDs and a boolean indicating if standby mode should be used.
// Queues the appropriate start operation and initiates a state transition.
func (t *Manager) resourceRestart(resourceRids []string, standby bool) {
	now := time.Now()
	rids := make([]string, 0, len(resourceRids))
	for _, rid := range resourceRids {
		rmon := t.state.Resources.Get(rid)
		if rmon == nil {
			continue
		}
		rids = append(rids, rid)
		rmon.Restart.LastAt = now
		rmon.Restart.Timer = nil
		t.state.Resources.Set(rid, *rmon)
		t.change = true
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
