package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opSetNodeStats struct {
		err   chan<- error
		value cluster.NodeStats
	}
)

// SetNodeStats sets Monitor.Node.<localhost>.Stats
func SetNodeStats(c chan<- interface{}, value cluster.NodeStats) error {
	err := make(chan error)
	op := opSetNodeStats{
		err:   err,
		value: value,
	}
	c <- op
	return <-err
}

func (o opSetNodeStats) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNodeStats
	v := d.pending.Cluster.Node[d.localNode]
	if v.Stats == o.value {
		o.err <- nil
		return
	}
	v.Stats = o.value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"stats"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(
		msgbus.NodeStatsUpdated{
			Node:  d.localNode,
			Value: o.value,
		},
		labelLocalNode,
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
