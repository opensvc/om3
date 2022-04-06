package daemondata

import "opensvc.com/opensvc/core/cluster"

type opGetStatus struct {
	status chan<- *cluster.Status
}

func (o opGetStatus) call(d *data) {
	d.counterCmd <- idGetStatus
	o.status <- d.committed.DeepCopy()
}

func (t T) GetStatus() *cluster.Status {
	status := make(chan *cluster.Status)
	t.cmdC <- opGetStatus{
		status: status,
	}
	return <-status
}
