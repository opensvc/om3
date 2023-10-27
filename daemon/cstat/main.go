// Package cstat is responsible for the cluster status
//
// It provides:
//
//	.cluster.status
package cstat

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	cstat struct {
		state cluster.Status

		ctx       context.Context
		cancel    context.CancelFunc
		cmdC      chan any
		bus       *pubsub.Bus
		log       *plog.Logger
		startedAt time.Time

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		nodeStatus map[string]node.Status

		cancelReady context.CancelFunc
		change      bool

		sub *pubsub.Subscription
		wg  sync.WaitGroup
	}
)

func New() *cstat {
	return &cstat{nodeStatus: make(map[string]node.Status)}
}

// Start launches the cstat worker goroutine
func (o *cstat) Start(parent context.Context) error {
	o.log = plog.NewDefaultLogger().WithPrefix("daemon: cstat: ").Attr("pkg", "daemon/cstat")
	o.ctx, o.cancel = context.WithCancel(parent)
	o.bus = pubsub.BusFromContext(o.ctx)

	o.startSubscriptions()
	running := make(chan bool)
	o.wg.Add(1)
	go func() {
		o.log.Debugf("start")
		running <- true
		defer o.log.Debugf("done")
		defer o.wg.Done()
		defer func() {
			if err := o.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				o.log.Errorf("subscription stop error %s", err)
			}
		}()
		o.worker()
	}()
	<-running
	return nil
}

func (o *cstat) Stop() error {
	o.cancel()
	o.wg.Wait()
	return nil
}

func (o *cstat) startSubscriptions() {
	sub := o.bus.Sub("cstat")
	sub.AddFilter(&msgbus.NodeStatusUpdated{})
	sub.Start()
	o.sub = sub
}

// worker watch for local cstat updates
func (o *cstat) worker() {
	o.startedAt = time.Now()

	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case *msgbus.NodeStatusUpdated:
				o.onNodeStatusUpdated(c)
			}
		}
	}
}
