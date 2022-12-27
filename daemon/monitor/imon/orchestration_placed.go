package imon

import "opensvc.com/opensvc/core/topology"

func (o *imon) orchestratePlaced() {
	if o.state.IsLeader {
		o.orchestratePlacedStart()
	} else {
		o.orchestratePlacedStop()
	}
}

func (o *imon) orchestratePlacedStart() {
	switch o.objStatus.Topology {
	case topology.Failover:
		o.orchestrateFailoverPlacedStart()
	case topology.Flex:
		o.orchestrateFlexPlacedStart()
	}
}

func (o *imon) orchestratePlacedStop() {
	switch o.objStatus.Topology {
	case topology.Failover:
		o.orchestrateFailoverPlacedStop()
	case topology.Flex:
		o.orchestrateFlexPlacedStop()
	}
}
