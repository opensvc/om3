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
	idDelServiceAgg
	idDelNodeMonitor
	idDelInstanceMonitor
	idGetHbMessage
	idGetInstanceStatus
	idGetNodeData
	idGetNodeStatus
	idGetNodesInfo
	idGetServiceNames
	idGetNodeMonitor
	idSetHeartbeatPing
	idSetSubHb
	idGetStatus
	idSetInstanceConfig
	idSetInstanceFrozen
	idSetInstanceStatus
	idSetServiceAgg
	idSetNodeMonitor
	idSetNodeOsPaths
	idSetNodeStats
	idSetInstanceMonitor
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
		idDelServiceAgg:      "del-service-agg",
		idDelNodeMonitor:     "del-nmon",
		idDelInstanceMonitor: "del-smon",
		idGetHbMessage:       "get-hb-message",
		idGetInstanceStatus:  "get-instance-status",
		idGetNodeData:        "get-node-data",
		idGetNodeStatus:      "get-node-status",
		idGetNodesInfo:       "get-nodes-info",
		idGetServiceNames:    "get-service-names",
		idGetStatus:          "get-status",
		idSetHeartbeatPing:   "set-heartbeat-ping",
		idSetSubHb:           "set-sub-hb",
		idSetServiceAgg:      "set-service-agg",
		idSetInstanceConfig:  "set-instance-config",
		idSetInstanceFrozen:  "set-instance-frozen",
		idSetInstanceStatus:  "set-instance-status",
		idSetNodeMonitor:     "set-nmon",
		idSetNodeOsPaths:     "set-node-os-paths",
		idSetNodeStats:       "set-node-stats",
		idSetInstanceMonitor: "set-smon",
		idStats:              "stats",
	}
)
