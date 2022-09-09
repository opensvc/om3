package daemondata

import (
	"context"
	"time"

	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opSetNodeFrozen struct {
		err   chan<- error
		value time.Time
	}
)

func (o opSetNodeFrozen) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNmon
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"frozen"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	msgbus.PubFrozen(d.bus, hostname.Hostname(), msgbus.Frozen{
		Node:  hostname.Hostname(),
		Path:  path.T{},
		Value: o.value,
	})
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
