package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/node"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opSetNodeConfig struct {
		err   chan<- error
		value node.Config
	}
)

// SetNodeConfig sets Monitor.Node.<localhost>.Config
func (t T) SetNodeConfig(value node.Config) error {
	err := make(chan error)
	op := opSetNodeConfig{
		err:   err,
		value: value,
	}
	t.cmdC <- op
	return <-err
}

func (o opSetNodeConfig) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNodeConfig
	v := d.pending.Cluster.Node[d.localNode]
	if v.Config == o.value {
		o.err <- nil
		return
	}
	v.Config = o.value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"config"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(
		msgbus.NodeConfigUpdated{
			Node:  d.localNode,
			Value: o.value,
		},
		labelLocalNode,
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
