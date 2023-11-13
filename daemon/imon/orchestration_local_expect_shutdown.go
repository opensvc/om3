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
func (o *imon) orchestrateLocalExpectShutdown() {
	o.log.Debugf("orchestrateLocalExpectShutdown from state %s", o.state.State)
	switch o.state.State {
	case instance.MonitorStateShutdown:
		// already in expected state, no more actions
	case instance.MonitorStateIdle:
		o.doShutdown()
	case instance.MonitorStateWaitChildren:
		o.setWaitChildren()
	case instance.MonitorStateShutting:
		// already shutting in progress
		o.log.Warnf("unexpected orchestrate local expect shutdown from state %s", o.state.State)
	case instance.MonitorStateShutdownFailed:
		// wait for clear or abort
	default:
		o.log.Errorf("don't know how to shutdown from %s", o.state.State)
	}
}

func (o *imon) doShutdown() {
	if o.isLocalShutdown() {
		o.log.Infof("shutdown reached")
		o.transitionTo(instance.MonitorStateShutdown)
		return
	}
	if o.setWaitChildren() {
		return
	}
	o.doAction(o.crmShutdown, instance.MonitorStateShutting, instance.MonitorStateShutdown, instance.MonitorStateShutdownFailed)
}

func (o *imon) isLocalShutdown() bool {
	return o.instStatus[o.localhost].Avail.Is(status.Down, status.StandbyDown)
}
