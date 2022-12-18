package smon

import "opensvc.com/opensvc/core/topology"

func (o *smon) orchestratePlaced() {
	if o.state.IsLeader {
		o.orchestratePlacedStart()
	} else {
		o.orchestratePlacedStop()
	}
}

func (o *smon) orchestratePlacedStart() {
	switch o.objStatus.Topology {
	case topology.Failover:
		o.orchestrateFailoverPlacedStart()
	case topology.Flex:
		o.orchestrateFlexPlacedStart()
	}
}

func (o *smon) orchestratePlacedStop() {
	switch o.objStatus.Topology {
	case topology.Failover:
		o.orchestrateFailoverPlacedStop()
	case topology.Flex:
		o.orchestrateFlexPlacedStop()
	}
}
