package imon

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/toc"
)

type (
	todoMap map[string]bool

	// orchestrationResource manages the resource orchestration state, scheduling,
	// and actions for restart/monitoring.
	orchestrationResource struct {
		// scheduler represents a timer used to schedule the restart of a group
		// of resources with same standby value.
		scheduler *time.Timer

		// scheduled tracks the set of resources identified by their IDs that
		// are currently scheduled for a specific action.
		scheduled map[string]bool

		// standby indicates whether the resource is in a standby mode,
		// or in regular mode (non-standby).
		standby bool

		log *plog.Logger
	}
)

const (
	enableMonitorMsg  = "enable resource restart and monitoring"
	disableMonitorMsg = "disable resource restart and monitoring"
)

var (
	resourceOrchestrableKinds = naming.NewKinds(naming.KindSvc, naming.KindVol)
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
			t.loggerWithState().Tracef("%s: local expect is already %s", msg, localExpect)
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
		t.queueAction(t.crmStop, instance.MonitorStateStopProgress, instance.MonitorStateStartFailure, instance.MonitorStateStopFailure)
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
	t.publisher.Pub(
		&msgbus.InstanceMonitorAction{
			Path:   t.path,
			Node:   t.localhost,
			Action: action,
			RID:    rid,
		}, t.pubLabels...)
}

// orchestrateResourceRestart manages the restart orchestration process for resources,
// handling delays, timers, and retries.
func (t *Manager) orchestrateResourceRestart() {
	// only available for svc or vol
	if !resourceOrchestrableKinds.Has(t.path.Kind) {
		// discard non svc or vol
		return
	}

	// ignore if the instance status does not exist
	if _, ok := t.instStatus[t.localhost]; !ok {
		t.log.Infof("skip restart: missing instance status")
		t.cancelResourceOrchestrateSchedules()
		return
	}

	// ignore if the node is frozen
	if t.nodeStatus[t.localhost].IsFrozen() {
		t.log.Tracef("skip restart: node is frozen")
		t.cancelResourceOrchestrateSchedules()
		return
	}

	// ignore if the node is not idle
	if t.nodeMonitor[t.localhost].State != node.MonitorStateIdle {
		t.log.Tracef("skip restart: node is %s", t.nodeMonitor[t.localhost].State)
		t.cancelResourceOrchestrateSchedules()
		return
	}

	if t.state.LocalExpect == instance.MonitorLocalExpectEvicted && t.state.State == instance.MonitorStateStopFailure {
		if action, ok := t.getValidMonitorAction(1); ok {
			t.disableMonitor("initial monitor action failed, try alternate monitor action %s", action)
			t.doMonitorAction("", action)
		} else {
			t.disableMonitor("initial monitor action failed, no alternate monitor action")
		}
	}

	// ignore if the instance is frozen
	if t.instStatus[t.localhost].IsFrozen() {
		t.log.Tracef("skip restart: instance is frozen")
		t.cancelResourceOrchestrateSchedules()
		return
	}

	// ignore if the instance is not provisioned
	if instanceStatus := t.instStatus[t.localhost]; instanceStatus.Provisioned.IsOneOf(provisioned.False, provisioned.Mixed, provisioned.Undef) {
		t.log.Tracef("skip restart: provisioned is %s", instanceStatus.Provisioned)
		t.cancelResourceOrchestrateSchedules()
		return
	}

	// ignore if the instance has no monitor data
	instMonitor, ok := t.GetInstanceMonitor(t.localhost)
	if !ok {
		t.log.Tracef("skip restart: no instance monitor")
		t.cancelResourceOrchestrateSchedules()
		return
	}

	// ignore if the instance is not idle, start failed or stop failed.
	switch instMonitor.State {
	case instance.MonitorStateIdle, instance.MonitorStateStartFailure, instance.MonitorStateStopFailure:
		// pass
	default:
		t.log.Tracef("skip restart: state=%s", instMonitor.State)
		return
	}

	todoRestart := newTodoMap()
	todoStandby := newTodoMap()

	started := t.state.LocalExpect == instance.MonitorLocalExpectStarted

	for rid, rstat := range t.instStatus[t.localhost].Resources {
		// Encap node don't manage restart restart for encap resources. The
		// master node will manage.
		if rstat.IsEncap {
			continue
		}
		rcfg := t.instConfig.Resources.Get(rid)
		if rcfg == nil {
			continue
		}
		rmon := t.state.Resources.Get(rid)
		if rmon == nil {
			continue
		}
		needRestart, needMonitorAction, err := t.orchestrateResourcePlan(rid, rcfg, rmon, rstat, started)
		if err != nil {
			t.log.Errorf("orchestrate resource plan for resource %s: %s", rid, err)
			t.cancelResourceOrchestrateSchedules()
			return
		} else if needMonitorAction {
			t.setLocalExpect(instance.MonitorLocalExpectEvicted, "monitor action evicting: %s", disableMonitorMsg)
			t.doMonitorAction(rid, t.initialMonitorAction)
			t.cancelResourceOrchestrateSchedules()
		} else if needRestart {
			if rcfg.IsStandby {
				todoStandby.Add(rid)
			} else {
				todoRestart.Add(rid)
			}
		}
	}

	for rid, encapStatus := range t.instStatus[t.localhost].Encap {
		if rStatus, ok := t.instStatus[t.localhost].Resources[rid]; ok && !rStatus.Status.Is(status.Up, status.StandbyUp) {
			continue
		}
		for rid, rstat := range encapStatus.Resources {
			rcfg := t.instConfig.Resources.Get(rid)
			if rcfg == nil {
				continue
			}
			rmon := t.state.Resources.Get(rid)
			if rmon == nil {
				continue
			}
			needRestart, needMonitorAction, err := t.orchestrateResourcePlan(rid, rcfg, rmon, rstat, started)
			if err != nil {
				t.log.Errorf("orchestrate resource plan for resource %s: %s", rid, err)
				t.cancelResourceOrchestrateSchedules()
				return
			} else if needMonitorAction {
				t.setLocalExpect(instance.MonitorLocalExpectEvicted, "monitor action evicting: %s", disableMonitorMsg)
				t.doMonitorAction(rid, t.initialMonitorAction)
				t.cancelResourceOrchestrateSchedules()
			} else if needRestart {
				if rcfg.IsStandby {
					todoStandby.Add(rid)
				} else {
					todoRestart.Add(rid)
				}
			}
		}
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

	or := t.orchestrationResource(standby)
	or.log.Infof("schedule restart resources %v in %s", rids, delay)
	or.scheduler = time.AfterFunc(delay, func() {
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
		t.state.Resources.Set(rid, *rmon)
		t.change = true
		or.scheduled[rid] = true
	}

}

// resourceRestart restarts the specified resources and updates their state in the resource monitor.
// Accepts a list of resource IDs and a boolean indicating if standby mode should be used.
// Queues the appropriate start operation and initiates a state transition.
func (t *Manager) resourceRestart(resourceRids []string, standby bool) {
	now := time.Now()
	rids := make([]string, 0, len(resourceRids))
	or := t.orchestrationResource(standby)
	for _, rid := range resourceRids {
		rmon := t.state.Resources.Get(rid)
		if rmon == nil {
			continue
		}

		if _, ok := or.scheduled[rid]; !ok {
			or.log.Infof("drop restart rid %s: not anymore candidate", rid)
			continue
		}
		rids = append(rids, rid)
		rmon.Restart.LastAt = now
		t.state.Resources.Set(rid, *rmon)
		t.change = true
		delete(or.scheduled, rid)
	}
	if len(rids) == 0 {
		or.log.Infof("abort restart: no more candidates")
		return
	}
	queueFunc := t.queueResourceStart
	if standby {
		queueFunc = t.queueResourceStartStandby
	}
	action := func() error {
		return queueFunc(rids)
	}
	t.doTransitionAction(action, instance.MonitorStateStartProgress, instance.MonitorStateIdle, instance.MonitorStateStartFailure)
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
		instance.MonitorActionNone,
		instance.MonitorActionReboot,
		instance.MonitorActionSwitch:
		return true
	default:
		t.log.Infof("unsupported monitor action: %s", action)
		return false
	}
}

// orchestrationResource selects and returns the appropriate orchestration resource
// based on the standby mode flag provided.
func (t *Manager) orchestrationResource(standby bool) *orchestrationResource {
	if standby {
		return &t.standbyResourceOrchestrate
	} else {
		return &t.regularResourceOrchestrate
	}
}

func (t *Manager) cancelResourceOrchestrateSchedules() {
	t.standbyResourceOrchestrate.cancelSchedule()
	t.regularResourceOrchestrate.cancelSchedule()
}

// orchestrateResourcePlan determines the plan for resource from its configuration and state.
// It returns flags indicating if a restart, or a monitor action is needed.
// the monitor action is needed when all the following conditions are met:
//
//	    the resource status is not in [NotApplicable, Undef, Up, StandbyUp]
//	    the started is true or the resource configuration is standby
//	    the remaining restarts is 0
//		the `monitor` value is true
//		the `monitor_action` is not `MonitorActionNone`
func (t *Manager) orchestrateResourcePlan(rid string, rcfg *instance.ResourceConfig, rmon *instance.ResourceMonitor, rStatus resource.Status, started bool) (needRestart, needMonitorAction bool, err error) {
	if rcfg == nil {
		err = fmt.Errorf("orchestrate resource plan called with nil resource monitor")
		return
	} else if rmon == nil {
		err = fmt.Errorf("orchestrate resource plan called with nil resource config")
		return
	}

	or := t.orchestrationResource(rcfg.IsStandby)

	dropScheduled := func(rid string, reason string) {
		if changed := or.dropScheduled(rid, reason); changed {
			t.change = true
		}
	}

	resetRemaining := func(rid string, reason string) {
		if rmon.Restart.Remaining != rcfg.Restart {
			or.log.Infof("rid %s %s: reset restart count to config value (%d -> %d)", rid, reason, rmon.Restart.Remaining, rcfg.Restart)
			rmon.Restart.Remaining = rcfg.Restart
			// reset the last monitor action execution time, to rearm the next monitor action
			t.state.MonitorActionExecutedAt = time.Time{}
			t.state.Resources.Set(rid, *rmon)
			t.change = true
		}
	}

	switch {
	case rcfg.IsDisabled:
		reason := "is disabled"
		or.log.Tracef("planFor rid %s skipped: %s", rid, reason)
		dropScheduled(rid, reason)
		resetRemaining(rid, reason)
	case rStatus.IsStopped:
		reason := fmt.Sprintf("resource is explicitely stopped")
		or.log.Tracef("planFor rid %s skipped: %s", rid, reason)
		dropScheduled(rid, reason)
		resetRemaining(rid, reason)
	case rStatus.Status.Is(status.NotApplicable, status.Undef, status.Up, status.StandbyUp):
		reason := fmt.Sprintf("status is %s", rStatus.Status)
		or.log.Tracef("planFor rid %s skipped: %s", rid, reason)
		dropScheduled(rid, reason)
		resetRemaining(rid, reason)
	case or.alreadyScheduled(rid):
		or.log.Tracef("planFor rid %s skipped: already scheduled", rid)
	case t.monitorActionCalled():
		or.log.Tracef("planFor rid %s skipped: monitor action has been already called", rid)
	case rcfg.IsStandby || started:
		if rmon.Restart.Remaining == 0 && rcfg.IsMonitored {
			or.log.Infof("rid %s status %s, restart remaining %d out of %d: need monitor action", rid, rStatus.Status, rmon.Restart.Remaining, rcfg.Restart)
			needMonitorAction = true
		} else if rmon.Restart.Remaining > 0 {
			or.log.Infof("rid %s status %s, restart remaining %d out of %d", rid, rStatus.Status, rmon.Restart.Remaining, rcfg.Restart)
			needRestart = true
		}
	default:
		dropScheduled(rid, "not standby or instance is not started")
	}
	return
}

// cancelSchedule stops and clears any active scheduler associated with the
// orchestration resource. Logs the cancellation action.
func (or *orchestrationResource) cancelSchedule() {
	if or.scheduler != nil {
		or.log.Infof("cancel previously scheduled restart")
		or.scheduler.Stop()
		or.scheduler = nil
	}
}

// dropScheduled removes a resource from the scheduled list using its ID and
// logs the action with the given reason.
// If the scheduled list becomes empty, it cancels any pending schedule.
// Returns true if the resource was found and removed, otherwise returns false.
func (or *orchestrationResource) dropScheduled(rid string, reason string) (change bool) {
	if _, ok := or.scheduled[rid]; ok {
		delete(or.scheduled, rid)
		change = true
		or.log.Infof("rid %s %s: drop delayed restart", rid, reason)
		if len(or.scheduled) == 0 {
			or.cancelSchedule()
		}
	}
	return
}

// alreadyScheduled returns true if a resource, identified by its ID, has been
// scheduled for a restart (is already in the scheduled map).
func (or *orchestrationResource) alreadyScheduled(rid string) bool {
	_, ok := or.scheduled[rid]
	return ok
}
