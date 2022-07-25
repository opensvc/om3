package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
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
	if _, ok := d.pending.Monitor.Nodes[d.localNode].Services.Status[s]; ok {
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"services", "status", s},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	daemonps.PubInstStatusDelete(d.bus, s, moncmd.InstStatusDeleted{
		Path: o.path,
		Node: d.localNode,
	})
	select {
	case o.err <- nil:
	case <-ctx.Done():
	}
}

func (o opGetInstanceStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetInstanceStatus
	s := instance.Status{}
	if nodeStatus, ok := d.pending.Monitor.Nodes[o.node]; ok {
		if instStatus, ok := nodeStatus.Services.Status[o.path.String()]; ok {
			s = instStatus
		}
	}
	select {
	case o.status <- s:
	case <-ctx.Done():
	}
}

func (o opSetInstanceStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetInstanceStatus
	s := o.path.String()
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"services", "status", s},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	daemonps.PubInstStatusUpdated(d.bus, s, moncmd.InstStatusUpdated{
		Path:   o.path,
		Node:   d.localNode,
		Status: o.value,
	})
	select {
	case o.err <- nil:
	case <-ctx.Done():
	}
}
