package daemondata

import (
	"context"
	"encoding/json"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/pubsub"
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

// ClusterDataAfterPublished returns a deep copy of the cluster data once the
// manager processed every message published before this call, and false
// when the manager did not answer within timeout.
//
// ClusterData answers from the commands channel, which the manager serves
// apart from its subscription: a message published, but still queued on the
// subscription, is not in the copy. A caller subscribing to the bus, then
// replaying the cluster data before forwarding what its subscription
// receives, would never see such a message. The request goes through the
// bus instead, queued after the messages published before it.
//
// The bus returns from a publication once it queued it on every matching
// subscription and waits while one of them is full: a manager stalled on
// its subscription would hold the request, and the caller, past timeout.
// The request is published apart, the timeout running from the call; its
// reply channel is buffered, the manager never waits on a caller that gave
// up.
func (t T) ClusterDataAfterPublished(publisher pubsub.Publisher, timeout time.Duration) (*clusterdump.Data, bool) {
	req := &msgbus.ClusterDataSnapshotRequest{ReplyC: make(chan *clusterdump.Data, 1)}
	go publisher.Pub(req)
	select {
	case data := <-req.ReplyC:
		return data, true
	case <-time.After(timeout):
		return nil, false
	}
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
