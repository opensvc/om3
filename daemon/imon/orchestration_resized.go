package imon

import (
	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/core/provisioned"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/xerrors"
	"github.com/opensvc/om3/v3/daemon/runner"
	"github.com/opensvc/om3/v3/util/file"
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
	if stage, ok := t.state.State.ResizeStage(); ok {
		t.resizedFromStage(stage)
		return
	}
	switch t.state.State {
	case instance.MonitorStateIdle:
		t.resizedFromIdle()
	case instance.MonitorStateResizeSuccess:
		t.resizedEnd("the instance holds the size asked for", true)
	case instance.MonitorStateResizeFailure:
		t.resizedEnd("the instance resize failed", false)
	}
}

func (t *Manager) resizedFromIdle() {
	if !t.hasResizeConfig() {
		// Waiting, not failing: the configuration is on its way, and the
		// orchestration is re-evaluated as the node monitor changes.
		return
	}
	if t.instStatus[t.localhost].Provisioned.IsOneOf(provisioned.False) {
		// There is nothing here to grow. Saying so lets the nodes still
		// growing stop waiting for this one.
		t.log.Infof("resize: the instance is not provisioned, nothing to grow here")
		t.transitionTo(instance.MonitorStateResizeSuccess)
		return
	}
	if !t.hasAnyInstanceUp() {
		// The head grows on the node holding the object up, and no node holds
		// it up. The stages below it would run and the head would stay as it
		// is, and the orchestration would report a size the object does not
		// hold.
		t.log.Infof("resize: no instance is up, so nothing can grow the head resource")
		t.transitionTo(instance.MonitorStateResizeFailure)
		return
	}
	t.queueResizeStage(0)
}

// resizedFromStage moves on from a stage this node has finished.
//
// The next stage is asked for only once every node has finished this one: a
// replicated resource offers what its smallest replica holds, so growing it
// before its peers have grown what is under it would be refused.
func (t *Manager) resizedFromStage(stage int) {
	if t.hasAnyPeerResizeFailed() {
		// Growing further would ask a replicated link for more than its
		// smallest replica holds, and be refused. Stop here instead, so what
		// failed is what is reported.
		t.log.Infof("resize: a peer instance resize failed, the chain cannot grow further")
		t.transitionTo(instance.MonitorStateResizeFailure)
		return
	}
	if !t.hasAllPeersReachedStage(stage) {
		return
	}
	t.queueResizeStage(stage + 1)
}

// queueResizeStage grows one stage, and reads from what it answers whether
// another follows.
//
// The stages are read from the chain, so only the node walking it knows how
// many there are. A node asking for one past the last is told there is no
// stage of that number to run here, which is how it learns it is done.
func (t *Manager) queueResizeStage(stage int) {
	staged, ok := instance.NewMonitorStateResizeStage(stage)
	if !ok {
		// The chain is grown in more stages than there are names for the
		// state of a node between them. The resize refuses it too, saying so,
		// but this one is reached first when a stage fails to end.
		t.log.Infof("resize: stage %d is past the %d an orchestration grows a chain in", stage, instance.MaxResizeStages)
		t.transitionTo(instance.MonitorStateResizeFailure)
		return
	}
	_ = runner.Run(t.instConfig.Priority, func() error {
		t.transitionTo(instance.MonitorStateResizeProgress)
		next := staged
		exitCode, err := t.crmResizeStage(stage)
		switch {
		case err != nil:
			next = instance.MonitorStateResizeFailure
		case exitCode == xerrors.ExitCodeResizeNoSuchStage:
			// There is no stage of that number to run here, which is the
			// answer that ends the walk.
			next = instance.MonitorStateResizeSuccess
		}
		go t.orchestrateAfterAction(instance.MonitorStateResizeProgress, next)
		return nil
	})
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

// hasResizeConfig says whether this node holds the configuration the resize
// was asked for.
//
// The size to grow to is read from the local configuration, and a
// configuration write is acknowledged by the node that received it a moment
// before it reaches the others. Without this, a node told to resize before the
// write lands grows to the size it was asked to replace, and says it is done.
//
// The file is read rather than the configuration the daemon caches, because
// the file is what the resize is about to read, and the cache trails it by the
// time it takes to notice the change.
func (t *Manager) hasResizeConfig() bool {
	options, ok := t.state.GlobalExpectOptions.(instance.MonitorGlobalExpectOptionsResized)
	if !ok || options.ConfigUpdatedAt.IsZero() {
		return true
	}
	mtime := file.ModTime(t.path.ConfigFile())
	if mtime.IsZero() || mtime.Before(options.ConfigUpdatedAt) {
		t.log.Infof("resize: wait for the configuration of %s to land here", options.ConfigUpdatedAt)
		return false
	}
	return true
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

// hasAllPeersReachedStage says whether every peer has finished the stage this
// node just finished, or has nothing left to do at all.
//
// A peer that is done is a peer that will not grow anything more, so it holds
// nobody up: a node not holding the object up finishes at the stage below the
// one holding the head, and a node with nothing provisioned finishes at once.
func (t *Manager) hasAllPeersReachedStage(stage int) bool {
	for nodename, instMon := range t.instMonitor {
		if peerStage, ok := instMon.State.ResizeStage(); ok {
			if peerStage >= stage {
				continue
			}
		} else if instMon.State.IsOneOf(instance.MonitorStateResizeSuccess) {
			continue
		}
		t.log.Tracef("resize: wait for %s to finish stage %d", nodename, stage)
		return false
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
