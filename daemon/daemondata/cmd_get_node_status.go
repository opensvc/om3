package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
)

type opGetNodeStatus struct {
	node   string
	result chan<- *cluster.NodeStatus
}

func (o opGetNodeStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeStatus
	nodeStatus := d.committed.GetNodeStatus(o.node).DeepCopy()
	select {
	case <-ctx.Done():
	case o.result <- &nodeStatus:
	}
}

func (t T) GetNodeStatus(node string) *cluster.NodeStatus {
	result := make(chan *cluster.NodeStatus)
	t.cmdC <- opGetNodeStatus{
		node:   node,
		result: result,
	}
	return <-result
}
