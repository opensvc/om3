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
	idDropPeerNode
	idGetHbMessage
	idGetHbMessageType
	idGetStatus
	idSetHBSendQ
	idStats
)

var (
	idToName = map[int]string{
		idUndef:            "undef",
		idApplyFull:        "apply-full",
		idApplyPatch:       "apply-patch",
		idDropPeerNode:     "drop-peer-node",
		idGetHbMessage:     "get-hb-message",
		idGetHbMessageType: "get-hb-message-type",
		idGetStatus:        "get-status",
		idSetHBSendQ:       "set-hb-send-q",
		idStats:            "stats",
	}
)
