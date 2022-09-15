package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
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

func (o opDelSmon) setError(err error) {
	o.err <- err
}

func (o opDelSmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelSmon
	s := o.path.String()
	if _, ok := d.pending.Cluster.Node[d.localNode].Services.Smon[s]; ok {
		delete(d.pending.Cluster.Node[d.localNode].Services.Smon, s)
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"services", "smon", s},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	msgbus.PubSmonDelete(d.bus, s, msgbus.SmonDeleted{
		Path: o.path,
		Node: d.localNode,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opSetSmon) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetSmon
	s := o.path.String()
	d.pending.Cluster.Node[d.localNode].Services.Smon[s] = o.value
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"services", "smon", s},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	msgbus.PubSmonUpdated(d.bus, s, msgbus.SmonUpdated{
		Path:   o.path,
		Node:   d.localNode,
		Status: o.value,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
