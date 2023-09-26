// cstat is responsible of the cluster status
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

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	cstat struct {
		state cluster.Status

		ctx       context.Context
		cancel    context.CancelFunc
		cmdC      chan any
		bus       *pubsub.Bus
		log       zerolog.Logger
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
	return &cstat{
		log:        log.Logger.With().Str("func", "cstat").Logger(),
		nodeStatus: make(map[string]node.Status),
	}
}

// Start launches the cstat worker goroutine
func (o *cstat) Start(parent context.Context) error {
	o.ctx, o.cancel = context.WithCancel(parent)
	o.bus = pubsub.BusFromContext(o.ctx)

	o.startSubscriptions()
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		defer func() {
			if err := o.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				o.log.Error().Err(err).Msg("subscription stop")
			}
		}()
		o.worker()
	}()
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
	defer o.log.Debug().Msg("done")

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
