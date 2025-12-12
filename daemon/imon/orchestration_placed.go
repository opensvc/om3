package imon

import (
	"github.com/opensvc/om3/v3/core/topology"
)

func (t *Manager) orchestratePlaced() {
	if t.state.IsHALeader {
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
