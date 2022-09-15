package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
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
)

func (o opDelInstanceStatus) setError(ctx context.Context, err error) {
	select {
	case o.err <- err:
	case <-ctx.Done():
	}
}

func (o opDelInstanceStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelInstanceStatus
	s := o.path.String()
	if _, ok := d.pending.Cluster.Node[d.localNode].Services.Status[s]; ok {
		delete(d.pending.Cluster.Node[d.localNode].Services.Status, s)
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"services", "status", s},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	msgbus.PubInstStatusDelete(d.bus, s, msgbus.InstStatusDeleted{
		Path: o.path,
		Node: d.localNode,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opGetInstanceStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetInstanceStatus
	s := instance.Status{}
	if nodeStatus, ok := d.pending.Cluster.Node[o.node]; ok {
		if instStatus, ok := nodeStatus.Services.Status[o.path.String()]; ok {
			s = instStatus
		}
	}
	select {
	case <-ctx.Done():
	case o.status <- s:
	}
}

func (o opSetInstanceStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetInstanceStatus
	s := o.path.String()
	d.pending.Cluster.Node[d.localNode].Services.Status[s] = o.value
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"services", "status", s},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	msgbus.PubInstStatusUpdated(d.bus, s, msgbus.InstStatusUpdated{
		Path:   o.path,
		Node:   d.localNode,
		Status: o.value,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
