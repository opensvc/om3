package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
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
