/*
Package daemondata implements daemon journaled data

import "opensvc.com/opensvc/daemon/daemondata"
cmdC, cancel := daemondata.Start(context.Background())
defer cancel()
dataBus := daemondata.New(cmdC)

status := dataBus.ClusterData() // retrieve daemon data
bus.ApplyFull("remote-1", remoteNodeStatus)
bus.ApplyPatch("remote-1", patchMsg)
bus.CommitPending(context.Background())
status = bus.ClusterData()
localNodeStatus := bus.GetLocalNodeStatus()
*/
package daemondata

import (
	"context"
	"sync"
	"time"

	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	// T struct holds a daemondata manager cmdC to submit orders
	T struct {
		cmdC   chan<- Caller
		cancel func()
	}
)

// Start runs the daemon journaled data manager
//
// It returns a cmdC chan to submit actions on cluster data
func Start(parent context.Context, drainDuration time.Duration, subQS pubsub.QueueSizer) (chan<- Caller, chan<- *hbtype.Msg, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	cmdC := make(chan Caller)
	hbRecvMsgQ := make(chan *hbtype.Msg)
	var wg sync.WaitGroup
	d := newData()
	d.pub = pubsub.PubFromContext(ctx)
	d.startSubscriptions(ctx, subQS)
	wg.Add(1)
	go func() {
		defer wg.Done()
		d.run(ctx, cmdC, hbRecvMsgQ, drainDuration)
	}()
	return cmdC, hbRecvMsgQ, func() {
		cancel()
		wg.Wait()
	}
}

// New returns a new *T from an existing daemondata manager
func New(cmd chan<- Caller) *T {
	return &T{cmdC: cmd}
}
