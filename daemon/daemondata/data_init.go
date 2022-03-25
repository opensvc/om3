package daemondata

import (
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/timestamp"
)

func newData(counterCmd chan<- interface{}) *data {
	node := object.NewNode()
	config := node.MergedConfig()
	localNode := hostname.Hostname()
	status := cluster.Status{
		Cluster: cluster.Info{
			ID:    config.Get(key.New("cluster", "id")),
			Name:  config.Get(key.New("cluster", "name")),
			Nodes: config.GetSlice(key.New("cluster", "nodes")),
		},
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
		Agent:           "3.0-0",
		Speaker:         false,
		API:             7,
		Arbitrators:     map[string]cluster.ArbitratorStatus{},
		Compat:          10,
		Env:             "",
		Frozen:          timestamp.T{},
		Gen:             map[string]uint64{localNode: 1},
		Labels:          map[string]string{},
		MinAvailMemPct:  0,
		MinAvailSwapPct: 0,
		Monitor:         cluster.NodeMonitor{},
		Services: cluster.NodeServices{
			Config: map[string]instance.Config{},
			Status: map[string]instance.Status{},
		},
		Stats: cluster.NodeStatusStats{},
	}
	return nodeStatus
}
