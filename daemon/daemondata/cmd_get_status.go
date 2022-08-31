package daemondata

import (
	"context"

	"opensvc.com/opensvc/core/cluster"
)

type opGetStatus struct {
	status chan<- *cluster.Status
}

func (o opGetStatus) call(ctx context.Context, d *data) {
	d.counterCmd <- idGetStatus
	select {
	case <-ctx.Done():
	case o.status <- d.pending.DeepCopy():
	}
}

func (t T) GetStatus() *cluster.Status {
	status := make(chan *cluster.Status)
	t.cmdC <- opGetStatus{
		status: status,
	}
	return <-status
}
