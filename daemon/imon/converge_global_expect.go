package imon

import (
	"fmt"
	"time"

	"github.com/opensvc/om3/core/instance"
)

// convergeGlobalExpectFromRemote set global expect from most recent global expect value
func (t *Manager) convergeGlobalExpectFromRemote() {
	var mostRecentNode string
	var mostRecentUpdated time.Time
	for node, instMon := range t.instMonitor {
		if instMon.GlobalExpect == instance.MonitorGlobalExpectInit {
			continue
		}
		nodeTime := instMon.GlobalExpectUpdatedAt
		if mostRecentUpdated.Before(nodeTime) {
			mostRecentNode = node
			mostRecentUpdated = nodeTime
		}
	}
	if mostRecentUpdated.IsZero() {
		return
	}
	if mostRecentUpdated.After(t.state.GlobalExpectUpdatedAt) {
		t.change = true
		t.state.GlobalExpect = t.instMonitor[mostRecentNode].GlobalExpect
		t.state.GlobalExpectUpdatedAt = t.instMonitor[mostRecentNode].GlobalExpectUpdatedAt
		t.state.GlobalExpectOptions = t.instMonitor[mostRecentNode].GlobalExpectOptions
		t.state.OrchestrationID = t.instMonitor[mostRecentNode].OrchestrationID
		t.state.State = instance.MonitorStateIdle
		strVal := t.instMonitor[mostRecentNode].GlobalExpect.String()
		if strVal == "" {
			strVal = "unset"
		}
		t.log.Infof("fetch global expect from node %s -> %s orchestration id %s updated at %s",
			mostRecentNode, strVal, t.state.OrchestrationID, mostRecentUpdated)
		if t.state.OrchestrationIsDone {
			t.state.OrchestrationIsDone = false
			t.log.Tracef("reset previous orchestration is done on fetched global expect")
		}
		t.log = t.newLogger(t.state.OrchestrationID)
		if t.state.GlobalExpect == instance.MonitorGlobalExpectAborted {
			t.savePendingOrchestration()
		}
	}
}

func (t *Manager) isConvergedGlobalExpect() bool {
	localUpdated := t.state.GlobalExpectUpdatedAt
	for s, v := range t.instMonitor {
		if s == t.localhost {
			err := fmt.Errorf("bug: isConvergedGlobalExpect detect unexpected localhost in internal instance monitor cache keys")
			t.log.Errorf("isConvergedGlobalExpect: %s", err)
			panic(err)
		}
		if localUpdated.After(v.GlobalExpectUpdatedAt) {
			t.log.Tracef("wait GlobalExpect propagation on %s", s)
			return false
		}
	}
	return true
}
