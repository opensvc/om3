package daemondata

import (
	"context"

	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/jsondelta"
	"github.com/opensvc/om3/util/san"
)

type (
	opSetNodeOsPaths struct {
		errC
		value san.Paths
	}
)

// SetNodeOsPaths sets Monitor.Node.<localhost>.Status.Paths
func (t T) SetNodeOsPaths(paths san.Paths) error {
	err := make(chan error, 1)
	op := opSetNodeOsPaths{
		errC:  err,
		value: paths,
	}
	t.cmdC <- op
	return <-err
}

func (o opSetNodeOsPaths) call(ctx context.Context, d *data) error {
	d.statCount[idSetNodeOsPaths]++
	v := d.pending.Cluster.Node[d.localNode]
	v.Os.Paths = o.value
	d.pending.Cluster.Node[d.localNode] = v
	op := jsondelta.Operation{
		OpPath:  jsondelta.OperationPath{"os", "paths"},
		OpValue: jsondelta.NewOptValue(o.value),
		OpKind:  "replace",
	}
	d.pendingOps = append(d.pendingOps, op)
	d.bus.Pub(msgbus.NodeOsPathsUpdated{Node: d.localNode, Value: o.value}, d.labelLocalNode)
	return nil
}
