package daemondata

import (
	"opensvc.com/opensvc/core/instance"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/daemonps"
	"opensvc.com/opensvc/daemon/monitor/moncmd"
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

func (o opDelSmon) call(d *data) {
	d.counterCmd <- idDelSmon
	s := o.path.String()
	if _, ok := d.pending.Monitor.Nodes[d.localNode].Services.Smon[s]; ok {
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"services", "smon", s},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	daemonps.PubSmonDelete(d.pubSub, s, moncmd.SmonDeleted{
		Path: o.path,
		Node: d.localNode,
	})
	o.err <- nil
}

func (o opSetSmon) call(d *data) {
	d.counterCmd <- idSetSmon
	s := o.path.String()
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"services", "smon", s},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	daemonps.PubSmonUpdated(d.pubSub, s, moncmd.SmonUpdated{
		Path:   o.path,
		Node:   d.localNode,
		Status: o.value,
	})
	o.err <- nil
}
