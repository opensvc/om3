package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
)

type opGetLocalNodeStatus struct {
	localStatus chan<- *cluster.NodeStatus
}

func (o opGetLocalNodeStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetLocalNodeStatus
	select {
	case <-ctx.Done():
	case o.localStatus <- GetNodeStatus(d.committed, d.localNode):
	}
}

func (t T) GetLocalNodeStatus() *cluster.NodeStatus {
	result := make(chan *cluster.NodeStatus)
	t.cmdC <- opGetLocalNodeStatus{
		localStatus: result,
	}
	return <-result
}
