package daemondata

import (
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
	"github.com/opensvc/om3/util/san"
)

var startedAt = time.Now()

func newData() *data {
	localNode := hostname.Hostname()

	nodeData := newNodeData(localNode)

	node.MonitorData.Set(localNode, &nodeData.Monitor)
	node.StatusData.Set(localNode, &nodeData.Status)
	node.StatsData.Set(localNode, &nodeData.Stats)
	node.ConfigData.Set(localNode, &nodeData.Config)
	node.GenData.Set(localNode, &nodeData.Status.Gen)

	daemonsubsystem.DataDaemondata.Set(localNode, &nodeData.Daemon.Daemondata)
	daemonsubsystem.DataHeartbeat.Set(localNode, &nodeData.Daemon.Heartbeat)

	status := clusterdump.Data{
		Cluster: clusterdump.Cluster{
			Status: clusterdump.Status{
				IsCompat: false,
				IsFrozen: true,
			},
			Object: map[string]object.Status{},

			Node: map[string]node.Node{
				localNode: nodeData,
			},
		},
		Daemon: daemonsubsystem.DaemonLocal{
			Nodename: localNode,
		},
	}

	initialMsgType := "undef"
	return &data{
		gen:                nodeData.Status.Gen[localNode],
		hbGens:             map[string]map[string]uint64{localNode: {localNode: 0}},
		hbMessageType:      initialMsgType,
		hbPatchMsgUpdated:  make(map[string]time.Time),
		localNode:          localNode,
		clusterNodes:       map[string]struct{}{localNode: {}},
		clusterData:        msgbus.NewClusterData(status.DeepCopy()),
		eventQueue:         make(eventQueue),
		previousRemoteInfo: make(map[string]remoteInfo),
		hbMsgPatchLength:   map[string]int{localNode: 0},
		hbMsgType:          map[string]string{localNode: initialMsgType},
		labelLocalNode:     pubsub.Label{"node", hostname.Hostname()},
	}
}

func newNodeData(localNode string) node.Node {
	nodeFrozenFile := filepath.Join(rawconfig.Paths.Var, "node", "frozen")
	frozen := file.ModTime(nodeFrozenFile)
	now := time.Now()
	nodeStatus := node.Node{
		Config: node.Config{
			// use initial default value
			MaxParallel: object.DefaultNodeMaxParallel,
		},
		Instance: map[string]instance.Instance{},
		Monitor: node.Monitor{
			LocalExpect:  node.MonitorLocalExpectNone,
			GlobalExpect: node.MonitorGlobalExpectNone,
			State:        node.MonitorStateInit, // this prevents imon orchestration
		},
		Stats: node.Stats{},
		Status: node.Status{
			// TODO: API fix
			API:         8,
			Arbitrators: map[string]node.ArbitratorStatus{},
			// TODO: Compat fix
			Compat:          12,
			FrozenAt:        frozen,
			Gen:             map[string]uint64{localNode: 1},
			Labels:          node.Labels{},
			MinAvailMemPct:  0,
			MinAvailSwapPct: 0,
		},
		Os: node.Os{
			Paths: san.Paths{},
		},
		Daemon: daemonsubsystem.Daemon{
			Nodename: localNode,

			Pid: os.Getpid(),

			StartedAt: startedAt,

			Daemondata: daemonsubsystem.Daemondata{
				Status: daemonsubsystem.Status{
					ID:           "daemondata",
					ConfiguredAt: now,
					CreatedAt:    now,
					UpdatedAt:    now,
					State:        "running",
				},
			},
		},
	}
	return nodeStatus
}
