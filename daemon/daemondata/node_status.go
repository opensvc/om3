package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
)

// GetNodeStatus returns daemondata deep copy of cluster.Node.<node>
func (t T) GetNodeStatus(node string) *cluster.TNodeStatus {
	return GetNodeStatus(t.cmdC, node)
}

// GetNodeStatus returns deep copy of cluster.Node.<node>.status
func GetNodeStatus(c chan<- any, node string) *cluster.TNodeStatus {
	result := make(chan *cluster.TNodeStatus)
	op := opGetNodeStatus{
		result: result,
		node:   node,
	}
	c <- op
	return <-result
}

type opGetNodeStatus struct {
	node   string
	result chan<- *cluster.TNodeStatus
}

func (o opGetNodeStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeStatus
	if nodeData, ok := d.pending.Cluster.Node[o.node]; ok {
		o.result <- nodeData.Status.DeepCopy()
	} else {
		o.result <- nil
	}
}
