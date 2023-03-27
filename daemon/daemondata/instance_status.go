package daemondata

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
)

// onInstanceStatusDeleted remove .cluster.node.<node>.instance.<path>.status
func (d *data) onInstanceStatusDeleted(c msgbus.InstanceStatusDeleted) {
	d.statCount[idDelInstanceStatus]++
	s := c.Path.String()
	if inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]; ok && inst.Status != nil {
		inst.Status = nil
		d.pending.Cluster.Node[d.localNode].Instance[s] = inst
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"instance", s, "status"},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
}

// onInstanceStatusUpdated updates .cluster.node.<node>.instance.<path>.status
func (d *data) onInstanceStatusUpdated(c msgbus.InstanceStatusUpdated) {
	d.statCount[idSetInstanceStatus]++
	var op jsondelta.Operation
	s := c.Path.String()
	value := c.Value.DeepCopy()
	if inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]; ok {
		inst.Status = value
		d.pending.Cluster.Node[d.localNode].Instance[s] = inst

	} else {
		d.pending.Cluster.Node[d.localNode].Instance[s] = instance.Instance{Status: value}
		op = jsondelta.Operation{
			OpPath:  jsondelta.OperationPath{"instance", s},
			OpValue: jsondelta.NewOptValue(struct{}{}),
			OpKind:  "replace",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	op = jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"instance", s, "status"},
		OpValue: jsondelta.NewOptValue(*value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
}
