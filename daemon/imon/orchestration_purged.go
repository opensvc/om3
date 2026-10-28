package imon

import (
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/provisioned"
)

var (
	kindWithImmediatePurge = naming.NewKinds(naming.KindSvc, naming.KindVol)
)

func (t *Manager) orchestratePurged() {
	t.log.Tracef("orchestratePurged starting from %s", t.state.State)
	switch t.state.State {
	case instance.MonitorStateDeleteSuccess:
		t.purgedFromDeleted()
	case instance.MonitorStateIdle:
		t.purgedFromIdle()
	case instance.MonitorStateStopFailure:
		t.done()
	case instance.MonitorStateUnprovisionSuccess:
		t.purgedFromUnprovisioned()
	case instance.MonitorStateWaitNonLeader:
		t.purgedFromWaitNonLeader()
	case instance.MonitorStatePurgeFailed:
		t.done()
	case instance.MonitorStateUnprovisionProgress,
		instance.MonitorStateDeleteProgress,
		instance.MonitorStateRunning:
	case instance.MonitorStateWaitChildren:
		t.setWaitChildren()
	default:
		t.log.Warnf("orchestratePurged has no solution from state %s", t.state.State)
	}
}

func (t *Manager) purgedFromIdle() {
	if !kindWithImmediatePurge.Has(t.path.Kind) {
		t.transitionTo(instance.MonitorStateUnprovisionSuccess)
		return
	}
	if t.setWaitChildren() || !t.isLocalStatusCurrent() {
		return
	}
	// A mixed instance, as one whose provision failed half way, holds
	// what its provisioned resources took: deleting it unprovisioned
	// would leave that behind, a drbd resource up on a volume among them.
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.True, provisioned.Mixed, provisioned.NotApplicable) {
		t.purgedFromIdleProvisioned()
		return
	}
	go t.orchestrateAfterAction(instance.MonitorStateIdle, instance.MonitorStateUnprovisionSuccess)
	return
}

// purgedFromDeleted ends the purge orchestration the way every other
// orchestration ends it: by marking it done and letting endOrchestration()
// clear the global expect and the orchestration id together. The end is
// announced when the instance monitor is deleted, as the end of a delete is:
// the instance is gone then, from this daemon too, and a client told the
// purge is over finds no trace of the object, as a create of the same name
// that the daemon refused while it still knew the instance.
//
// Resetting the global expect here instead left the instance monitor with no
// global expect but still holding the orchestration id, a pair that means
// "an orchestration is running" everywhere it is read: the next submit was
// refused as "already in progress", and the orchestration table never saw an
// end for an orchestration it went on listing as running.
func (t *Manager) purgedFromDeleted() {
	t.endWithDeletion = true
	t.doneAndIdle()
}

func (t *Manager) purgedFromUnprovisioned() {
	t.queueAction(t.crmDelete, instance.MonitorStateDeleteProgress, instance.MonitorStateDeleteSuccess, instance.MonitorStatePurgeFailed)
}

func (t *Manager) purgedFromIdleProvisioned() {
	if t.isUnprovisionLeader() {
		t.transitionTo(instance.MonitorStateWaitNonLeader)
		t.purgedFromWaitNonLeader()
		return
	}
	t.queueAction(t.crmUnprovisionNonLeader, instance.MonitorStateUnprovisionProgress, instance.MonitorStateUnprovisionSuccess, instance.MonitorStatePurgeFailed)
}

func (t *Manager) purgedFromWaitNonLeader() {
	if !t.isUnprovisionLeader() {
		t.transitionTo(instance.MonitorStateIdle)
		return
	}
	if t.hasNonLeaderProvisioned() {
		return
	}
	t.queueAction(t.crmUnprovisionLeader, instance.MonitorStateUnprovisionProgress, instance.MonitorStateUnprovisionSuccess, instance.MonitorStatePurgeFailed)
}
