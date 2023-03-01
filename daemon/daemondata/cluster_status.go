package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/msgbus"
)

type (
	opSetClusterStatus struct {
		errC
		value cluster.Status
	}
)

// GetStatus returns deep copy of status
func (t T) GetStatus() *cluster.Data {
	status := make(chan *cluster.Data, 1)
	err := make(chan error, 1)
	t.cmdC <- opGetStatus{
		errC:   err,
		status: status,
	}
	if <-err != nil {
		return nil
	}
	return <-status
}

type opGetStatus struct {
	errC
	status chan<- *cluster.Data
}

func (o opGetStatus) call(ctx context.Context, d *data) error {
	d.statCount[idGetStatus]++
	o.status <- d.pending.DeepCopy()
	return nil
}

// SetClusterStatus
//
// cluster.status
func (t T) SetClusterStatus(v cluster.Status) error {
	err := make(chan error, 1)
	op := opSetClusterStatus{
		errC:  err,
		value: v,
	}
	t.cmdC <- op
	return <-err
}

func (o opSetClusterStatus) call(ctx context.Context, d *data) error {
	d.statCount[idSetClusterStatus]++
	d.pending.Cluster.Status = o.value
	d.bus.Pub(
		msgbus.ClusterStatusUpdated{
			Node:  d.localNode,
			Value: o.value,
		},
		labelLocalNode,
	)
	return nil
}
