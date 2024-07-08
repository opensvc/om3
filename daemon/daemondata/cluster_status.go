package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/cluster"
)

// ClusterData returns deep copy of status
func (t T) ClusterData() *cluster.Data {
	status := make(chan *cluster.Data, 1)
	err := make(chan error, 1)
	t.cmdC <- opGetClusterData{
		errC:   err,
		status: status,
	}
	if <-err != nil {
		return nil
	}
	return <-status
}

type opGetClusterData struct {
	errC
	status chan<- *cluster.Data
}

func (o opGetClusterData) call(ctx context.Context, d *data) error {
	o.status <- d.clusterData.DeepCopy()
	return nil
}
