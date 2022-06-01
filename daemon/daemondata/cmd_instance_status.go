package daemondata

import (
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

func (o opDelInstanceStatus) call(d *data) {
	d.counterCmd <- idDelInstanceStatus
	s := o.path.String()
	if _, ok := d.pending.Monitor.Nodes[d.localNode].Services.Status[s]; ok {
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"services", "status", s},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	daemonps.PubInstStatusDelete(d.pubSub, s, moncmd.InstStatusDeleted{
		Path: o.path,
		Node: d.localNode,
	})
	o.err <- nil
}

func (o opGetInstanceStatus) call(d *data) {
	d.counterCmd <- idGetInstanceStatus
	if nodeStatus, ok := d.pending.Monitor.Nodes[o.node]; ok {
		if instStatus, ok := nodeStatus.Services.Status[o.path.String()]; ok {
			o.status <- instStatus
			return
		}
	}
	o.status <- instance.Status{}
}

func (o opSetInstanceStatus) call(d *data) {
	d.counterCmd <- idSetInstanceStatus
	s := o.path.String()
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"services", "status", s},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	daemonps.PubInstStatusUpdated(d.pubSub, s, moncmd.InstStatusUpdated{
		Path:   o.path,
		Node:   d.localNode,
		Status: o.value,
	})
	o.err <- nil
}
