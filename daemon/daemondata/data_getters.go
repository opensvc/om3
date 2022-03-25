package daemondata

import (
	"opensvc.com/opensvc/core/cluster"
)

func GetNodeStatus(status *cluster.Status, nodename string) *cluster.NodeStatus {
	if nodeStatus, ok := status.Monitor.Nodes[nodename]; ok {
		return &nodeStatus
	}
	return nil
}
