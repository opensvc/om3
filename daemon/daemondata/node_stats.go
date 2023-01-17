package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/node"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opGetNodeStatsMap struct {
		result chan<- map[string]node.Stats
	}
	opSetNodeStats struct {
		err   chan<- error
		value node.Stats
	}
)

// SetNodeStats sets Monitor.Node.<localhost>.Stats
func (t T) SetNodeStats(value node.Stats) error {
	err := make(chan error)
	op := opSetNodeStats{
		err:   err,
		value: value,
	}
	t.cmdC <- op
	return <-err
}

// GetNodeStatsMap returns a map of NodeStats indexed by nodename
func (t T) GetNodeStatsMap() map[string]node.Stats {
	result := make(chan map[string]node.Stats)
	op := opGetNodeStatsMap{
		result: result,
	}
	t.cmdC <- op
	return <-result
}

func (o opGetNodeStatsMap) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeStatsMap
	m := make(map[string]node.Stats)
	for nodename, nodeData := range d.pending.Cluster.Node {
		m[nodename] = *nodeData.Stats.DeepCopy()
	}
	o.result <- m
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
