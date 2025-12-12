package daemondata

import (
	"context"

	"golang.org/x/sync/singleflight"

	"github.com/opensvc/om3/v3/core/clusterdump"
)

var (
	singleFlightGrp singleflight.Group
)

// ClusterData returns deep copy of status
func (t T) ClusterData() *clusterdump.Data {
	i, err, _ := singleFlightGrp.Do("clusterData", func() (interface{}, error) {
		return t.clusterData(), nil
	})
	if err != nil {
		return nil
	}
	return i.(*clusterdump.Data)
}

func (t T) clusterData() *clusterdump.Data {
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
