package daemondata

import (
	"path/filepath"
	"time"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/nodesinfo"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/san"
)

func newData() *data {
	localNode := hostname.Hostname()
	nodeData := newNodeData(localNode)
	status := cluster.Data{
		Cluster: cluster.Cluster{
			Status: cluster.Status{
				Compat: false,
				Frozen: true,
			},
			Object: map[string]object.Status{},

			Node: map[string]node.Node{
				localNode: nodeData,
			},
		},
		Daemon: cluster.Deamon{
			Collector: cluster.DaemonCollector{},
			DNS:       cluster.DaemonDNS{},
			Scheduler: cluster.DaemonScheduler{},
			Listener:  cluster.DaemonListener{},
			Monitor: cluster.DaemonMonitor{
				DaemonSubsystemStatus: cluster.DaemonSubsystemStatus{},
			},
			Hb: cluster.DaemonHb{
				Streams: make([]cluster.HeartbeatStream, 0),
				Modes:   make([]cluster.HbMode, 0),
			},
		},
	}
	initialMsgType := "undef"
	return &data{
		statCount:          make(map[int]uint64),
		gen:                nodeData.Status.Gen[localNode],
		hbGens:             map[string]map[string]uint64{localNode: map[string]uint64{localNode: 0}},
		hbMessageType:      initialMsgType,
		hbPatchMsgUpdated:  make(map[string]time.Time),
		localNode:          localNode,
		clusterNodes:       map[string]struct{}{localNode: struct{}{}},
		pending:            status.DeepCopy(),
		patchQueue:         make(patchQueue),
		previousRemoteInfo: make(map[string]remoteInfo),
		hbMsgMode:          map[string]string{localNode: initialMsgType},
		hbMsgType:          map[string]string{localNode: initialMsgType},
	}
}

func newNodeData(localNode string) node.Node {
	nodeFrozenFile := filepath.Join(rawconfig.Paths.Var, "node", "frozen")
	frozen := file.ModTime(nodeFrozenFile)
	nodeStatus := node.Node{
		Instance: map[string]instance.Instance{},
		Monitor: node.Monitor{
			LocalExpect:  node.MonitorLocalExpectNone,
			GlobalExpect: node.MonitorGlobalExpectNone,
			State:        node.MonitorStateIdle,
		},
		Stats: node.Stats{},
		Status: node.Status{
			Agent:           "3.0-0",
			API:             8,
			Arbitrators:     map[string]node.ArbitratorStatus{},
			Compat:          12,
			Env:             "",
			Frozen:          frozen,
			Gen:             map[string]uint64{localNode: 1},
			Labels:          nodesinfo.Labels{},
			MinAvailMemPct:  0,
			MinAvailSwapPct: 0,
			Speaker:         false,
		},
		Os: node.Os{
			Paths: san.Paths{},
		},
	}
	return nodeStatus
}
