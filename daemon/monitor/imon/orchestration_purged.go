package imon

import (
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
)

func (o *imon) orchestratePurged() {
	switch o.state.State {
	case instance.MonitorStateDeleted:
		o.purgedFromDeleted()
	case instance.MonitorStateIdle:
		o.purgedFromIdle()
	case instance.MonitorStateUnprovisioned:
		o.purgedFromUnprovisioned()
	case instance.MonitorStateWaitNonLeader:
		o.purgedFromWaitNonLeader()
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

func (o *imon) purgedFromDeleted() {
	o.change = true
	o.state.GlobalExpect = instance.MonitorGlobalExpectUnset
	o.state.State = instance.MonitorStateIdle
	o.updateIfChange()
}

func (o *imon) purgedFromUnprovisioned() {
	o.doAction(o.crmDelete, instance.MonitorStateDeleting, instance.MonitorStateDeleted, instance.MonitorStatePurgeFailed)
}

func (o *imon) purgedFromIdleUp() {
	o.doAction(o.crmStop, instance.MonitorStateStopping, instance.MonitorStateIdle, instance.MonitorStateStopFailed)
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
