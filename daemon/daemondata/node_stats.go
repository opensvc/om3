package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opGetNodeStatsMap struct {
		result chan<- map[string]cluster.NodeStats
	}
	opSetNodeStats struct {
		err   chan<- error
		value cluster.NodeStats
	}
)

// SetNodeStats sets Monitor.Node.<localhost>.Stats
func (t T) SetNodeStats(value cluster.NodeStats) error {
	err := make(chan error)
	op := opSetNodeStats{
		err:   err,
		value: value,
	}
	t.cmdC <- op
	return <-err
}

// GetNodeStatsMap returns a map of NodeStats indexed by nodename
func (t T) GetNodeStatsMap() map[string]cluster.NodeStats {
	result := make(chan map[string]cluster.NodeStats)
	op := opGetNodeStatsMap{
		result: result,
	}
	t.cmdC <- op
	return <-result
}

func (o opGetNodeStatsMap) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNodeStatsMap
	m := make(map[string]cluster.NodeStats)
	for node, nodeData := range d.pending.Cluster.Node {
		m[node] = *nodeData.Stats.DeepCopy()
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
