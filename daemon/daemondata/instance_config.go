package daemondata

import (
	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
)

// onConfigDeleted removes cluster.node.<node>.instance.<path>.config
func (d *data) onInstanceConfigDeleted(c msgbus.InstanceConfigDeleted) {
	d.statCount[idDelInstanceConfig]++
	s := c.Path.String()
	if inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]; ok && inst.Config != nil {
		inst.Config = nil
		d.pending.Cluster.Node[d.localNode].Instance[s] = inst
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"instance", s, "config"},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
}

// onInstanceConfigUpdated updates cluster.node.<node>.instance.<path>.config
func (d *data) onInstanceConfigUpdated(c msgbus.InstanceConfigUpdated) {
	d.statCount[idSetInstanceConfig]++
	var op jsondelta.Operation
	s := c.Path.String()
	value := c.Value.DeepCopy()
	if inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]; ok {
		inst.Config = value
		d.pending.Cluster.Node[d.localNode].Instance[s] = inst
	} else {
		d.pending.Cluster.Node[d.localNode].Instance[s] = instance.Instance{Config: value}
		op = jsondelta.Operation{
			OpPath:  jsondelta.OperationPath{"instance", s},
			OpValue: jsondelta.NewOptValue(struct{}{}),
			OpKind:  "replace",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	op = jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"instance", s, "config"},
		OpValue: jsondelta.NewOptValue(*value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
}
