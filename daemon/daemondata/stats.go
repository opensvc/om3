package daemondata

import (
	"context"

	"github.com/opensvc/om3/util/callcount"
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
	idClusterConfigUpdated
	idDelInstanceConfig
	idDelInstanceStatus
	idDelObjectStatus
	idDelNodeMonitor
	idDelInstanceMonitor
	idDropPeerNode
	idGetHbMessage
	idGetHbMessageType
	idGetInstanceConfig
	idGetInstanceMonitorMap
	idGetInstanceStatus
	idGetNode
	idGetNodeMonitor
	idGetNodeMonitorMap
	idGetNodeStatus
	idGetNodeStatusMap
	idGetNodeStatsMap
	idGetNodesInfo
	idGetServiceNames
	idGetStatus
	idSetHeartbeatPing
	idSetClusterConfig
	idSetDaemonHb
	idSetInstanceConfig
	idSetInstanceFrozen
	idSetInstanceMonitor
	idSetInstanceStatus
	idSetNodeConfig
	idSetNodeMonitor
	idSetNodeOsPaths
	idSetNodeStats
	idSetObjectStatus
	idStats
)

var (
	idToName = map[int]string{
		idUndef:                 "undef",
		idApplyFull:             "apply-full",
		idApplyPatch:            "apply-patch",
		idCommitPending:         "commit-pending",
		idClusterConfigUpdated:  "cluster-config-updated",
		idDelInstanceConfig:     "del-instance-config",
		idDelInstanceStatus:     "del-instance-status",
		idDelObjectStatus:       "del-object-status",
		idDelNodeMonitor:        "del-node-monitor",
		idDelInstanceMonitor:    "del-intance-monitor",
		idDropPeerNode:          "drop-peer-node",
		idGetHbMessage:          "get-hb-message",
		idGetHbMessageType:      "get-hb-message-type",
		idGetInstanceMonitorMap: "get-instance-monitor-map",
		idGetInstanceStatus:     "get-instance-status",
		idGetNode:               "get-node",
		idGetNodeStatus:         "get-node-status",
		idGetNodeStatusMap:      "get-node-status-map",
		idGetNodesInfo:          "get-nodes-info",
		idGetServiceNames:       "get-service-names",
		idGetStatus:             "get-status",
		idSetClusterConfig:      "set-cluster-config",
		idSetDaemonHb:           "set-daemon-hb",
		idSetObjectStatus:       "set-object-status",
		idSetInstanceConfig:     "set-instance-config",
		idSetInstanceFrozen:     "set-instance-frozen",
		idSetInstanceMonitor:    "set-instance-monitor",
		idSetInstanceStatus:     "set-instance-status",
		idSetNodeConfig:         "set-node-config",
		idSetNodeMonitor:        "set-node-monitor",
		idSetNodeOsPaths:        "set-node-os-paths",
		idSetNodeStats:          "set-node-stats",
		idStats:                 "stats",
	}
)
