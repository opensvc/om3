package imon

import (
	"github.com/opensvc/om3/v3/core/topology"
)

// orchestratePlaced moves the object to where it belongs, as a giveback asks.
//
// It is a user request, so the instance to start is the one the start rules
// of a user request pick, as for a start: the stopped flags and the freezes,
// which keep the daemon from acting by itself, keep nothing from a request.
// The ha leader rule skipped them, and each node lowers its own stopped flag
// as the request reaches it: a peer lowering its flag first, or unstopped
// before, took itself for the leader and started in its place.
func (t *Manager) orchestratePlaced() {
	if t.isStartCandidateLeader(false) {
		t.orchestratePlacedStart()
	} else {
		t.orchestratePlacedStop()
	}
}

func (t *Manager) orchestratePlacedStart() {
	switch t.objStatus.Topology {
	case topology.Failover:
		t.orchestrateFailoverPlacedStart()
	case topology.Flex:
		t.orchestrateFlexPlacedStart()
	}
}

func (t *Manager) orchestratePlacedStop() {
	t.disableMonitor("orchestrate placed stop")
	switch t.objStatus.Topology {
	case topology.Failover:
		t.orchestrateFailoverPlacedStop()
	case topology.Flex:
		t.orchestrateFlexPlacedStop()
	}
}
