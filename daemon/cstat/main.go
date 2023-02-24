// cstat is responsible of the cluster status
//
// It provides:
//
//	.cluster.status
package cstat

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	cstat struct {
		state cluster.Status

		ctx       context.Context
		cancel    context.CancelFunc
		cmdC      chan any
		databus   *daemondata.T
		bus       *pubsub.Bus
		log       zerolog.Logger
		startedAt time.Time

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		nodeStatus map[string]node.Status

		cancelReady context.CancelFunc
		change      bool

		sub *pubsub.Subscription
	}
)

// Start launches the cstat worker goroutine
func Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)

	o := &cstat{
		ctx:        ctx,
		cancel:     cancel,
		databus:    daemondata.FromContext(ctx),
		bus:        pubsub.BusFromContext(ctx),
		log:        log.Logger.With().Str("func", "cstat").Logger(),
		nodeStatus: make(map[string]node.Status),
	}

	o.startSubscriptions()
	go func() {
		defer func() {
			if err := o.sub.Stop(); err != nil {
				o.log.Error().Err(err).Msg("subscription stop")
			}
		}()
		o.worker()
	}()
	return nil
}

func (o *cstat) startSubscriptions() {
	sub := o.bus.Sub("cstat")
	sub.AddFilter(msgbus.NodeStatusUpdated{})
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
			case msgbus.NodeStatusUpdated:
				o.onNodeStatusUpdated(c)
			}
		}
	}
}
