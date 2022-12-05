package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/nodesinfo"
)

// GetNodesInfo returns a NodesInfo struct, ie a map of
// a subset of information from cluster.Node.<node>.Status
// indexed by nodename
func (t T) GetNodesInfo() *nodesinfo.NodesInfo {
	return GetNodesInfo(t.cmdC)
}

func GetNodesInfo(c chan<- any) *nodesinfo.NodesInfo {
	result := make(chan *nodesinfo.NodesInfo)
	op := opGetNodesInfo{
		result: result,
	}
	c <- op
	return <-result
}

type opGetNodesInfo struct {
	result chan<- *nodesinfo.NodesInfo
}

func (o opGetNodesInfo) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodesInfo
	result := make(nodesinfo.NodesInfo)
	for node, nodeData := range d.pending.Cluster.Node {
		result[node] = nodesinfo.NodeInfo{
			Labels: nodeData.Status.Labels.DeepCopy(),
			Paths:  nodeData.Os.Paths.DeepCopy(),
		}
	}
	o.result <- &result
}
