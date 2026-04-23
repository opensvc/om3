package imon

import (
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/util/file"
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
		t.logSetOrchestrationID(t.state.OrchestrationID)
		if t.state.GlobalExpect == instance.MonitorGlobalExpectAborted {
			t.savePendingOrchestration()
		}
	}
}

func (t *Manager) isConvergedGlobalExpect() bool {
	localUpdated := t.state.GlobalExpectUpdatedAt

	// defines the expected instance monitors from scope nodes:
	// If local instance has been created recently, it will take time to:
	// 1- propagate the instance config file on peers
	// 2- peer imon startup, push its state
	// 3- peer imon state is propagated to local imon.
	expectedInstanceMonitor := make(map[string]struct{})
	for _, n := range t.scopeNodes {
		if n == t.localhost {
			continue
		}
		expectedInstanceMonitor[n] = struct{}{}
	}

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
		// we have a converged GlobalExpect (we don't expect any more propagation from this peer node)
		delete(expectedInstanceMonitor, s)
	}
	if len(expectedInstanceMonitor) > 0 {
		// expected peer instance monitors are still missing
		switch t.state.GlobalExpect {
		case instance.MonitorGlobalExpectPurged, instance.MonitorGlobalExpectDeleted:
			// purge or deletion orchestration, we can ignore missing peer monitor states,
			// peer instances may have been already deleted.
		default:
			for n := range expectedInstanceMonitor {
				if _, ok := t.nodeMonitor[n]; ok {
					t.log.Tracef("wait for initial GlobalExpect propagation on %s", n)
					return false
				}
			}
		}
	}
	if t.state.GlobalExpect != instance.MonitorGlobalExpectNone && t.state.GlobalExpect != instance.MonitorGlobalExpectInit {
		if t.state.GlobalExpectUpdatedAt.Sub(t.instConfig.UpdatedAt) < time.Second {
			// The instance config has been modified within the same second of the global expectation update.
			// Take time to verify if we have the last instance config:
			//     t.instConfig.UpdatedAt versus config file mod time
			cfgFileModtime := file.ModTime(t.path.ConfigFile())
			if cfgFileModtime.IsZero() {
				return false
			}
			if cfgFileModtime.After(t.instConfig.UpdatedAt) {
				t.log.Tracef("wait GlobalExpect propagation delayed: config file has been updated")
				return false
			}
		}
	}
	return true
}
