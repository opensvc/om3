// ccfg is responsible of the cluster config
//
// It provides:
//
//	.cluster.config
package ccfg

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/node"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/xconfig"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/msgbus"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	ccfg struct {
		state         cluster.Config
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
		resp chan cluster.Config
	}
)

var (
	cmdC chan any
)

func init() {
	cmdC = make(chan any)
}

// Start launches the ccfg worker goroutine
func Start(parent context.Context) error {
	ctx, cancel := context.WithCancel(parent)

	o := &ccfg{
		ctx:       ctx,
		cancel:    cancel,
		cmdC:      make(chan any),
		databus:   daemondata.FromContext(ctx),
		bus:       pubsub.BusFromContext(ctx),
		log:       log.Logger.With().Str("func", "ccfg").Logger(),
		localhost: hostname.Hostname(),
	}

	if n, err := object.NewCluster(object.WithVolatile(true)); err != nil {
		return err
	} else {
		o.clusterConfig = n.Config()
	}

	o.startSubscriptions()
	go func() {
		defer func() {
			msgbus.DropPendingMsg(o.cmdC, time.Second)
			o.sub.Stop()
		}()
		o.worker()
	}()

	// start serving
	cmdC = o.cmdC

	return nil
}

func (o *ccfg) startSubscriptions() {
	sub := o.bus.Sub("ccfg")
	sub.AddFilter(msgbus.ConfigFileUpdated{}, pubsub.Label{"path", "cluster"})
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
			case msgbus.ConfigFileUpdated:
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
	c := cmdGet{
		resp: make(chan cluster.Config),
	}
	cmdC <- c
	return <-c.resp
}
