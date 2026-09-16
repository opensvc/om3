package imon

import (
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
)

// orchestrateResized grows every instance of the object to the size its
// configuration asks for.
//
// It runs in two phases, because a replicated resource offers only what its
// smallest replica holds: every node first grows the links under the
// replicated one, and only then does the node holding the object up grow the
// replicated link and what rests on it.
//
// A failed resize is final on the instance it failed on. Nothing is retried,
// so the order between the nodes is waited for rather than hoped for.
func (t *Manager) orchestrateResized() {
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.resizedFromIdle()
	case instance.MonitorStateWaitNonLeader:
		t.resizedFromWaitNonLeader()
	case instance.MonitorStateResizeSuccess:
		t.resizedEnd("the instance holds the size asked for", true)
	case instance.MonitorStateResizeFailure:
		t.resizedEnd("the instance resize failed", false)
	}
}

func (t *Manager) resizedFromIdle() {
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.False) {
		// There is nothing here to grow. Saying so lets the node holding the
		// object up stop waiting for this one.
		t.log.Infof("resize: the instance is not provisioned, nothing to grow here")
		t.transitionTo(instance.MonitorStateResizeSuccess)
		return
	}
	if !t.hasAnyInstanceUp() {
		// The head resource grows on the node holding the object up, and no
		// node holds it up. Every node would grow what is under the
		// replicated link and stop there, which for an object having no
		// replicated link is nothing at all, and the orchestration would
		// report a size the object does not hold.
		t.log.Infof("resize: no instance is up, so nothing can grow the head resource")
		t.transitionTo(instance.MonitorStateResizeFailure)
		return
	}
	if t.isResizeLeader() {
		// The leader grows what is under the replicated link like every other
		// node, then waits for them before growing the rest.
		t.queueAction(t.crmResizeBelowReplicated,
			instance.MonitorStateResizeProgress,
			instance.MonitorStateWaitNonLeader,
			instance.MonitorStateResizeFailure)
		return
	}
	t.queueAction(t.crmResizeBelowReplicated,
		instance.MonitorStateResizeProgress,
		instance.MonitorStateResizeSuccess,
		instance.MonitorStateResizeFailure)
}

func (t *Manager) resizedFromWaitNonLeader() {
	if t.hasAnyPeerResizeFailed() {
		// Growing the replicated link now would ask it for more than the
		// smallest replica holds, and be refused. Stop here instead, so what
		// failed is what is reported.
		t.log.Infof("resize: a peer instance resize failed, the replicated resource cannot grow")
		t.transitionTo(instance.MonitorStateResizeFailure)
		return
	}
	if !t.hasAllPeersResized() {
		return
	}
	t.queueAction(t.crmResize,
		instance.MonitorStateResizeProgress,
		instance.MonitorStateResizeSuccess,
		instance.MonitorStateResizeFailure)
}

// resizedEnd ends the orchestration on this instance, leaving the state to
// linger so an operator sees what happened and so the node holding the object
// up can tell this one is ready.
func (t *Manager) resizedEnd(msg string, succeed bool) {
	if t.state.OrchestrationIsDone {
		return
	}
	if succeed {
		t.log.Infof("resized orchestration reached: %s", msg)
	} else {
		t.log.Infof("resized orchestration done: %s", msg)
	}
	t.done()
	t.updateIfChange()
}

// isResizeLeader says the local instance is the one to grow the replicated
// link and what rests on it.
//
// That is the instance holding the object up, because what rests on the
// replicated link is a filesystem, and a filesystem only grows where it is
// mounted.
func (t *Manager) isResizeLeader() bool {
	return t.instStatus[t.localhost].Avail.Is(status.Up)
}

// hasAnyInstanceUp says whether a node holds the object up, which is the node
// the second phase runs on. Without one the first phase is all that would run,
// and the head resource, which is what the size was asked of, stays as it is.
func (t *Manager) hasAnyInstanceUp() bool {
	for _, instStatus := range t.instStatus {
		if instStatus.Avail.Is(status.Up) {
			return true
		}
	}
	return false
}

func (t *Manager) hasAllPeersResized() bool {
	for _, instMon := range t.instMonitor {
		if !instMon.State.IsOneOf(instance.MonitorStateResizeSuccess) {
			return false
		}
	}
	return true
}

func (t *Manager) hasAnyPeerResizeFailed() bool {
	for _, instMon := range t.instMonitor {
		if instMon.State.IsOneOf(instance.MonitorStateResizeFailure) {
			return true
		}
	}
	return false
}
