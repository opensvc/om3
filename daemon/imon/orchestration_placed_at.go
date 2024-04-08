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
	case instance.MonitorStateThawed:
		t.orchestrateFailoverPlacedStartFromThawed()
	case instance.MonitorStateStarted:
		t.orchestrateFailoverPlacedStartFromStarted()
	case instance.MonitorStateStopped:
		t.orchestrateFailoverPlacedStartFromStopped()
	case instance.MonitorStateStartFailed:
		t.orchestratePlacedFromStartFailed()
	case instance.MonitorStateThawing:
	case instance.MonitorStateFreezing:
	case instance.MonitorStateStopping:
	case instance.MonitorStateStarting:
	default:
		t.log.Errorf("don't know how to orchestrate placed start from %s", t.state.State)
	}
}

func (t *Manager) orchestrateFlexPlacedStart() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.placedUnfreeze()
	case instance.MonitorStateThawed:
		t.orchestrateFlexPlacedStartFromThawed()
	case instance.MonitorStateStarted:
		t.orchestrateFlexPlacedStartFromStarted()
	case instance.MonitorStateStopped:
		t.transitionTo(instance.MonitorStateIdle)
	case instance.MonitorStateStartFailed:
		t.orchestratePlacedFromStartFailed()
	case instance.MonitorStateThawing:
	case instance.MonitorStateFreezing:
	case instance.MonitorStateStopping:
	case instance.MonitorStateStarting:
	default:
		t.log.Errorf("don't know how to orchestrate placed start from %s", t.state.State)
	}
}

func (t *Manager) orchestrateFailoverPlacedStop() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.placedUnfreeze()
	case instance.MonitorStateThawed:
		t.placedStop()
	case instance.MonitorStateStopFailed:
		t.clearStopFailedIfDown()
	case instance.MonitorStateStopped:
		t.clearStopped()
	case instance.MonitorStateReady:
		t.transitionTo(instance.MonitorStateIdle)
	case instance.MonitorStateStartFailed:
		t.orchestratePlacedFromStartFailed()
	case instance.MonitorStateThawing:
	case instance.MonitorStateFreezing:
	case instance.MonitorStateStopping:
	case instance.MonitorStateStarting:
	default:
		t.log.Errorf("don't know how to orchestrate placed stop from %s", t.state.State)
	}
}

func (t *Manager) orchestrateFlexPlacedStop() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.placedUnfreeze()
	case instance.MonitorStateThawed:
		t.placedStop()
	case instance.MonitorStateStopFailed:
		t.clearStopFailedIfDown()
	case instance.MonitorStateStopped:
		t.clearStopped()
	case instance.MonitorStateReady:
		t.transitionTo(instance.MonitorStateIdle)
	case instance.MonitorStateStartFailed:
		t.orchestratePlacedFromStartFailed()
	case instance.MonitorStateThawing:
	case instance.MonitorStateFreezing:
	case instance.MonitorStateStopping:
	case instance.MonitorStateStarting:
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
	if t.instStatus[t.localhost].IsThawed() {
		t.transitionTo(instance.MonitorStateThawed)
	} else {
		t.doUnfreeze()
	}
}

func (t *Manager) doPlacedStart() {
	t.doAction(t.crmStart, instance.MonitorStateStarting, instance.MonitorStateStarted, instance.MonitorStateStartFailed)
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
	t.doAction(t.crmStop, instance.MonitorStateStopping, instance.MonitorStateStopped, instance.MonitorStateStopFailed)
}

func (t *Manager) skipPlacedStop() {
	t.loggerWithState().Infof("instance is already down")
	t.change = true
	t.state.State = instance.MonitorStateStopped
	t.clearPending()
}

func (t *Manager) skipPlacedStart() {
	t.loggerWithState().Infof("instance is already up")
	t.change = true
	t.state.State = instance.MonitorStateStarted
	t.clearPending()
}

func (t *Manager) clearStopFailedIfDown() {
	instStatus := t.instStatus[t.localhost]
	switch instStatus.Avail {
	case status.Down, status.StandbyDown:
		t.loggerWithState().Infof("instance is down, clear stop failed")
		t.change = true
		t.state.State = instance.MonitorStateStopped
		t.clearPending()
	}
}

func (t *Manager) clearStopped() {
	t.doneAndIdle()
	t.state.LocalExpect = instance.MonitorLocalExpectNone
	t.clearPending()
}

func (t *Manager) orchestrateFailoverPlacedStartFromThawed() {
	instStatus := t.instStatus[t.localhost]
	switch instStatus.Avail {
	case status.Up:
		t.transitionTo(instance.MonitorStateStarted)
	default:
		t.transitionTo(instance.MonitorStateStopped)
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

func (t *Manager) orchestrateFlexPlacedStartFromThawed() {
	t.placedStart()
}

func (t *Manager) orchestrateFlexPlacedStartFromStarted() {
	t.startedClearIfReached()
}

func (t *Manager) orchestratePlacedFromStartFailed() {
	switch {
	/*
		case o.AllInstanceMonitorState(instance.MonitorStateStartFailed):
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
