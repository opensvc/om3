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
		t.state.OrchestrationID = t.nodeMonitor[mostRecentNode].OrchestrationID
		strVal := t.nodeMonitor[mostRecentNode].GlobalExpect.String()
		if strVal == "" {
			strVal = "unset"
		}
		t.log.Infof("fetch global expect from node %s -> %s orchestration_id %s updated at %s",
			mostRecentNode, strVal, t.state.OrchestrationID, mostRecentUpdated)
		t.logSetOrchestrationID(t.state.OrchestrationID)

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
