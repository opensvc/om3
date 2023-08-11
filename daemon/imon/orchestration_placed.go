package imon

import (
	"github.com/opensvc/om3/core/topology"
)

func (o *imon) orchestratePlaced() {
	if o.state.IsHALeader {
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
