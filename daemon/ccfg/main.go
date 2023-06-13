// Package ccfg is responsible for the cluster config
//
// It subscribes on msgbus.ConfigFileUpdated for cluster to provide:
//
//	cluster configuration reload:
//	  => cluster.ConfigData update => .cluster.config
//	  => clusternode update (for node selector, clusternodes dereference)
//	  => publication of msgbus.ClusterConfigUpdated for local node
package ccfg

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/daemon/daemondata"
	"github.com/opensvc/om3/daemon/draincommand"
	"github.com/opensvc/om3/daemon/msgbus"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/pubsub"
)

type (
	ccfg struct {
		state       cluster.Config
		networkSigs map[string]string

		clusterConfig *xconfig.T
		ctx           context.Context
		cancel        context.CancelFunc
		cmdC          chan any
		databus       *daemondata.T
		bus           *pubsub.Bus
		log           zerolog.Logger
		startedAt     time.Time

		pendingCtx    context.Context
		pendingCancel context.CancelFunc

		scopeNodes  []string
		nodeMonitor map[string]node.Monitor

		cancelReady context.CancelFunc
		localhost   string
		change      bool

		sub *pubsub.Subscription
	}

	cmdGet struct {
		draincommand.ErrC
		resp chan cluster.Config
	}
)

var (
	cmdC chan any
)

// Start launches the ccfg worker goroutine
func Start(parent context.Context, drainDuration time.Duration) error {
	ctx, cancel := context.WithCancel(parent)

	o := &ccfg{
		networkSigs: make(map[string]string),
		ctx:         ctx,
		cancel:      cancel,
		cmdC:        make(chan any),
		databus:     daemondata.FromContext(ctx),
		bus:         pubsub.BusFromContext(ctx),
		log:         log.Logger.With().Str("func", "ccfg").Logger(),
		localhost:   hostname.Hostname(),
	}
	cmdC = o.cmdC

	if n, err := object.NewCluster(object.WithVolatile(true)); err != nil {
		return err
	} else {
		o.clusterConfig = n.Config()
	}

	o.pubClusterConfig()

	o.startSubscriptions()
	go func() {
		defer func() {
			draincommand.Do(o.cmdC, drainDuration)
			if err := o.sub.Stop(); err != nil && !errors.Is(err, context.Canceled) {
				o.log.Warn().Err(err).Msg("subscription stop")
			}
		}()
		o.worker()
	}()

	// start serving
	cmdC = o.cmdC

	return nil
}

func (o *ccfg) startSubscriptions() {
	sub := o.bus.Sub("ccfg")
	sub.AddFilter(&msgbus.ConfigFileUpdated{}, pubsub.Label{"path", "cluster"})
	sub.Start()
	o.sub = sub
}

// worker watch for local ccfg updates
func (o *ccfg) worker() {
	defer o.log.Debug().Msg("done")

	o.startedAt = time.Now()

	for {
		select {
		case <-o.ctx.Done():
			return
		case i := <-o.sub.C:
			switch c := i.(type) {
			case *msgbus.ConfigFileUpdated:
				o.onConfigFileUpdated(c)
			}
		case i := <-o.cmdC:
			switch c := i.(type) {
			case cmdGet:
				o.onCmdGet(c)
			}
		}
	}
}

func Get() cluster.Config {
	err := make(chan error, 1)
	c := cmdGet{
		ErrC: err,
		resp: make(chan cluster.Config),
	}
	cmdC <- c
	if <-err != nil {
		return cluster.Config{}
	}
	return <-c.resp
}
