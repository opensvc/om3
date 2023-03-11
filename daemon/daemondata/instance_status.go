package daemondata

import (
	"context"
	"time"

	"github.com/opensvc/om3/core/instance"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	opGetInstanceStatus struct {
		errC
		status chan<- instance.Status
		path   path.T
		node   string
	}
)

// GetInstanceStatus
//
// Monitor.Node.<localhost>.services.status.*
func (t T) GetInstanceStatus(p path.T, node string) instance.Status {
	err := make(chan error, 1)
	status := make(chan instance.Status, 1)
	op := opGetInstanceStatus{
		errC:   err,
		status: status,
		path:   p,
		node:   node,
	}
	t.cmdC <- op
	if <-err != nil {
		return instance.Status{}
	}
	return <-status
}

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

func (o opGetInstanceStatus) call(ctx context.Context, d *data) error {
	d.statCount[idGetInstanceStatus]++
	s := instance.Status{}
	if nodeStatus, ok := d.pending.Cluster.Node[o.node]; ok {
		if inst, ok := nodeStatus.Instance[o.path.String()]; ok && inst.Status != nil {
			s = *inst.Status
		}
	}
	o.status <- s
	return nil
}

// onInstanceFrozenFileUpdated may publish InstanceStatusUpdated with updated frozen and updated.
// It publishes when instance status exists with both updated and frozen older that c.Updated
func (d *data) onInstanceFrozenFileUpdated(c msgbus.InstanceFrozenFileUpdated) {
	d.statCount[idSetInstanceFrozen]++
	s := c.Path.String()
	eventUpdated := c.Updated
	inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]
	if !ok {
		return
	}
	if inst.Status.Frozen.After(eventUpdated) {
		// skip update, we already have a more recent value for frozen
		return
	}
	if inst.Status.Updated.After(eventUpdated) {
		// skip update, we already have a more recent value of status
		return
	}
	newStatus := inst.Status.DeepCopy()
	newStatus.Frozen = eventUpdated
	newStatus.Updated = eventUpdated

	d.bus.Pub(msgbus.InstanceStatusUpdated{Path: c.Path, Node: d.localNode, Value: *newStatus},
		pubsub.Label{"path", s},
		d.labelLocalNode,
	)
}

// onInstanceFrozenFileRemoved may publish InstanceStatusUpdated with updated frozen and updated.
// It publishes when instance status exists with both updated and frozen older that c.Updated
func (d *data) onInstanceFrozenFileRemoved(c msgbus.InstanceFrozenFileRemoved) {
	d.statCount[idSetInstanceFrozen]++
	s := c.Path.String()
	eventUpdated := c.Updated
	inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]
	if !ok {
		return
	}
	if inst.Status.Frozen.After(eventUpdated) {
		// skip update, we already have a more recent value for frozen
		return
	}
	if inst.Status.Updated.After(eventUpdated) {
		// skip update, we already have a more recent value of status
		return
	}
	newStatus := inst.Status.DeepCopy()
	newStatus.Frozen = time.Time{}
	newStatus.Updated = eventUpdated

	d.bus.Pub(msgbus.InstanceStatusUpdated{Path: c.Path, Node: d.localNode, Value: *newStatus},
		pubsub.Label{"path", s},
		d.labelLocalNode,
	)
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
