package daemondata

import "opensvc.com/opensvc/core/cluster"

// GetNodeStatus returns Monitor.Node.<node>
func GetNodeStatus(c chan<- any, node string) *cluster.TNodeData {
	result := make(chan *cluster.TNodeData)
	op := opGetNodeStatus{
		result: result,
		node:   node,
	}
	c <- op
	return <-result
}
