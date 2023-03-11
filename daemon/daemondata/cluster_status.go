package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/daemon/msgbus"
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

// onClusterStatusUpdated updates .cluster.status
func (d *data) onClusterStatusUpdated(m msgbus.ClusterStatusUpdated) {
	d.statCount[idSetClusterStatus]++
	d.pending.Cluster.Status = m.Value
}
