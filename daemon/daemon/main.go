/*
	Package daemon provide the subdaemon main responsible ot other opensvc daemons

    It is responsible for other sub daemons (monitor, ...)
*/
package daemon

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondata"
	"opensvc.com/opensvc/daemon/daemondiscover"
	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/enable"
	"opensvc.com/opensvc/daemon/hb"
	"opensvc.com/opensvc/daemon/listener"
	"opensvc.com/opensvc/daemon/monitor"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
	"opensvc.com/opensvc/util/pubsub"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
		ctx           context.Context
		cancel        context.CancelFunc
		log           zerolog.Logger
		loopC         chan action
		loopDelay     time.Duration
		loopEnabled   *enable.T
		mandatorySubs map[string]sub
		otherSubs     []string
		cancelFuncs   []context.CancelFunc
	}
	action struct {
		do   string
		done chan string
	}
	sub struct {
		new        func(t *T) subdaemon.Manager
		subActions subdaemon.Manager
	}
)

var (
	mandatorySubs = map[string]sub{
		"monitor": {
			new: func(t *T) subdaemon.Manager {
				return monitor.New(
					monitor.WithRoutineTracer(&t.TT),
					monitor.WithContext(t.ctx),
				)
			},
		},
		"listener": {
			new: func(t *T) subdaemon.Manager {
				return listener.New(
					listener.WithRoutineTracer(&t.TT),
					listener.WithContext(t.ctx),
				)
			},
		},
		"hb": {
			new: func(t *T) subdaemon.Manager {
				return hb.New(
					hb.WithRoutineTracer(&t.TT),
					hb.WithRootDaemon(t),
					hb.WithContext(t.ctx),
				)
			},
		},
	}
)

func New(opts ...funcopt.O) *T {
	ctx, cancel := context.WithCancel(context.Background())
	log := daemonlogctx.Logger(ctx).With().Str("name", "daemon-main").Logger()
	ctx = daemonlogctx.WithLogger(ctx, log)
	t := &T{
		loopDelay:   10 * time.Second,
		loopEnabled: enable.New(),
		log:         log,
		ctx:         ctx,
		cancel:      cancel,
	}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("daemon main funcopt.Apply")
		return nil
	}
	t.T = subdaemon.New(
		subdaemon.WithName("main"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
		subdaemon.WithContext(t.ctx),
	)
	t.cancelFuncs = make([]context.CancelFunc, 0)
	t.loopC = make(chan action)
	return t
}

// RunDaemon starts main daemon
func RunDaemon() (*T, error) {
	main := New(WithRoutineTracer(routinehelper.NewTracer()))

	if err := main.Init(); err != nil {
		main.log.Error().Err(err).Msg("daemon Init")
		return main, err
	}
	if err := main.Start(); err != nil {
		main.log.Error().Err(err).Msg("daemon Start")
		return main, err
	}
	return main, nil
}

// MainStart starts loop, mandatory subdaemons
func (t *T) MainStart() error {
	t.log.Info().Msg("mgr starting")
	started := make(chan bool)
	go func() {
		defer t.Trace(t.Name() + "-loop")()
		t.loop(started)
	}()

	t.ctx = daemonctx.WithDaemonPubSubCmd(t.ctx, pubsub.Start(t.ctx, "daemon pub sub"))

	t.ctx = daemonctx.WithDaemon(t.ctx, t)

	t.ctx = daemonctx.WithHBSendQ(t.ctx, make(chan []byte))
	dataCmd, cancel := daemondata.Start(t.ctx)
	t.cancelFuncs = append(t.cancelFuncs, cancel)
	t.ctx = daemonctx.WithDaemonDataCmd(t.ctx, dataCmd)

	<-started
	for subName, sub := range mandatorySubs {
		sub.subActions = sub.new(t)
		if err := sub.subActions.Init(); err != nil {
			t.log.Err(err).Msgf("%s Init", subName)
			return err
		}
		if err := t.Register(sub.subActions); err != nil {
			t.log.Err(err).Msgf("%s register", subName)
			return err
		}
		if err := sub.subActions.Start(); err != nil {
			t.log.Err(err).Msgf("%s start", subName)
			return err
		}
	}

	daemondiscover.Start(t.ctx)

	t.log.Info().Msg("mgr started")
	return nil
}

func (t *T) MainStop() error {
	t.log.Info().Msg("mgr stopping")
	if t.loopEnabled.Enabled() {
		done := make(chan string)
		t.loopC <- action{"stop", done}
		<-done
	}
	for _, cancel := range t.cancelFuncs {
		cancel()
	}
	t.cancel()
	t.log.Info().Msg("mgr stopped")
	return nil
}

func (t *T) loop(c chan bool) {
	t.log.Info().Msg("loop started")
	t.loopEnabled.Enable()
	t.aLoop()
	c <- true
	ticker := time.NewTicker(t.loopDelay)
	defer ticker.Stop()
	for {
		select {
		case a := <-t.loopC:
			t.loopEnabled.Disable()
			t.log.Info().Msg("loop stopped")
			a.done <- "loop stopped"
			return
		case <-ticker.C:
			t.aLoop()
		}
	}
}

func (t *T) aLoop() {
}
