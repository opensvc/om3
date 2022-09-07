package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opDelNmon struct {
		err chan<- error
	}

	opSetNmon struct {
		err   chan<- error
		value cluster.NodeMonitor
	}
)

func (o opDelNmon) setError(err error) {
	o.err <- err
}

func (o opDelNmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelNmon
	if _, ok := d.pending.Monitor.Nodes[d.localNode]; ok {
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"monitor"},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	daemonps.PubNmonDelete(d.bus, moncmd.NmonDeleted{
		Node: d.localNode,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opSetNmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNmon
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"monitor"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	daemonps.PubNmonUpdated(d.bus, moncmd.NmonUpdated{
		Node:    d.localNode,
		Monitor: o.value,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
