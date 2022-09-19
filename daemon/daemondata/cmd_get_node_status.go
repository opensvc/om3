package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
)

type opGetNodeStatus struct {
	node   string
	result chan<- *cluster.TNodeData
}

func (o opGetNodeStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeStatus
	nodeStatus := d.pending.GetNodeStatus(o.node).DeepCopy()
	select {
	case <-ctx.Done():
	case o.result <- &nodeStatus:
	}
}

func (t T) GetNodeStatus(node string) *cluster.TNodeData {
	result := make(chan *cluster.TNodeData)
	t.cmdC <- opGetNodeStatus{
		node:   node,
		result: result,
	}
	return <-result
}
