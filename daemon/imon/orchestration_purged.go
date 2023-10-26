package imon

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/status"
)

func (o *imon) orchestratePurged() {
	o.log.Debugf("orchestratePurged starting from %s", o.state.State)
	switch o.state.State {
	case instance.MonitorStateDeleted:
		o.purgedFromDeleted()
	case instance.MonitorStateIdle:
		o.purgedFromIdle()
	case instance.MonitorStateStopped:
		o.purgedFromStopped()
	case instance.MonitorStateStopFailed:
		o.done()
	case instance.MonitorStateUnprovisioned:
		o.purgedFromUnprovisioned()
	case instance.MonitorStateWaitNonLeader:
		o.purgedFromWaitNonLeader()
	case instance.MonitorStateUnprovisioning,
		instance.MonitorStateDeleting,
		instance.MonitorStateStopping:
	default:
		o.log.Warnf("orchestratePurged has no solution from state %s", o.state.State)
	}
}

func (o *imon) purgedFromIdle() {
	if o.instStatus[o.localhost].Avail == status.Up {
		o.purgedFromIdleUp()
		return
	}
	if o.instStatus[o.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		o.purgedFromIdleProvisioned()
		return
	}
	go o.orchestrateAfterAction(instance.MonitorStateIdle, instance.MonitorStateUnprovisioned)
	return
}

func (o *imon) purgedFromStopped() {
	if o.instStatus[o.localhost].Avail.Is(status.Up, status.Warn) {
		o.log.Debugf("purgedFromStopped return on o.instStatus[o.localhost].Avail.Is(status.Up, status.Warn)")
		return
	}
	if o.instStatus[o.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		o.log.Debugf("purgedFromStopped return on o.instStatus[o.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable)")
		o.purgedFromIdleProvisioned()
		return
	}
	go o.orchestrateAfterAction(instance.MonitorStateStopped, instance.MonitorStateUnprovisioned)
	return
}

func (o *imon) purgedFromDeleted() {
	o.change = true
	o.state.GlobalExpect = instance.MonitorGlobalExpectNone
	o.state.State = instance.MonitorStateIdle
	o.updateIfChange()
}

func (o *imon) purgedFromUnprovisioned() {
	o.doAction(o.crmDelete, instance.MonitorStateDeleting, instance.MonitorStateDeleted, instance.MonitorStatePurgeFailed)
}

func (o *imon) purgedFromIdleUp() {
	o.doAction(o.crmStop, instance.MonitorStateStopping, instance.MonitorStateStopped, instance.MonitorStateStopFailed)
}

func (o *imon) purgedFromIdleProvisioned() {
	if o.isUnprovisionLeader() {
		o.transitionTo(instance.MonitorStateWaitNonLeader)
		o.purgedFromWaitNonLeader()
		return
	}
	o.doAction(o.crmUnprovisionNonLeader, instance.MonitorStateUnprovisioning, instance.MonitorStateUnprovisioned, instance.MonitorStatePurgeFailed)
}

func (o *imon) purgedFromWaitNonLeader() {
	if !o.isUnprovisionLeader() {
		o.transitionTo(instance.MonitorStateIdle)
		return
	}
	if o.hasNonLeaderProvisioned() {
		return
	}
	o.doAction(o.crmUnprovisionLeader, instance.MonitorStateUnprovisioning, instance.MonitorStateUnprovisioned, instance.MonitorStatePurgeFailed)
}
