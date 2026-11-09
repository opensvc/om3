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
	case instance.MonitorStateBootFailed,
		instance.MonitorStateFreezeFailure,
		instance.MonitorStateProvisionFailure,
		instance.MonitorStateStartFailure,
		instance.MonitorStateUnfreezeFailure:
		// A failure the purge has nothing to keep of, as the delete:
		// a start failed, a standby restart of a booting node among
		// them, leaves resources down the unprovision tears down
		// anyway. A stop failed still ends the purge, as in v2: the
		// unprovision would stop what would not stop.
		t.transitionTo(instance.MonitorStateIdle)
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
	if t.setWaitChildren() || !t.isLocalStatusCurrent() || t.waitPeersStarting() {
		return
	}
	// The instance is unprovisioned whatever its provisioned state says,
	// as in v2: the state aggregates the resources having one, and a
	// resource with none may hold what only its unprovision releases, as
	// the container of an oci resource, while the others read false. A
	// resource recorded not provisioned is stopped, not unprovisioned.
	t.purgedFromIdleProvisioned()
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
		// A non-leader whose unprovision failed stays provisioned, and
		// a failed action is not retried: the leader would wait for it
		// forever, and the purge would never end. The leader gives up,
		// as the unprovision orchestration does, and keeps its instance
		// and its configuration: its unprovision releases what the
		// non-leaders share, which one of them still holds.
		if node, ok := t.nonLeaderProvisionedWithState(instance.MonitorStatePurgeFailed); ok {
			t.log.Warnf("the purge failed on %s, still provisioned there: the leader does not unprovision", node)
			t.transitionTo(instance.MonitorStatePurgeFailed)
		}
		return
	}
	t.queueAction(t.crmUnprovisionLeader, instance.MonitorStateUnprovisionProgress, instance.MonitorStateUnprovisionSuccess, instance.MonitorStatePurgeFailed)
}

// nonLeaderProvisionedWithState returns a non-leader both provisioned, which
// the leader waits for, and in one of the states, as one whose purge failed.
// A non-leader whose purge failed once it was not provisioned any more, as
// on its delete, does not hold the leader, nor another non-leader still
// unprovisioning.
func (t *Manager) nonLeaderProvisionedWithState(states ...instance.MonitorState) (string, bool) {
	for node, instStatus := range t.instStatus {
		if t.isUnprovisionLeaderNode(node) {
			continue
		}
		if !instStatus.Provisioned.IsOneOf(provisioned.True, provisioned.Mixed) {
			continue
		}
		if t.instMonitor[node].State.IsOneOf(states...) {
			return node, true
		}
	}
	return "", false
}
