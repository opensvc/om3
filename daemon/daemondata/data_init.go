package daemondata

import (
	"path/filepath"
	"strings"
	"time"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/nodesinfo"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/daemon/hbcache"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/san"
)

func newData(counterCmd chan<- interface{}) *data {
	localNode := hostname.Hostname()
	nodeData := newNodeData(localNode)
	status := cluster.Status{
		Cluster: cluster.Cluster{
			Config: cluster.ClusterConfig{
				ID:    rawconfig.ClusterSection().ID,
				Name:  rawconfig.ClusterSection().Name,
				Nodes: strings.Fields(rawconfig.ClusterSection().Nodes),
			},
			Status: cluster.ClusterStatus{
				Compat: false,
				Frozen: true,
			},
			Object: map[string]object.Status{},

			Node: map[string]cluster.NodeData{
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
		Sub: cluster.Sub{
			Hb: cluster.SubHb{
				Heartbeats: hbcache.Heartbeats(),
				Modes:      make([]cluster.HbMode, 0),
			},
		},
	}
	return &data{
		previousRemoteInfo: make(map[string]remoteInfo),
		pending:            status.DeepCopy(),
		localNode:          localNode,
		counterCmd:         counterCmd,
		gen:                nodeData.Status.Gen[localNode],
		patchQueue:         make(patchQueue),
		subHbMode:          map[string]string{localNode: "undef"},
		hbMessageType:      "undef",
		hbGens:             map[string]map[string]uint64{localNode: map[string]uint64{localNode: 0}},
		hbPatchMsgUpdated:  make(map[string]time.Time),
	}
}

func newNodeData(localNode string) cluster.NodeData {
	nodeFrozenFile := filepath.Join(rawconfig.Paths.Var, "node", "frozen")
	frozen := file.ModTime(nodeFrozenFile)
	nodeStatus := cluster.NodeData{
		Instance: map[string]instance.Instance{},
		Monitor:  cluster.NodeMonitor{},
		Stats:    cluster.NodeStats{},
		Status: cluster.NodeStatus{
			Agent:           "3.0-0",
			API:             8,
			Arbitrators:     map[string]cluster.ArbitratorStatus{},
			Compat:          12,
			Env:             "",
			Frozen:          frozen,
			Gen:             map[string]uint64{localNode: 1},
			Labels:          nodesinfo.Labels{},
			MinAvailMemPct:  0,
			MinAvailSwapPct: 0,
			Speaker:         false,
		},
		Os: cluster.NodeOs{
			Paths: san.Paths{},
		},
	}
	return nodeStatus
}
