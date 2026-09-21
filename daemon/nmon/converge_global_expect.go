package nmon

import (
	"time"

	"github.com/opensvc/om3/v3/core/node"
)

// convergeGlobalExpectFromRemote set global expect from most recent global expect value
func (t *Manager) convergeGlobalExpectFromRemote() {
	var mostRecentNode string
	var mostRecentUpdated time.Time
	for nodename, data := range t.nodeMonitor {
		if data.GlobalExpect == node.MonitorGlobalExpectInit {
			continue
		}
		if data.GlobalExpect == node.MonitorGlobalExpectNone {
			continue
		}
		nodeTime := data.GlobalExpectUpdatedAt
		if mostRecentUpdated.Before(nodeTime) {
			mostRecentNode = nodename
			mostRecentUpdated = nodeTime
		}
	}
	if mostRecentUpdated.IsZero() {
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
