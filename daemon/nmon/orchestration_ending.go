package nmon

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/node"
)

// orchestrationEndingTimeout bounds the wait for the peers to be done with a
// cluster orchestration done here: they run an om command each, as a node
// freeze. Past it, the end is announced failed, naming them.
var orchestrationEndingTimeout = 30 * time.Second

// peerNotWaited lists the node monitor states of a peer an orchestration
// does not wait for, as v2 waited for the live nodes only: a node starting,
// rejoining, in maintenance or shutting down runs no orchestration, and one
// that missed a freeze catches it as it rejoins.
var peerNotWaited = []node.MonitorState{
	node.MonitorStateInit,
	node.MonitorStateRejoin,
	node.MonitorStateMaintenance,
	node.MonitorStateShutdownProgress,
	node.MonitorStateShutdownSuccess,
	node.MonitorStateShutdownFailure,
}

// holdsEndForPeers says whether the end of the orchestration waits for the
// peers: a freeze, a thaw or an abort of the cluster runs on every node. A
// drain is of this node alone, and an upgrade restarts the daemons, the
// command waiting for them on its own.
func holdsEndForPeers(globalExpect node.MonitorGlobalExpect) bool {
	switch globalExpect {
	case node.MonitorGlobalExpectFrozen, node.MonitorGlobalExpectUnfrozen, node.MonitorGlobalExpectAborted:
		return true
	default:
		return false
	}
}

// endOrchestrationWhenPeersDone announces the end of the orchestration done
// here, or holds it back until no live peer carries the orchestration any
// more, each peer dropping it once its own part is done: the end, which ends
// the --wait of the requester, said the cluster frozen with its peers still
// freezing.
func (t *Manager) endOrchestrationWhenPeersDone() {
	pending := t.orchestrationPending
	if pending == nil {
		return
	}
	if pending.Failed || !holdsEndForPeers(pending.GlobalExpect) {
		t.publishOrchestrationEnded()
		return
	}
	t.publishOrchestrationEnding()
	t.orchestrationEnding = pending
	t.orchestrationPending = nil
	t.orchestrationEndingSince = time.Now()
	t.onOrchestrationEndingCheck()
}

// onOrchestrationEndingCheck announces the end held back once no live peer
// carries the orchestration, failed when a peer failed its part or did not
// end it in time, and arms the timer of the next check.
func (t *Manager) onOrchestrationEndingCheck() {
	e := t.orchestrationEnding
	if e == nil {
		return
	}
	var holding, failed []string
	for nodename, mon := range t.nodeMonitor {
		if nodename == t.localhost || mon.OrchestrationID.String() != e.ID {
			continue
		}
		switch {
		case mon.State.IsOneOf(peerNotWaited...):
		case mon.State.IsOneOf(node.MonitorStatesFailure...):
			failed = append(failed, fmt.Sprintf("%s on %s", mon.State, nodename))
		default:
			holding = append(holding, nodename)
		}
	}
	slices.Sort(holding)
	slices.Sort(failed)
	switch {
	case len(failed) > 0:
		e.Failed = true
		e.Error = strings.Join(failed, ", ")
		t.publishOrchestrationEnding()
	case len(holding) == 0:
		t.publishOrchestrationEnding()
	case time.Since(t.orchestrationEndingSince) >= orchestrationEndingTimeout:
		t.log.Warnf("orchestration %s: %s not done within %s", e.ID, strings.Join(holding, ", "), orchestrationEndingTimeout)
		e.Failed = true
		e.Error = fmt.Sprintf("not done within %s on %s", orchestrationEndingTimeout, strings.Join(holding, ", "))
		t.publishOrchestrationEnding()
	default:
		if t.orchestrationClearTimer != nil {
			t.orchestrationClearTimer.Stop()
			t.orchestrationClearTimer.Reset(orchestrationEndingTimeout - time.Since(t.orchestrationEndingSince))
		}
	}
}

// publishOrchestrationEnding announces the end held back.
func (t *Manager) publishOrchestrationEnding() {
	if t.orchestrationEnding == nil {
		return
	}
	if t.orchestrationClearTimer != nil {
		t.orchestrationClearTimer.Stop()
	}
	t.publisher.Pub(t.orchestrationEnding, t.labelLocalhost)
	t.orchestrationEnding = nil
}

// abortOrchestrationEnding announces the end held back aborted: a new
// orchestration displaces it on the peers still running it.
func (t *Manager) abortOrchestrationEnding() {
	if t.orchestrationEnding == nil {
		return
	}
	t.orchestrationEnding.Aborted = true
	t.publishOrchestrationEnding()
}
