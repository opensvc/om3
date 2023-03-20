package daemondata

import (
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
)

// onNodeConfigUpdated updates .cluster.node.<node>.config
func (d *data) onNodeConfigUpdated(m msgbus.NodeConfigUpdated) {
	d.statCount[idSetNodeConfig]++
	newConfig := m.Value
	v := d.pending.Cluster.Node[d.localNode]
	if v.Config == newConfig {
		return
	}
	v.Config = m.Value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"config"},
		OpValue: jsondelta.NewOptValue(newConfig),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
}
