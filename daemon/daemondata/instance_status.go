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

	opSetInstanceFrozen struct {
		errC
		path   path.T
		frozen time.Time
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

// SetInstanceFrozen
//
// Monitor.Node.<localhost>.instance.<p>.status.Frozen
func (t T) SetInstanceFrozen(p path.T, frozen time.Time) error {
	err := make(chan error, 1)
	op := opSetInstanceFrozen{
		errC:   err,
		path:   p,
		frozen: frozen,
	}
	t.cmdC <- op
	return <-err
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

func (o opSetInstanceFrozen) call(ctx context.Context, d *data) error {
	d.statCount[idSetInstanceFrozen]++
	var (
		op   jsondelta.Operation
		ok   bool
		inst instance.Instance
	)
	s := o.path.String()
	value := o.frozen
	if inst, ok = d.pending.Cluster.Node[d.localNode].Instance[s]; !ok {
		return nil
	}
	newStatus := inst.Status.DeepCopy()
	newStatus.Frozen = value
	// TODO don't update newStatus.Updated if more recent
	if value.IsZero() {
		newStatus.Updated = time.Now()
	} else {
		newStatus.Updated = value
	}
	newStatus.Frozen = value
	inst.Status = newStatus
	d.pending.Cluster.Node[d.localNode].Instance[s] = inst

	op = jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"instance", s, "status"},
		OpValue: jsondelta.NewOptValue(inst.Status.DeepCopy()),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(msgbus.InstanceStatusUpdated{Path: o.path, Node: d.localNode, Value: *newStatus},
		pubsub.Label{"path", s},
		d.labelLocalNode,
	)
	return nil
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
