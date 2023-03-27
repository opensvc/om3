package daemondata

import (
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
)

// onNodeStatsUpdated updates .cluster.node.<node>.stats
func (d *data) onNodeStatsUpdated(m msgbus.NodeStatsUpdated) {
	d.statCount[idSetNodeStats]++
	v := d.pending.Cluster.Node[d.localNode]
	if v.Stats == m.Value {
		return
	}
	v.Stats = *m.Value.DeepCopy()
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"stats"},
		OpValue: jsondelta.NewOptValue(m.Value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
}
