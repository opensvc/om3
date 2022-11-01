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
	idApplyPing
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
	idSetHeartbeats
	idGetStatus
	idSetInstanceConfig
	idSetInstanceStatus
	idSetServiceAgg
	idSetNmon
	idSetSmon
	idStats
)

var (
	idToName = map[int]string{
		idUndef:             "undef",
		idApplyFull:         "apply-full",
		idApplyPatch:        "apply-patch",
		idApplyPing:         "apply-ping",
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
		idSetHeartbeats:     "set-heartbeat",
		idSetServiceAgg:     "set-service-agg",
		idSetInstanceConfig: "set-instance-config",
		idSetInstanceStatus: "set-instance-status",
		idSetNmon:           "set-nmon",
		idSetSmon:           "set-smon",
		idStats:             "stats",
	}
)
