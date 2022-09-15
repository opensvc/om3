package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
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
	opGetNmon struct {
		node  string
		value chan<- cluster.NodeMonitor
	}
)

func (o opDelNmon) setError(err error) {
	o.err <- err
}

func (o opDelNmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelNmon
	if _, ok := d.pending.Monitor.Nodes[d.localNode]; ok {
		delete(d.pending.Monitor.Nodes, d.localNode)
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"monitor"},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	msgbus.PubNmonDelete(d.bus, msgbus.NmonDeleted{
		Node: d.localNode,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opGetNmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetNmon
	s := cluster.NodeMonitor{}
	if nodeStatus, ok := d.pending.Monitor.Nodes[o.node]; ok {
		s = nodeStatus.Monitor
	}
	select {
	case <-ctx.Done():
	case o.value <- s:
	}
}

func (o opSetNmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNmon
	newValue := d.pending.Monitor.Nodes[d.localNode]
	newValue.Monitor = o.value
	d.pending.Monitor.Nodes[d.localNode] = newValue
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"monitor"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	msgbus.PubNmonUpdated(d.bus, msgbus.NmonUpdated{
		Node:    d.localNode,
		Monitor: o.value,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
