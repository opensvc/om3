package daemondata

import (
	"strings"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/hostname"
)

func newData(counterCmd chan<- interface{}) *data {
	localNode := hostname.Hostname()
	localNodeStatus := newNodeStatus(localNode)
	status := cluster.Status{
		Cluster: cluster.TCluster{
			Config: cluster.ClusterConfig{
				ID:    rawconfig.ClusterSection().ID,
				Name:  rawconfig.ClusterSection().Name,
				Nodes: strings.Fields(rawconfig.ClusterSection().Nodes),
			},
			Status: cluster.TClusterStatus{
				Compat: false,
				Frozen: true,
			},
			Object: map[string]object.AggregatedStatus{},

			Node: map[string]cluster.NodeStatus{
				localNode: localNodeStatus,
			},
		},
		Collector: cluster.CollectorThreadStatus{},
		DNS:       cluster.DNSThreadStatus{},
		Scheduler: cluster.SchedulerThreadStatus{},
		Listener:  cluster.ListenerThreadStatus{},
		Monitor: cluster.MonitorThreadStatus{
			ThreadStatus: cluster.ThreadStatus{},
		},
		Heartbeats: nil,
	}
	return &data{
		previous:        &status,
		pending:         status.DeepCopy(),
		localNode:       localNode,
		counterCmd:      counterCmd,
		mergedFromPeer:  make(gens),
		mergedOnPeer:    make(gens),
		gen:             localNodeStatus.Gen[localNode],
		remotesNeedFull: make(map[string]bool),
		patchQueue:      make(patchQueue),
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
		Frozen:          time.Time{},
		Gen:             map[string]uint64{localNode: 1},
		Labels:          map[string]string{},
		MinAvailMemPct:  0,
		MinAvailSwapPct: 0,
		Monitor:         cluster.NodeMonitor{},
		Services: cluster.NodeServices{
			Config: map[string]instance.Config{},
			Status: map[string]instance.Status{},
			Smon:   map[string]instance.Monitor{},
		},
		Stats: cluster.NodeStatusStats{},
	}
	return nodeStatus
}
