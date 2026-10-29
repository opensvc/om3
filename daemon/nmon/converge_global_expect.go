package nmon

import (
	"time"

	"github.com/opensvc/om3/v3/core/node"
)

// convergeGlobalExpectFromRemote set global expect from most recent global expect value
func (t *Manager) convergeGlobalExpectFromRemote() {
	var mostRecentNode, mostRecentEndedNode string
	var mostRecentUpdated, mostRecentEndedUpdated time.Time
	for nodename, data := range t.nodeMonitor {
		if data.GlobalExpect == node.MonitorGlobalExpectInit {
			continue
		}
		if data.GlobalExpect == node.MonitorGlobalExpectNone {
			if mostRecentEndedUpdated.Before(data.GlobalExpectUpdatedAt) {
				mostRecentEndedNode = nodename
				mostRecentEndedUpdated = data.GlobalExpectUpdatedAt
			}
			continue
		}
		// A global expect of a later version is one this agent can not
		// orchestrate: adopting it, it would never end its part of it.
		if data.GlobalExpect == node.MonitorGlobalExpectUnknown {
			continue
		}
		nodeTime := data.GlobalExpectUpdatedAt
		if mostRecentUpdated.Before(nodeTime) {
			mostRecentNode = nodename
			mostRecentUpdated = nodeTime
		}
	}
	if mostRecentUpdated.IsZero() {
		t.alignEndedGlobalExpect(mostRecentEndedNode, mostRecentEndedUpdated)
		return
	}
	if mostRecentUpdated.After(t.state.GlobalExpectUpdatedAt) {
		t.change = true
		t.state.GlobalExpect = t.nodeMonitor[mostRecentNode].GlobalExpect
		t.state.GlobalExpectUpdatedAt = t.nodeMonitor[mostRecentNode].GlobalExpectUpdatedAt
		// The orchestration comes with the global expect it belongs to. Only
		// the node the requester reached adopts the id from the request; every
		// other node takes it from here, and without it the execs they fork
		// would be steps of an orchestration they could not name.
		//
		// It is adopted rather than assigned, so that the orchestration this
		// one displaces is ended as aborted and the adopted one gets a pending
		// end of its own. Assigning the id left the displaced one never
		// ending, and let the end of this one be published under the id it
		// replaced.
		t.adoptOrchestration(t.nodeMonitor[mostRecentNode].OrchestrationID)
		strVal := t.nodeMonitor[mostRecentNode].GlobalExpect.String()
		if strVal == "" {
			strVal = "unset"
		}
		t.log.Infof("fetch global expect from node %s -> %s orchestration_id %s updated at %s",
			mostRecentNode, strVal, t.state.OrchestrationID, mostRecentUpdated)

		if t.isStateFailed() {
			t.log.Tracef("reset failed state")
			t.state.State = node.MonitorStateIdle
		}
	}
}

// alignEndedGlobalExpect adopts the time of the global expect a peer ended,
// updated, when no global expect is pending here: an orchestration this
// daemon did not see, as one ended while it was down. A daemon restarted
// reports the zero time, and the peers, waiting for it to have the time of
// the global expect they ended, as a cluster thaw, never handled their local
// expect again, as a drain. The time alone is adopted, there being no global
// expect to orchestrate.
func (t *Manager) alignEndedGlobalExpect(nodename string, updated time.Time) {
	switch t.state.GlobalExpect {
	case node.MonitorGlobalExpectNone, node.MonitorGlobalExpectInit:
	default:
		return
	}
	if !updated.After(t.state.GlobalExpectUpdatedAt) {
		return
	}
	t.log.Infof("align the global expect time on node %s: %s, of a global expect ended", nodename, updated)
	t.change = true
	t.state.GlobalExpectUpdatedAt = updated
}

func (t *Manager) isConvergedGlobalExpect() bool {
	localUpdated := t.state.GlobalExpectUpdatedAt
	for s, data := range t.nodeMonitor {
		if s == t.localhost {
			continue
		}
		if localUpdated.After(data.GlobalExpectUpdatedAt) {
			t.log.Tracef("wait global expect propagation on %s", s)
			return false
		}
	}
	return true
}
