package daemondata

import (
	"context"

	"github.com/opensvc/om3/util/callcount"
)

type opStats struct {
	errC
	stats chan<- map[string]uint64
}

func (t T) Stats() callcount.Stats {
	err := make(chan error, 1)
	stats := make(chan map[string]uint64, 1)
	cmd := opStats{stats: stats, errC: err}
	t.cmdC <- cmd
	if <-err != nil {
		return nil
	}
	return <-stats
}

func (o opStats) call(_ context.Context, d *data) error {
	d.statCount[idStats]++
	stats := make(map[string]uint64)
	for id, count := range d.statCount {
		stats[idToName[id]] = count
	}
	o.stats <- stats
	return nil
}

const (
	idUndef = iota
	idApplyFull
	idApplyPatch
	idCommitPending
	idClusterConfigUpdated
	idClusterStatusUpdated
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
	idSetClusterConfig
	idSetClusterStatus
	idSetDaemonHb
	idSetHBSendQ
	idSetInstanceConfig
	idSetInstanceFrozen
	idSetInstanceMonitor
	idSetInstanceStatus
	idSetNodeConfig
	idSetNodeMonitor
	idSetNodeOsPaths
	idSetNodeStats
	idSetNodeStatusArbitrator
	idSetNodeStatusFrozen
	idSetNodeStatusLabels
	idSetObjectStatus
	idStats
)

var (
	idToName = map[int]string{
		idUndef:                   "undef",
		idApplyFull:               "apply-full",
		idApplyPatch:              "apply-patch",
		idCommitPending:           "commit-pending",
		idClusterConfigUpdated:    "cluster-config-updated",
		idClusterStatusUpdated:    "cluster-status-updated",
		idDelInstanceConfig:       "del-instance-config",
		idDelInstanceStatus:       "del-instance-status",
		idDelObjectStatus:         "del-object-status",
		idDelNodeMonitor:          "del-node-monitor",
		idDelInstanceMonitor:      "del-intance-monitor",
		idDropPeerNode:            "drop-peer-node",
		idGetHbMessage:            "get-hb-message",
		idGetHbMessageType:        "get-hb-message-type",
		idGetInstanceMonitorMap:   "get-instance-monitor-map",
		idGetInstanceStatus:       "get-instance-status",
		idGetNode:                 "get-node",
		idGetNodeStatus:           "get-node-status",
		idGetNodeStatusMap:        "get-node-status-map",
		idGetNodesInfo:            "get-nodes-info",
		idGetServiceNames:         "get-service-names",
		idGetStatus:               "get-status",
		idSetClusterConfig:        "set-cluster-config",
		idSetClusterStatus:        "set-cluster-status",
		idSetDaemonHb:             "set-daemon-hb",
		idSetHBSendQ:              "set-hb-send-q",
		idSetInstanceConfig:       "set-instance-config",
		idSetInstanceFrozen:       "set-instance-frozen",
		idSetInstanceMonitor:      "set-instance-monitor",
		idSetInstanceStatus:       "set-instance-status",
		idSetNodeConfig:           "set-node-config",
		idSetNodeMonitor:          "set-node-monitor",
		idSetNodeOsPaths:          "set-node-os-paths",
		idSetNodeStats:            "set-node-stats",
		idSetNodeStatusArbitrator: "set-node-status-arbitrator",
		idSetNodeStatusFrozen:     "set-node-status-frozen",
		idSetNodeStatusLabels:     "set-node-status-labels",
		idSetObjectStatus:         "set-object-status",
		idStats:                   "stats",
	}
)
