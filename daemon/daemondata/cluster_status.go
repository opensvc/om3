package daemondata

import (
	"context"
	"encoding/json"

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

// ClusterDataJSON returns the cluster dataset already marshalled.
//
// The callers that put the dataset on the wire, the collector feed and
// GET /daemon/status, were each paying three serializations of it: the
// marshal and the unmarshal of the deep copy, and then their own marshal
// of what came back. Marshalling on the daemondata goroutine, where the
// dataset cannot change under it, is one, and it needs no copy at all:
// what comes back is bytes, which no caller can modify and every caller
// can share.
//
// It stays behind the same singleflight. Sharing was what made sharing
// the struct wrong; sharing bytes is only ever right.
func (t T) ClusterDataJSON() ([]byte, error) {
	i, err, _ := singleFlightGrp.Do("clusterDataJSON", func() (interface{}, error) {
		return t.clusterDataJSON()
	})
	if err != nil {
		return nil, err
	}
	return i.([]byte), nil
}

func (t T) clusterDataJSON() ([]byte, error) {
	b := make(chan []byte, 1)
	err := make(chan error, 1)
	t.cmdC <- opGetClusterDataJSON{
		errC: err,
		b:    b,
	}
	if e := <-err; e != nil {
		return nil, e
	}
	return <-b, nil
}

type opGetClusterDataJSON struct {
	errC
	b chan<- []byte
}

func (o opGetClusterDataJSON) call(ctx context.Context, d *data) error {
	b, err := json.Marshal(d.clusterData)
	if err != nil {
		return err
	}
	o.b <- b
	return nil
}
