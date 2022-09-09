/*
	Package daemon provide the subdaemon main responsible ot other opensvc daemons

    It is responsible for other sub daemons (monitor, ...)
*/
package daemon

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/discover"
	"opensvc.com/opensvc/daemon/enable"
	"opensvc.com/opensvc/daemon/hb"
	"opensvc.com/opensvc/daemon/listener"
	"opensvc.com/opensvc/daemon/monitor"
	"opensvc.com/opensvc/daemon/monitor/nmon"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/scheduler"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
		ctx         context.Context
		cancel      context.CancelFunc
		log         zerolog.Logger
		loopC       chan action
		loopDelay   time.Duration
		loopEnabled *enable.T
		cancelFuncs []context.CancelFunc
	}
	action struct {
		do   string
		done chan string
	}
)

var (
	mandatorySubs = []func(t *T) subdaemon.Manager{
		func(t *T) subdaemon.Manager {
			return monitor.New(
				monitor.WithRoutineTracer(&t.TT),
			)
		},
		func(t *T) subdaemon.Manager {
			return listener.New(
				listener.WithRoutineTracer(&t.TT),
			)
		},
		func(t *T) subdaemon.Manager {
			return hb.New(
				hb.WithRoutineTracer(&t.TT),
				hb.WithRootDaemon(t),
			)
		},
		func(t *T) subdaemon.Manager {
			return scheduler.New(
				scheduler.WithRoutineTracer(&t.TT),
			)
		},
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{
		loopDelay:   10 * time.Second,
		loopEnabled: enable.New(),
		log:         log.Logger,
	}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		return nil
	}
	t.T = subdaemon.New(
		subdaemon.WithName("root"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
	)
	t.cancelFuncs = make([]context.CancelFunc, 0)
	t.loopC = make(chan action)
	return t
}

// RunDaemon starts main daemon
func RunDaemon(opts ...funcopt.O) (*T, error) {
	main := New(opts...)
	ctx := context.Background()
	if err := main.Start(ctx); err != nil {
		main.log.Error().Err(err).Msg("daemon Start")
		return main, err
	}
	return main, nil
}

// MainStart starts loop, mandatory subdaemons
func (t *T) MainStart(ctx context.Context) error {
	t.ctx = ctx
	started := make(chan bool)
	t.Add(1)
	go func() {
		defer t.Trace(t.Name() + "-loop")()
		defer t.Done()
		started <- true
		t.loop()
	}()

	bus := pubsub.NewBus("daemon")
	bus.Start(t.ctx)
	t.ctx = pubsub.ContextWithBus(t.ctx, bus)

	t.ctx = daemonctx.WithDaemon(t.ctx, t)
	t.ctx = daemonctx.WithHBSendQ(t.ctx, make(chan []byte))

	dataCmd, dataCmdCancel := daemondata.Start(t.ctx)
	t.ctx = daemondata.ContextWithBus(t.ctx, dataCmd)

	defer func() {
		t.cancelFuncs = append(t.cancelFuncs, func() {
			t.log.Debug().Msg("stop daemon data")
			dataCmdCancel()
		})
	}()
	defer func() {
		t.cancelFuncs = append(t.cancelFuncs, func() {
			t.log.Debug().Msg("stop daemon pubsub bus")
			bus.Stop()
		})
	}()

	<-started

	for _, newSub := range mandatorySubs {
		sub := newSub(t)
		if err := t.Register(sub); err != nil {
			return err
		}
		if err := sub.Start(t.ctx); err != nil {
			return err
		}
	}
	if err := nmon.Start(t.ctx); err != nil {
		return err
	}
	cancelDiscover, err := discover.Start(t.ctx)
	if err != nil {
		return err
	}
	t.cancelFuncs = append(t.cancelFuncs, func() {
		t.log.Debug().Msg("stop daemon discover")
		cancelDiscover()
		t.log.Debug().Msg("stopped daemon discover")
	})
	return nil
}

func (t *T) MainStop() error {
	// stop goroutines without cancel context
	for _, cancel := range t.cancelFuncs {
		cancel()
	}

	// goroutines started by MainStart are stopped by the context cancel
	return nil
}

func (t *T) loop() {
	t.log.Info().Msg("loop started")
	t.loopEnabled.Enable()
	t.aLoop()
	ticker := time.NewTicker(t.loopDelay)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.aLoop()
		case <-t.ctx.Done():
			return
		}
	}
}

func (t *T) aLoop() {
}
