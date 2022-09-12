package daemondata

import "opensvc.com/opensvc/core/cluster"

// GetNodeStatus returns Monitor.Node.<node>
func GetNodeStatus(c chan<- any, node string) *cluster.NodeStatus {
	result := make(chan *cluster.NodeStatus)
	op := opGetNodeStatus{
		result: result,
		node:   node,
	}
	c <- op
	return <-result
}
