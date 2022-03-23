package daemondata

import (
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/util/hostname"
)

func newData(counterCmd chan<- interface{}) *data {
	localNode := hostname.Hostname()
	status := cluster.Status{
		Cluster:   cluster.Info{},
		Collector: cluster.CollectorThreadStatus{},
		DNS:       cluster.DNSThreadStatus{},
		Scheduler: cluster.SchedulerThreadStatus{},
		Listener:  cluster.ListenerThreadStatus{},
		Monitor: cluster.MonitorThreadStatus{
			ThreadStatus: cluster.ThreadStatus{},
			Compat:       false,
			Frozen:       false,
			Nodes: map[string]cluster.NodeStatus{
				localNode: newNodeStatus(localNode),
			},
			Services: map[string]object.AggregatedStatus{},
		},
		Heartbeats: nil,
	}
	return &data{
		current:    &status,
		pending:    deepCopy(&status),
		localNode:  localNode,
		counterCmd: counterCmd,
	}
}

func newNodeStatus(localNode string) cluster.NodeStatus {
	nodeStatus := cluster.NodeStatus{
		Gen:      map[string]uint64{localNode: 0},
		Monitor:  cluster.NodeMonitor{},
		Services: cluster.NodeServices{},
		Stats:    cluster.NodeStatusStats{},
	}
	return nodeStatus
}
