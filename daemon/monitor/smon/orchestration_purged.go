package smon

import (
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/status"
)

func (o *smon) orchestratePurged() {
	switch o.state.Status {
	case statusDeleted:
		o.purgedFromDeleted()
	case statusIdle:
		o.purgedFromIdle()
	case statusUnprovisioned:
		o.purgedFromUnProvisioned()
	case statusWaitNonLeader:
		o.purgedFromWaitNonLeader()
	}
}

func (o *smon) purgedFromIdle() {
	if o.instStatus[o.localhost].Avail == status.Up {
		o.purgedFromIdleUp()
		return
	}
	if o.instStatus[o.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.NotApplicable) {
		o.purgedFromIdleProvisioned()
		return
	}
	go func() {
		o.cmdC <- cmdOrchestrate{state: statusIdle, newState: statusUnprovisioned}
	}()
	return
}

func (o *smon) purgedFromDeleted() {
	o.change = true
	o.state.GlobalExpect = globalExpectUnset
	o.state.Status = statusIdle
	o.updateIfChange()
}

func (o *smon) purgedFromUnProvisioned() {
	o.doAction(o.crmDelete, statusDeleting, statusDeleted, statusPurgeFailed)
}

func (o *smon) purgedFromIdleUp() {
	o.doAction(o.crmStop, statusStopping, statusIdle, statusStopFailed)
}

func (o *smon) purgedFromIdleProvisioned() {
	if o.isUnprovisionLeader() {
		o.transitionTo(statusWaitNonLeader)
		o.purgedFromWaitNonLeader()
		return
	}
	o.doAction(o.crmUnprovisionNonLeader, statusUnprovisioning, statusUnprovisioned, statusPurgeFailed)
}

func (o *smon) purgedFromWaitNonLeader() {
	if !o.isUnprovisionLeader() {
		o.transitionTo(statusIdle)
		return
	}
	if o.hasNonLeaderProvisioned() {
		return
	}
	o.doAction(o.crmUnprovisionLeader, statusUnprovisioning, statusUnprovisioned, statusPurgeFailed)
}
