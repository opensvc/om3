/*
	Package daemondata implements daemon journaled data

	import "opensvc.com/opensvc/daemon/daemondata"
	cmdC, cancel := daemondata.Start(context.Background())
	defer cancel()
	dataBus := daemondata.New(cmdC)

	status := dataBus.GetStatus() // retrieve daemon data
	bus.ApplyFull("remote-1", remoteNodeStatus)
	bus.ApplyPatch("remote-1", patchMsg)
	bus.CommitPending(context.Background())
	status = bus.GetStatus()
	localNodeStatus := bus.GetLocalNodeStatus()
*/
package daemondata

import (
	"context"
	"sync"
)

type (
	// T struct holds a daemondata manager cmdC to submit orders
	T struct {
		cmdC   chan<- interface{}
		cancel func()
	}
)

// Start runs the daemon journaled data manager
//
// It returns a cmdC chan to submit actions on cluster data
func Start(parent context.Context) (chan<- interface{}, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	cmdC := make(chan interface{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		run(ctx, cmdC)
	}()
	return cmdC, func() {
		cancel()
		wg.Wait()
	}
}

// New returns a new *T from an existing daemondata manager
func New(cmd chan<- interface{}) *T {
	return &T{cmdC: cmd}
}
