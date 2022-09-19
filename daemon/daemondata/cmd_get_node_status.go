package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
)

type opGetNodeStatus struct {
	node   string
	result chan<- *cluster.TNode
}

func (o opGetNodeStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeStatus
	nodeStatus := d.pending.GetNodeStatus(o.node).DeepCopy()
	select {
	case <-ctx.Done():
	case o.result <- &nodeStatus:
	}
}

func (t T) GetNodeStatus(node string) *cluster.TNode {
	result := make(chan *cluster.TNode)
	t.cmdC <- opGetNodeStatus{
		node:   node,
		result: result,
	}
	return <-result
}
