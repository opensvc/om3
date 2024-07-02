package imon

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/status"
)

// orchestrateLocalExpectShutdown handles orchestration for local expect
// shutdown with following possible state transitions:
//
//	  from state 'idle':
//		   -> 'shutdown' if already shutdown (done)
//	    or
//		   -> 'wait children' if children are not yet shutdown
//	    or
//		   -> 'shutting' (crmShutdown is running) if not already shutdown and
//		      children are shutdown
//	  from state 'wait children':
//		   -> 'idle' if all children are shutdown
//	    or
//		   => 'wait children' if children are not yet shutdown
//	  from state 'shutting':
//		   -> 'shutdown' if crmShutdown succeed (done)
//	    or
//		   -> 'shutdown failed' if crmShutdown fails (done)
//	  from state 'shutdown':
//		   => 'shutdown' no change (done)
func (t *Manager) orchestrateLocalExpectShutdown() {
	t.log.Debugf("orchestrateLocalExpectShutdown from state %s", t.state.State)
	switch t.state.State {
	case instance.MonitorStateShutdown:
		// already in expected state, no more actions
	case instance.MonitorStateIdle:
		t.doShutdown()
	case instance.MonitorStateWaitChildren:
		t.setWaitChildren()
	case instance.MonitorStateShutting:
		// already shutting in progress
		t.log.Warnf("unexpected orchestrate local expect shutdown from state %s", t.state.State)
	case instance.MonitorStateShutdownFailed:
		// wait for clear or abort
	default:
		t.log.Errorf("don't know how to shutdown from %s", t.state.State)
	}
}

func (t *Manager) doShutdown() {
	if t.isLocalShutdown() {
		t.log.Infof("shutdown reached")
		t.transitionTo(instance.MonitorStateShutdown)
		return
	}
	if t.setWaitChildren() {
		return
	}
	t.queueAction(t.crmShutdown, instance.MonitorStateShutting, instance.MonitorStateShutdown, instance.MonitorStateShutdownFailed)
}

func (t *Manager) isLocalShutdown() bool {
	return t.instStatus[t.localhost].Avail.Is(status.Down, status.StandbyDown)
}
