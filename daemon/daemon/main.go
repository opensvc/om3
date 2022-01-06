/*
	Package daemon provide the subdaemon main responsible ot other opensvc daemons

    It is responsible for other sub daemons (monitor, ...)
*/
package daemon

import (
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/enable"
	"opensvc.com/opensvc/daemon/listener"
	"opensvc.com/opensvc/daemon/monitor"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		log           zerolog.Logger
		loopC         chan action
		loopDelay     time.Duration
		loopEnabled   *enable.T
		mandatorySubs map[string]sub
		otherSubs     []string
		routinehelper.TT
	}
	action struct {
		do   string
		done chan string
	}
	sub struct {
		new        func(t *T) subber
		subActions subber
	}
	subber interface {
		subdaemon.MainManager
		Init() error
		Start() error
	}
)

var (
	mandatorySubs = map[string]sub{
		"monitor": {
			new: func(t *T) subber {
				return monitor.New(monitor.WithRoutineTracer(&t.TT))
			},
		},
		"listener": {
			new: func(t *T) subber {
				return listener.New(listener.WithRoutineTracer(&t.TT))
			},
		},
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{
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
	t.log = t.Log()
	t.loopC = make(chan action)
	return t
}

// RunDaemon() starts main daemon
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

// StopDaemon() stop main daemon and wait
func (t *T) StopDaemon() error {
	if err := t.Stop(); err != nil {
		t.log.Error().Err(err).Msg("daemon Stop")
		return err
	}
	done := make(chan bool)
	go func() {
		t.WaitDone()
		done <- true
	}()
	if err := t.Quit(); err != nil {
		t.log.Error().Err(err).Msg("daemon Quit")
		return err
	}
	<-done
	return nil
}

// MainStart() starts loop, mandatory subdaemons
func (t *T) MainStart() error {
	t.log.Info().Msg("mgr starting")
	started := make(chan bool)
	go func() {
		defer t.Trace(t.Name() + "-loop")()
		t.loop(started)
	}()
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
