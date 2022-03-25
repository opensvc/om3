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
	"opensvc.com/opensvc/daemon/enable"
	"opensvc.com/opensvc/daemon/hb"
	"opensvc.com/opensvc/daemon/listener"
	"opensvc.com/opensvc/daemon/monitor"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/eventbus"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		daemonctx.TCtx
		log           zerolog.Logger
		loopC         chan action
		loopDelay     time.Duration
		loopEnabled   *enable.T
		mandatorySubs map[string]sub
		otherSubs     []string
		cancelFuncs   []context.CancelFunc
		routinehelper.TT
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
					monitor.WithContext(t.Ctx),
				)
			},
		},
		"listener": {
			new: func(t *T) subdaemon.Manager {
				return listener.New(
					listener.WithRoutineTracer(&t.TT),
					listener.WithContext(t.Ctx),
				)
			},
		},
		"hb": {
			new: func(t *T) subdaemon.Manager {
				return hb.New(
					hb.WithRoutineTracer(&t.TT),
					hb.WithRootDaemon(t),
					hb.WithContext(t.Ctx),
				)
			},
		},
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{
		TCtx:        daemonctx.TCtx{},
		loopDelay:   1 * time.Second,
		loopEnabled: enable.New(),
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
	)
	t.cancelFuncs = make([]context.CancelFunc, 0)
	t.log = t.Log()
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
	t.Ctx, t.CancelFunc = context.WithCancel(context.Background())
	t.Ctx = daemonctx.WithDaemon(t.Ctx, t)
	evBus := eventbus.T{}
	evBusCmdC, err := evBus.Run(t.Ctx, "daemon event bus")
	if err != nil {
		t.log.Err(err).Msg("event bus start")
		return err
	}
	t.Ctx = daemonctx.WithEventBusCmd(t.Ctx, evBusCmdC)

	dataCmd, cancel := daemondata.Start(t.Ctx)
	t.cancelFuncs = append(t.cancelFuncs, cancel)
	t.Ctx = daemonctx.WithDaemonDataCmd(t.Ctx, dataCmd)

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
	t.CancelFunc()
	t.log.Info().Msg("mgr stopped")
	return nil
}

func (t *T) loop(c chan bool) {
	t.log.Info().Msg("loop started")
	t.loopEnabled.Enable()
	t.aLoop()
	c <- true
	for {
		select {
		case a := <-t.loopC:
			t.loopEnabled.Disable()
			t.log.Info().Msg("loop stopped")
			a.done <- "loop stopped"
			return
		case <-time.After(t.loopDelay):
			t.aLoop()
		}
	}
}

func (t *T) aLoop() {
	t.log.Debug().Msg("loop")
}
