package daemondata

import (
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
)

// onNodeStatusUpdated updates .cluster.node.<node>.status from msgbus.NodeStatusUpdated.
// It preserves the Gen value (because daemondata has the most recent version of gen.
func (d *data) onNodeStatusUpdated(m msgbus.NodeStatusUpdated) {
	daemondataGen := make(map[string]uint64)
	for k, v := range d.pending.Cluster.Node[d.localNode].Status.Gen {
		daemondataGen[k] = v
	}
	v := d.pending.Cluster.Node[d.localNode]
	v.Status = m.Value
	v.Status.Gen = daemondataGen
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"status"},
		OpValue: jsondelta.NewOptValue(m.Value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
}
