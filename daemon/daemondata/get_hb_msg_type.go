package daemondata

import (
	"context"

	"opensvc.com/opensvc/util/xmap"
)

type (
	HbMessageType struct {
		Type        string
		Nodes       []string
		JoinedNodes []string
	}
	opGetHbMessageType struct {
		result chan<- HbMessageType
	}
)

// GetHbMessageType returns either "undef", "ping", "full" or "patch"
// Used by nmon start up to determine if rejoin can be skipped.
func (t T) GetHbMessageType() HbMessageType {
	result := make(chan HbMessageType)
	op := opGetHbMessageType{
		result: result,
	}
	t.cmdC <- op
	return <-result
}

func (o opGetHbMessageType) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetHbMessageType
	o.result <- HbMessageType{
		Type:        d.hbMessageType,
		Nodes:       d.pending.Cluster.Config.Nodes,
		JoinedNodes: xmap.Keys(d.hbGens[d.localNode]),
	}
}
