package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
)

// GetNodeData returns daemondata deep copy of cluster.Node.<node>
func (t T) GetNodeData(node string) *cluster.TNodeData {
	return GetNodeData(t.cmdC, node)
}

// GetNodeData returns deep copy of cluster.Node.<node>
func GetNodeData(c chan<- any, node string) *cluster.TNodeData {
	result := make(chan *cluster.TNodeData)
	op := opGetNodeData{
		result: result,
		node:   node,
	}
	c <- op
	return <-result
}

type opGetNodeData struct {
	node   string
	result chan<- *cluster.TNodeData
}

func (o opGetNodeData) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeData
	if nodeData, ok := d.pending.Cluster.Node[o.node]; ok {
		o.result <- nodeData.DeepCopy()
	} else {
		o.result <- nil
	}
}
