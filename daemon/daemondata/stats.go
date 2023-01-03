package daemondata

import (
	"context"

	"opensvc.com/opensvc/util/callcount"
)

type opStats struct {
	stats chan<- callcount.Stats
}

func (t T) Stats() callcount.Stats {
	stats := make(chan callcount.Stats)
	t.cmdC <- opStats{stats: stats}
	return <-stats
}

func (o opStats) call(ctx context.Context, d *data) {
	d.counterCmd <- idStats
	select {
	case <-ctx.Done():
	case o.stats <- callcount.GetStats(d.counterCmd):
	}
}

const (
	idUndef = iota
	idApplyFull
	idApplyPatch
	idCommitPending
	idDelInstanceConfig
	idDelInstanceStatus
	idDelObjectStatus
	idDelNodeMonitor
	idDelInstanceMonitor
	idDropPeerNode
	idGetHbMessage
	idGetHbMessageType
	idGetInstanceStatus
	idGetNodeData
	idGetNodeMonitor
	idGetNodeMonitorMap
	idGetNodeStatus
	idGetNodeStatusMap
	idGetNodeStatsMap
	idGetNodesInfo
	idGetServiceNames
	idGetStatus
	idSetHeartbeatPing
	idSetSubHb
	idSetInstanceConfig
	idSetInstanceFrozen
	idSetInstanceMonitor
	idSetInstanceStatus
	idSetNodeMonitor
	idSetNodeOsPaths
	idSetNodeStats
	idSetObjectStatus
	idStats
)

var (
	idToName = map[int]string{
		idUndef:              "undef",
		idApplyFull:          "apply-full",
		idApplyPatch:         "apply-patch",
		idCommitPending:      "commit-pending",
		idDelInstanceConfig:  "del-instance-config",
		idDelInstanceStatus:  "del-instance-status",
		idDelObjectStatus:    "del-object-status",
		idDelNodeMonitor:     "del-node-monitor",
		idDelInstanceMonitor: "del-intance-monitor",
		idDropPeerNode:       "drop-peer-node",
		idGetHbMessage:       "get-hb-message",
		idGetHbMessageType:   "get-hb-message-type",
		idGetInstanceStatus:  "get-instance-status",
		idGetNodeData:        "get-node-data",
		idGetNodeStatus:      "get-node-status",
		idGetNodeStatusMap:   "get-node-status-map",
		idGetNodesInfo:       "get-nodes-info",
		idGetServiceNames:    "get-service-names",
		idGetStatus:          "get-status",
		idSetSubHb:           "set-sub-hb",
		idSetObjectStatus:    "set-object-status",
		idSetInstanceConfig:  "set-instance-config",
		idSetInstanceFrozen:  "set-instance-frozen",
		idSetInstanceStatus:  "set-instance-status",
		idSetNodeMonitor:     "set-node-monitor",
		idSetNodeOsPaths:     "set-node-os-paths",
		idSetNodeStats:       "set-node-stats",
		idSetInstanceMonitor: "set-instance-monitor",
		idStats:              "stats",
	}
)
