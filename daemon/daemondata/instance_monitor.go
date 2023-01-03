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
	opDelInstanceMonitor struct {
		err  chan<- error
		path path.T
	}

	opSetInstanceMonitor struct {
		err   chan<- error
		path  path.T
		value instance.Monitor
	}
)

// DelInstanceMonitor
//
// cluster.node.<localhost>.instance.<path>.monitor
func (t T) DelInstanceMonitor(p path.T) error {
	err := make(chan error)
	op := opDelInstanceMonitor{
		err:  err,
		path: p,
	}
	t.cmdC <- op
	return <-err
}

// SetInstanceMonitor
//
// cluster.node.<localhost>.instance.<path>.monitor
func (t T) SetInstanceMonitor(p path.T, v instance.Monitor) error {
	err := make(chan error)
	op := opSetInstanceMonitor{
		err:   err,
		path:  p,
		value: v,
	}
	t.cmdC <- op
	return <-err
}

func (o opDelInstanceMonitor) setError(err error) {
	o.err <- err
}

func (o opDelInstanceMonitor) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelInstanceMonitor
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
	d.bus.Pub(
		msgbus.InstanceMonitorDeleted{
			Path: o.path,
			Node: d.localNode,
		},
		pubsub.Label{"path", s},
		labelLocalNode,
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opSetInstanceMonitor) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetInstanceMonitor
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
	d.bus.Pub(
		msgbus.InstanceMonitorUpdated{
			Path:  o.path,
			Node:  d.localNode,
			Value: o.value,
		},
		pubsub.Label{"path", s},
		labelLocalNode,
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
