package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
)

type (
	opSetClusterConfig struct {
		err   chan<- error
		value cluster.Config
	}
)

// SetClusterConfig sets Monitor.Cluster.Config
func (t T) SetClusterConfig(value cluster.Config) error {
	err := make(chan error)
	op := opSetClusterConfig{
		err:   err,
		value: value,
	}
	t.cmdC <- op
	return <-err
}

func (o opSetClusterConfig) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetClusterConfig
	/*
		// TODO: do we need a Equal() ?
		if d.pending.Cluster.Config == o.value {
			o.err <- nil
			return
		}
	*/
	d.pending.Cluster.Config = o.value
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"cluster", "config"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(
		msgbus.ClusterConfigUpdated{
			Node:  d.localNode,
			Value: o.value,
		},
	)
	select {
	case <-ctx.Done():
	case o.err <- nil:
	}
}
