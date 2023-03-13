package daemondata

import (
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
)

// onNodeOsPathsUpdated updates .cluster.node.<node>.os.paths
func (d *data) onNodeOsPathsUpdated(m msgbus.NodeOsPathsUpdated) {
	d.statCount[idSetNodeOsPaths]++
	v := d.pending.Cluster.Node[d.localNode]
	v.Os.Paths = m.Value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"os", "paths"},
		OpValue: jsondelta.NewOptValue(m.Value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
}
