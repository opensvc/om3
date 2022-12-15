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
	idDelNmon
	idDelSmon
	idGetHbMessage
	idGetInstanceStatus
	idGetNodeData
	idGetNodeStatus
	idGetNodesInfo
	idGetServiceNames
	idGetNmon
	idSetHeartbeatPing
	idSetSubHb
	idGetStatus
	idSetInstanceConfig
	idSetInstanceFrozen
	idSetInstanceStatus
	idSetServiceAgg
	idSetNmon
	idSetNodeOsPaths
	idSetNodeStats
	idSetSmon
	idStats
)

var (
	idToName = map[int]string{
		idUndef:             "undef",
		idApplyFull:         "apply-full",
		idApplyPatch:        "apply-patch",
		idCommitPending:     "commit-pending",
		idDelInstanceConfig: "del-instance-config",
		idDelInstanceStatus: "del-instance-status",
		idDelServiceAgg:     "del-service-agg",
		idDelNmon:           "del-nmon",
		idDelSmon:           "del-smon",
		idGetHbMessage:      "get-hb-message",
		idGetInstanceStatus: "get-instance-status",
		idGetNodeData:       "get-node-data",
		idGetNodeStatus:     "get-node-status",
		idGetNodesInfo:      "get-nodes-info",
		idGetServiceNames:   "get-service-names",
		idGetStatus:         "get-status",
		idSetHeartbeatPing:  "set-heartbeat-ping",
		idSetSubHb:          "set-sub-hb",
		idSetServiceAgg:     "set-service-agg",
		idSetInstanceConfig: "set-instance-config",
		idSetInstanceFrozen: "set-instance-frozen",
		idSetInstanceStatus: "set-instance-status",
		idSetNmon:           "set-nmon",
		idSetNodeOsPaths:    "set-node-os-paths",
		idSetNodeStats:      "set-node-stats",
		idSetSmon:           "set-smon",
		idStats:             "stats",
	}
)
