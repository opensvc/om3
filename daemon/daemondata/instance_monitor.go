package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	opDelSmon struct {
		err  chan<- error
		path path.T
	}

	opSetSmon struct {
		err   chan<- error
		path  path.T
		value instance.Monitor
	}
)

// DelSmon
//
// cluster.node.<localhost>.instance.<path>.monitor
func DelSmon(c chan<- interface{}, p path.T) error {
	err := make(chan error)
	op := opDelSmon{
		err:  err,
		path: p,
	}
	c <- op
	return <-err
}

// SetSmon
//
// cluster.node.<localhost>.instance.<path>.monitor
func SetSmon(c chan<- interface{}, p path.T, v instance.Monitor) error {
	err := make(chan error)
	op := opSetSmon{
		err:   err,
		path:  p,
		value: v,
	}
	c <- op
	return <-err
}

func (o opDelSmon) setError(err error) {
	o.err <- err
}

func (o opDelSmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelSmon
	s := o.path.String()
	if inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]; ok && inst.Monitor != nil {
		inst.Monitor = nil
		d.pending.Cluster.Node[d.localNode].Instance[s] = inst
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"instance", s, "monitor"},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	d.bus.Pub(msgbus.InstanceMonitorDeleted{
		Path: o.path,
		Node: d.localNode,
	}, pubsub.Label{"path", s})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opSetSmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetSmon
	var op jsondelta.Operation
	s := o.path.String()
	value := o.value.DeepCopy()
	if inst, ok := d.pending.Cluster.Node[d.localNode].Instance[s]; ok {
		inst.Monitor = value
		d.pending.Cluster.Node[d.localNode].Instance[s] = inst

	} else {
		d.pending.Cluster.Node[d.localNode].Instance[s] = instance.Instance{Monitor: value}
		op = jsondelta.Operation{
			OpPath:  jsondelta.OperationPath{"instance", s},
			OpValue: jsondelta.NewOptValue(struct{}{}),
			OpKind:  "replace",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	op = jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"instance", s, "monitor"},
		OpValue: jsondelta.NewOptValue(*value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(msgbus.InstanceMonitorUpdated{
		Path:   o.path,
		Node:   d.localNode,
		Status: o.value,
	}, pubsub.Label{"path", s})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
