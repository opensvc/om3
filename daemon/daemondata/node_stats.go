package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
)

type (
	opGetNodeStatsMap struct {
		errC
		result chan<- map[string]node.Stats
	}
)

// GetNodeStatsMap returns a map of NodeStats indexed by nodename
func (t T) GetNodeStatsMap() map[string]node.Stats {
	err := make(chan error, 1)
	result := make(chan map[string]node.Stats, 1)
	op := opGetNodeStatsMap{
		errC:   err,
		result: result,
	}
	t.cmdC <- op
	if <-err != nil {
		return make(map[string]node.Stats)
	}
	return <-result
}

func (o opGetNodeStatsMap) call(ctx context.Context, d *data) error {
	d.statCount[idGetNodeStatsMap]++
	m := make(map[string]node.Stats)
	for nodename, nodeData := range d.pending.Cluster.Node {
		m[nodename] = *nodeData.Stats.DeepCopy()
	}
	o.result <- m
	return nil
}

// onNodeStatsUpdated updates .cluster.node.<node>.stats
func (d *data) onNodeStatsUpdated(m msgbus.NodeStatsUpdated) {
	d.statCount[idSetNodeStats]++
	v := d.pending.Cluster.Node[d.localNode]
	if v.Stats == m.Value {
		return
	}
	v.Stats = m.Value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"stats"},
		OpValue: jsondelta.NewOptValue(m.Value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
}
