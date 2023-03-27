package daemondata

import (
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
)

// onNodeMonitorDeleted removes .cluster.node.<node>.monitor
func (d *data) onNodeMonitorDeleted(m msgbus.NodeMonitorDeleted) {
	d.statCount[idDelNodeMonitor]++
	if _, ok := d.pending.Cluster.Node[d.localNode]; ok {
		delete(d.pending.Cluster.Node, d.localNode)
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"monitor"},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
}

// onNodeMonitorUpdated updates .cluster.node.<node>.monitor
func (d *data) onNodeMonitorUpdated(m msgbus.NodeMonitorUpdated) {
	d.statCount[idSetNodeMonitor]++
	newValue := d.pending.Cluster.Node[d.localNode]
	newValue.Monitor = m.Value
	d.pending.Cluster.Node[d.localNode] = newValue
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"monitor"},
		OpValue: jsondelta.NewOptValue(m.Value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
}
