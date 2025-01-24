package imon

import (
	"slices"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
)

func (t *Manager) orchestrateFailoverPlacedStart() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.placedUnfreeze()
	case instance.MonitorStateUnfreezeSuccess:
		t.orchestrateFailoverPlacedStartFromUnfreezeSuccess()
	case instance.MonitorStateStartSuccess:
		t.orchestrateFailoverPlacedStartFromStarted()
	case instance.MonitorStateStopSuccess:
		t.orchestrateFailoverPlacedStartFromStopped()
	case instance.MonitorStateStartFailure:
		t.orchestratePlacedFromStartFailed()
	case instance.MonitorStateUnfreezeProgress:
	case instance.MonitorStateFreezeProgress:
	case instance.MonitorStateStopProgress:
	case instance.MonitorStateStartProgress:
	default:
		t.log.Errorf("don't know how to orchestrate placed start from %s", t.state.State)
	}
}

func (t *Manager) orchestrateFlexPlacedStart() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.placedUnfreeze()
	case instance.MonitorStateUnfreezeSuccess:
		t.orchestrateFlexPlacedStartFromUnfrozen()
	case instance.MonitorStateStartSuccess:
		t.orchestrateFlexPlacedStartFromStarted()
	case instance.MonitorStateStopSuccess:
		t.transitionTo(instance.MonitorStateIdle)
	case instance.MonitorStateStartFailure:
		t.orchestratePlacedFromStartFailed()
	case instance.MonitorStateUnfreezeProgress:
	case instance.MonitorStateFreezeProgress:
	case instance.MonitorStateStopProgress:
	case instance.MonitorStateStartProgress:
	default:
		t.log.Errorf("don't know how to orchestrate placed start from %s", t.state.State)
	}
}

func (t *Manager) orchestrateFailoverPlacedStop() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.placedUnfreeze()
	case instance.MonitorStateUnfreezeSuccess:
		t.placedStop()
	case instance.MonitorStateStopFailure:
		t.clearStopFailedIfDown()
	case instance.MonitorStateStopSuccess:
		t.clearStopped()
	case instance.MonitorStateReady:
		t.transitionTo(instance.MonitorStateIdle)
	case instance.MonitorStateStartFailure:
		t.orchestratePlacedFromStartFailed()
	case instance.MonitorStateUnfreezeProgress:
	case instance.MonitorStateFreezeProgress:
	case instance.MonitorStateStopProgress:
	case instance.MonitorStateStartProgress:
	default:
		t.log.Errorf("don't know how to orchestrate placed stop from %s", t.state.State)
	}
}

func (t *Manager) orchestrateFlexPlacedStop() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.placedUnfreeze()
	case instance.MonitorStateUnfreezeSuccess:
		t.placedStop()
	case instance.MonitorStateStopFailure:
		t.clearStopFailedIfDown()
	case instance.MonitorStateStopSuccess:
		t.clearStopped()
	case instance.MonitorStateReady:
		t.transitionTo(instance.MonitorStateIdle)
	case instance.MonitorStateStartFailure:
		t.orchestratePlacedFromStartFailed()
	case instance.MonitorStateUnfreezeProgress:
	case instance.MonitorStateFreezeProgress:
	case instance.MonitorStateStopProgress:
	case instance.MonitorStateStartProgress:
	default:
		t.log.Errorf("don't know how to orchestrate placed stop from %s", t.state.State)
	}
}

func (t *Manager) getPlacedAtDestination() ([]string, bool) {
	options, ok := t.state.GlobalExpectOptions.(instance.MonitorGlobalExpectOptionsPlacedAt)
	if !ok {
		return nil, ok
	}
	return options.Destination, true
}

func (t *Manager) orchestratePlacedAt() {
	dstNodes, ok := t.getPlacedAtDestination()
	if !ok {
		t.log.Errorf("missing placed@ destination")
		return
	}
	if slices.Contains(dstNodes, t.localhost) {
		t.orchestratePlacedStart()
	} else {
		t.orchestratePlacedStop()
	}
}

func (t *Manager) placedUnfreeze() {
	if t.instStatus[t.localhost].IsUnfrozen() {
		t.transitionTo(instance.MonitorStateUnfreezeSuccess)
	} else {
		t.doUnfreeze()
	}
}

func (t *Manager) doPlacedStart() {
	t.queueAction(t.crmStart, instance.MonitorStateStartProgress, instance.MonitorStateStartSuccess, instance.MonitorStateStartFailure)
}

func (t *Manager) placedStart() {
	instStatus := t.instStatus[t.localhost]
	switch instStatus.Avail {
	case status.Down, status.StandbyDown, status.StandbyUp:
		t.doPlacedStart()
	case status.Up, status.Warn:
		t.skipPlacedStart()
	default:
		return
	}
}

func (t *Manager) placedStop() {
	instStatus := t.instStatus[t.localhost]
	switch instStatus.Avail {
	case status.Down, status.StandbyDown, status.StandbyUp:
		t.skipPlacedStop()
	case status.Up, status.Warn:
		t.doPlacedStop()
	default:
		return
	}
}

func (t *Manager) doPlacedStop() {
	t.createPendingWithDuration(stopDuration)
	t.disableMonitor("orchestrate placed stopping")
	t.queueAction(t.crmStop, instance.MonitorStateStopProgress, instance.MonitorStateStopSuccess, instance.MonitorStateStopFailure)
}

func (t *Manager) skipPlacedStop() {
	t.loggerWithState().Infof("instance is already down")
	t.change = true
	t.state.State = instance.MonitorStateStopSuccess
	t.clearPending()
}

func (t *Manager) skipPlacedStart() {
	t.loggerWithState().Infof("instance is already up")
	t.change = true
	t.state.State = instance.MonitorStateStartSuccess
	t.clearPending()
}

func (t *Manager) clearStopFailedIfDown() {
	instStatus := t.instStatus[t.localhost]
	switch instStatus.Avail {
	case status.Down, status.StandbyDown:
		t.loggerWithState().Infof("instance is down, clear stop failed")
		t.change = true
		t.state.State = instance.MonitorStateStopSuccess
		t.clearPending()
	}
}

func (t *Manager) clearStopped() {
	t.doneAndIdle()
	t.clearPending()
}

func (t *Manager) orchestrateFailoverPlacedStartFromUnfreezeSuccess() {
	instStatus := t.instStatus[t.localhost]
	switch instStatus.Avail {
	case status.Up:
		t.transitionTo(instance.MonitorStateStartSuccess)
	default:
		t.transitionTo(instance.MonitorStateStopSuccess)
	}
}

func (t *Manager) orchestrateFailoverPlacedStartFromStopped() {
	switch t.objStatus.Avail {
	case status.NotApplicable, status.Undef:
		t.startedClearIfReached()
	case status.Down:
		t.placedStart()
	default:
		return
	}
}

func (t *Manager) orchestrateFailoverPlacedStartFromStarted() {
	t.startedClearIfReached()
}

func (t *Manager) orchestrateFlexPlacedStartFromUnfrozen() {
	t.placedStart()
}

func (t *Manager) orchestrateFlexPlacedStartFromStarted() {
	t.startedClearIfReached()
}

func (t *Manager) orchestratePlacedFromStartFailed() {
	switch {
	/*
		case o.AllInstanceMonitorState(instance.MonitorStateStartFailure):
			o.loggerWithState().Info().Msgf("daemon: imon: %s: all instances are start failed -> set done", o.path)
			o.done()
			o.clearPending()
	*/
	case t.objStatus.Avail == status.Up:
		t.startedClearIfReached()
	default:
		t.loggerWithState().Infof("local instance is start failed -> set done")
		t.done()
		t.clearPending()
	}
}
