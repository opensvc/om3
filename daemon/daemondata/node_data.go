package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
)

type opGetNodeData struct {
	node   string
	result chan<- *cluster.NodeData
}

// GetNodeData returns a deep copy of cluster.Node.<node>
func (t T) GetNodeData(node string) *cluster.NodeData {
	result := make(chan *cluster.NodeData)
	op := opGetNodeData{
		result: result,
		node:   node,
	}
	t.cmdC <- op
	return <-result
}

func (o opGetNodeData) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeData
	if nodeData, ok := d.pending.Cluster.Node[o.node]; ok {
		o.result <- nodeData.DeepCopy()
	} else {
		o.result <- nil
	}
}
