package daemondata

import (
	"context"

	"github.com/opensvc/om3/util/xmap"
)

type (
	HbMessageType struct {
		Type        string
		Nodes       []string
		JoinedNodes []string
	}
	opGetHbMessageType struct {
		errC
		result chan<- HbMessageType
	}
)

// GetHbMessageType returns either "undef", "ping", "full" or "patch"
// Used by nmon start up to determine if rejoin can be skipped.
func (t T) GetHbMessageType() HbMessageType {
	result := make(chan HbMessageType, 1)
	err := make(chan error, 1)
	op := opGetHbMessageType{
		errC:   err,
		result: result,
	}
	t.cmdC <- op
	if <-err != nil {
		return HbMessageType{}
	}
	return <-result
}

func (o opGetHbMessageType) call(ctx context.Context, d *data) error {
	d.counterCmd <- idGetHbMessageType
	o.result <- HbMessageType{
		Type:        d.hbMessageType,
		Nodes:       d.pending.Cluster.Config.Nodes,
		JoinedNodes: xmap.Keys(d.hbGens[d.localNode]),
	}
	return nil
}
