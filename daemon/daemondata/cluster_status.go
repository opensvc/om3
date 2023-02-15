package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/msgbus"
)

type (
	opSetClusterStatus struct {
		err   chan<- error
		value cluster.Status
	}
)

// GetStatus returns deep copy of status
func (t T) GetStatus() *cluster.Data {
	status := make(chan *cluster.Data)
	t.cmdC <- opGetStatus{
		status: status,
	}
	return <-status
}

type opGetStatus struct {
	status chan<- *cluster.Data
}

func (o opGetStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetStatus
	o.status <- d.pending.DeepCopy()
}

// SetClusterStatus
//
// cluster.status
func (t T) SetClusterStatus(v cluster.Status) error {
	err := make(chan error)
	op := opSetClusterStatus{
		err:   err,
		value: v,
	}
	t.cmdC <- op
	return <-err
}

func (o opSetClusterStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idSetClusterStatus
	d.pending.Cluster.Status = o.value
	d.bus.Pub(
		msgbus.ClusterStatusUpdated{
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
