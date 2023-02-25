package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/nodesinfo"
)

type opGetNodesInfo struct {
	errC
	result chan<- *nodesinfo.NodesInfo
}

// GetNodesInfo returns a NodesInfo struct, ie a map of
// a subset of information from cluster.Node.<node>.Status
// indexed by nodename
func (t T) GetNodesInfo() *nodesinfo.NodesInfo {
	err := make(chan error, 1)
	result := make(chan *nodesinfo.NodesInfo, 1)
	op := opGetNodesInfo{
		errC:   err,
		result: result,
	}
	t.cmdC <- op
	if <-err != nil {
		return nil
	}
	return <-result
}

func (o opGetNodesInfo) call(ctx context.Context, d *data) error {
	d.counterCmd <- idGetNodesInfo
	result := make(nodesinfo.NodesInfo)
	for node, nodeData := range d.pending.Cluster.Node {
		result[node] = nodesinfo.NodeInfo{
			Labels: nodeData.Status.Labels.DeepCopy(),
			Paths:  nodeData.Os.Paths.DeepCopy(),
		}
	}
	o.result <- &result
	return nil
}
