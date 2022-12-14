package daemondata

import (
	"context"

	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/jsondelta"
	"opensvc.com/opensvc/util/san"
)

type (
	opSetNodeOsPaths struct {
		err   chan<- error
		value san.Paths
	}
)

// SetNodeOsPaths sets Monitor.Node.<localhost>.Status.Paths
func SetNodeOsPaths(c chan<- interface{}, paths san.Paths) error {
	err := make(chan error)
	op := opSetNodeOsPaths{
		err:   err,
		value: paths,
	}
	c <- op
	return <-err
}

func (o opSetNodeOsPaths) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetNodeOsPaths
	v := d.pending.Cluster.Node[d.localNode]
	v.Os.Paths = o.value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"os", "paths"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(
		msgbus.NodeOsPathsUpdated{
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
