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
	nodeData := newNodeData(localNode)
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

			Node: map[string]cluster.TNodeData{
				localNode: nodeData,
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
		gen:             nodeData.Status.Gen[localNode],
		remotesNeedFull: make(map[string]bool),
		patchQueue:      make(patchQueue),
	}
}

func newNodeData(localNode string) cluster.TNodeData {
	nodeStatus := cluster.TNodeData{
		Instance: map[string]instance.Instance{},
		Monitor:  cluster.NodeMonitor{},
		Stats:    cluster.NodeStatusStats{},
		Status: cluster.TNodeStatus{
			Agent:           "3.0-0",
			API:             8,
			Arbitrators:     map[string]cluster.ArbitratorStatus{},
			Compat:          11,
			Env:             "",
			Frozen:          time.Time{},
			Gen:             map[string]uint64{localNode: 1},
			Labels:          map[string]string{},
			MinAvailMemPct:  0,
			MinAvailSwapPct: 0,
			Speaker:         false,
		},
	}
	return nodeStatus
}
