package imon

import (
	"fmt"
	"time"

	"github.com/opensvc/om3/v3/core/instance"
	"github.com/opensvc/om3/v3/daemon/expectstamp"
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
		// A global expect of a later version is one this agent can not
		// orchestrate. Adopting it, it would never end its part of the
		// orchestration, and the peers running it would wait for it for
		// good. Left alone, this instance reads as done to them, so the
		// orchestration runs on the nodes that know it.
		if instMon.GlobalExpect == instance.MonitorGlobalExpectUnknown {
			continue
		}
		nodeTime := instMon.GlobalExpectUpdatedAt
		if mostRecentNode == "" || expectstamp.NewerCandidate(nodeTime, instMon.OrchestrationID, mostRecentUpdated, t.instMonitor[mostRecentNode].OrchestrationID) {
			mostRecentNode = node
			mostRecentUpdated = nodeTime
		}
	}
	if mostRecentUpdated.IsZero() {
		return
	}
	// Of a peer stamping its global expect the same as this node, as both
	// accepting a request at once after the same stamp, the one of the
	// greater orchestration id is kept on both.
	if expectstamp.Newer(mostRecentUpdated, t.instMonitor[mostRecentNode].OrchestrationID, t.state.GlobalExpectUpdatedAt, t.state.OrchestrationID) {
		t.change = true
		t.state.GlobalExpect = t.instMonitor[mostRecentNode].GlobalExpect
		t.state.GlobalExpectUpdatedAt = t.instMonitor[mostRecentNode].GlobalExpectUpdatedAt
		t.globalExpectSeenAt = time.Now()
		t.state.GlobalExpectOptions = t.instMonitor[mostRecentNode].GlobalExpectOptions
		if adopted := t.instMonitor[mostRecentNode].OrchestrationID; t.orchestrationPending != nil && t.orchestrationPending.ID != adopted.String() {
			// The orchestration accepted here is displaced by the one
			// of a peer, as of a request accepted there at once and
			// stamped the same: it ends aborted, rather than end as
			// the one adopted does, under its id.
			t.log.Infof("orchestration %s accepted here displaced by %s of node %s", t.orchestrationPending.ID, adopted, mostRecentNode)
			t.orchestrationPending.Aborted = true
			t.publishOrchestrationEnded()
		}
		t.state.OrchestrationID = t.instMonitor[mostRecentNode].OrchestrationID
		t.state.State = instance.MonitorStateIdle
		strVal := t.instMonitor[mostRecentNode].GlobalExpect.String()
		if strVal == "" {
			strVal = "unset"
		}
		t.log.Infof("fetch global expect from node %s -> %s orchestration_id %s updated at %s",
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
		if expectstamp.Newer(localUpdated, t.state.OrchestrationID, v.GlobalExpectUpdatedAt, v.OrchestrationID) {
			t.log.Tracef("wait GlobalExpect propagation on %s", s)
			return false
		}
		// we have a converged GlobalExpect (we don't expect any more propagation from this peer node)
		delete(expectedInstanceMonitor, s)
	}
	if len(expectedInstanceMonitor) > 0 {
		// expected peer instance monitors are still missing
		switch t.state.GlobalExpect {
		case instance.MonitorGlobalExpectPurged, instance.MonitorGlobalExpectDeleted, instance.MonitorGlobalExpectAborted:
			// purge or deletion orchestration, we can ignore missing peer monitor states,
			// peer instances may have been already deleted.
			// The abort of such an orchestration meets the same deleted
			// peers, which have nothing left to abort: waiting for their
			// monitors would never end the abort.
		default:
			for n := range expectedInstanceMonitor {
				// A node in maintenance or shutting down runs no
				// orchestration: waited for, it held a forced
				// unprovision until it came back. A node starting
				// is waited for: it adopts it as its monitor starts.
				if mon, ok := t.nodeMonitor[n]; ok && !mon.State.LeftOrchestrations() {
					t.log.Tracef("wait for initial GlobalExpect propagation on %s", n)
					return false
				}
			}
		}
	}
	if t.state.GlobalExpect != instance.MonitorGlobalExpectNone && t.state.GlobalExpect != instance.MonitorGlobalExpectInit {
		// The local time the global expect was seen at, the stamp being
		// of the clock of the node that accepted it, or after it.
		seenAt := t.globalExpectSeenAt
		if seenAt.IsZero() {
			seenAt = t.state.GlobalExpectUpdatedAt
		}
		if seenAt.Sub(t.instConfig.UpdatedAt) < time.Second {
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

// seenGlobalExpectStamps returns the stamps of the global expects of this
// monitor and of its peers, the stamp of a new one to be after them.
func (t *Manager) seenGlobalExpectStamps() []time.Time {
	l := []time.Time{t.state.GlobalExpectUpdatedAt}
	for _, instMon := range t.instMonitor {
		l = append(l, instMon.GlobalExpectUpdatedAt)
	}
	return l
}
