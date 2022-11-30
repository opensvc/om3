package daemondata

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	opDelInstanceStatus struct {
		err  chan<- error
		path path.T
	}

	opGetInstanceStatus struct {
		status chan<- instance.Status
		path   path.T
		node   string
	}

	opSetInstanceStatus struct {
		err   chan<- error
		path  path.T
		value instance.Status
	}

	opSetInstanceFrozen struct {
		err    chan<- error
		path   path.T
		frozen time.Time
	}
)

// DelInstanceStatus
//
// Monitor.Node.<localhost>.services.status.*
func DelInstanceStatus(c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelInstanceStatus{
		err:  err,
		path: p,
	}
	c <- op
	return <-err
}

// GetInstanceStatus
//
// Monitor.Node.<localhost>.services.status.*
func GetInstanceStatus(c chan<- interface{}, p path.T, node string) instance.Status {
	status := make(chan instance.Status)
	op := opGetInstanceStatus{
		status: status,
		path:   p,
		node:   node,
	}
	c <- op
	return <-status
}

// SetInstanceFrozen
//
// Monitor.Node.<localhost>.instance.<p>.status.Frozen
func SetInstanceFrozen(c chan<- interface{}, p path.T, frozen time.Time) error {
	err := make(chan error)
	op := opSetInstanceFrozen{
		err:    err,
		path:   p,
		frozen: frozen,
	}
	c <- op
	return <-err
}

// SetInstanceStatus
//
// Monitor.Node.<localhost>.services.status.*
func SetInstanceStatus(c chan<- interface{}, p path.T, v instance.Status) error {
	err := make(chan error)
	op := opSetInstanceStatus{
		err:   err,
		path:  p,
		value: v,
	}
	c <- op
	return <-err
}

func (o opDelInstanceStatus) setError(ctx context.Context, err error) {
	select {
	case o.err <- err:
	case <-ctx.Done():
	}
}

func (o opDelInstanceStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelInstanceStatus
	s := o.path.String()
	if inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]; ok && inst.Status != nil {
		inst.Status = nil
		d.pending.Cluster.Node[d.localNode].Instance[s] = inst
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"instance", s, "status"},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	d.bus.Pub(msgbus.InstanceStatusDeleted{
		Path: o.path,
		Node: d.localNode,
	}, pubsub.Label{"path", s})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opGetInstanceStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetInstanceStatus
	s := instance.Status{}
	if nodeStatus, ok := d.pending.Cluster.Node[o.node]; ok {
		if inst, ok := nodeStatus.Instance[o.path.String()]; ok && inst.Status != nil {
			s = *inst.Status
		}
	}
	select {
	case <-ctx.Done():
	case o.status <- s:
	}
}

func (o opSetInstanceFrozen) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetInstanceFrozen
	var (
		op   jsondelta.Operation
		ok   bool
		inst instance.Instance
	)
	s := o.path.String()
	value := o.frozen
	if inst, ok = d.pending.Cluster.Node[d.localNode].Instance[s]; !ok {
		o.err <- nil
		return
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
	d.bus.Pub(msgbus.InstanceStatusUpdated{
		Path:   o.path,
		Node:   d.localNode,
		Status: *newStatus,
	}, pubsub.Label{"path", s})
	o.err <- nil
}

func (o opSetInstanceStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetInstanceStatus
	var op jsondelta.Operation
	s := o.path.String()
	value := o.value.DeepCopy()
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
	d.bus.Pub(msgbus.InstanceStatusUpdated{
		Path:   o.path,
		Node:   d.localNode,
		Status: o.value,
	}, pubsub.Label{"path", s})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
