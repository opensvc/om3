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
	opDelInstanceConfig struct {
		err  chan<- error
		path path.T
	}

	opSetInstanceConfig struct {
		err   chan<- error
		path  path.T
		value instance.Config
	}
)

func (o opSetInstanceConfig) setError(err error) {
	select {
	case o.err <- err:
	}
}

func (o opDelInstanceConfig) setError(err error) {
	select {
	case o.err <- err:
	}
}

func (o opDelInstanceConfig) call(ctx context.Context, d *data) {
	d.counterCmd <- idDelInstanceConfig
	s := o.path.String()
	if _, ok := d.pending.Monitor.Nodes[d.localNode].Services.Config[s]; ok {
		op := jsondelta.Operation{
			OpPath: jsondelta.OperationPath{"services", "config", s},
			OpKind: "remove",
		}
		d.pendingOps = append(d.pendingOps, op)
	}
	daemonps.PubCfgDelete(d.bus, s, moncmd.CfgDeleted{
		Path: o.path,
		Node: d.localNode,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}

func (o opSetInstanceConfig) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetInstanceConfig
	s := o.path.String()
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"services", "config", s},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	daemonps.PubCfgUpdate(d.bus, s, moncmd.CfgUpdated{
		Path:   o.path,
		Node:   d.localNode,
		Config: o.value,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
