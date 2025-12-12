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

	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/node"
	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
)

type (
	T struct {
		state clusterdump.Status

		ctx       context.Context
		cancel    context.CancelFunc
		cmdC      chan any
		publisher pubsub.Publisher
		log       *plog.Logger
		startedAt time.Time

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		nodeStatus map[string]node.Status

		cancelReady context.CancelFunc
		change      bool

		sub   *pubsub.Subscription
		subQS pubsub.QueueSizer

		wg sync.WaitGroup
	}
)

func New(subQS pubsub.QueueSizer) *T {
	return &T{
		nodeStatus: make(map[string]node.Status),
		subQS:      subQS,
	}
}

// Start launches the cstat worker goroutine
func (o *T) Start(parent context.Context) error {
	o.log = plog.NewDefaultLogger().WithPrefix("daemon: cstat: ").Attr("pkg", "daemon/cstat")
	o.log.Tracef("starting")
	defer o.log.Tracef("started")
	o.ctx, o.cancel = context.WithCancel(parent)
	o.publisher = pubsub.PubFromContext(o.ctx)

	o.startSubscriptions()
	running := make(chan bool)
	o.wg.Add(1)
	go func() {
		o.log.Tracef("start")
		running <- true
		defer o.log.Tracef("done")
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

func (o *T) Stop() error {
	o.cancel()
	o.wg.Wait()
	return nil
}

func (o *T) startSubscriptions() {
	sub := pubsub.SubFromContext(o.ctx, "daemon.cstat", o.subQS)
	sub.AddFilter(&msgbus.NodeStatusUpdated{})
	sub.Start()
	o.sub = sub
}

// worker watch for local cstat updates
func (o *T) worker() {
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
