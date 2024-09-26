package daemondata

import (
	"context"

	"github.com/opensvc/om3/core/clusterdump"
)

// ClusterData returns deep copy of status
func (t T) ClusterData() *clusterdump.Data {
	status := make(chan *clusterdump.Data, 1)
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
	status chan<- *clusterdump.Data
}

func (o opGetClusterData) call(ctx context.Context, d *data) error {
	o.status <- d.clusterData.DeepCopy()
	return nil
}
