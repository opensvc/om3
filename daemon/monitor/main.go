package monitor

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/daemondatactx"
	"opensvc.com/opensvc/daemon/enable"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
		ctx          context.Context
		cancel       context.CancelFunc
		log          zerolog.Logger
		loopC        chan action
		loopDelay    time.Duration
		loopEnabled  *enable.T
		routineTrace routineTracer
	}
	action struct {
		do   string
		done chan string
	}
	routineTracer interface {
		Trace(string) func()
		Stats() routinehelper.Stat
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{
		loopDelay:   1000 * time.Millisecond,
		loopEnabled: enable.New(),
	}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("monitor funcopt.Apply")
		return nil
	}
	t.ctx, t.cancel = context.WithCancel(t.ctx)
	t.T = subdaemon.New(
		subdaemon.WithName("monitor"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
		subdaemon.WithContext(t.ctx),
	)
	t.log = t.Log()
	t.loopC = make(chan action)
	return t
}

func (t *T) MainStart() error {
	t.log.Info().Msg("mgr starting")
	started := make(chan bool)
	go func() {
		defer t.Trace(t.Name() + "-loop")()
		defer t.cancel()
		t.loop(started)
	}()
	<-started
	t.log.Info().Msg("mgr started")
	return nil
}

func (t *T) MainStop() error {
	t.log.Info().Msg("mgr stopping")
	t.cancel()
	t.log.Info().Msg("mgr stopped")
	return nil
}

func (t *T) loop(c chan bool) {
	t.log.Info().Msg("loop started")
	c <- true
	t.aLoop()
	for {
		select {
		case <-t.ctx.Done():
			t.log.Info().Msg("loop stopped")
			return
		case <-time.After(t.loopDelay):
			t.aLoop()
		}
	}
}

func (t *T) aLoop() {
	t.log.Debug().Msg("loop")
	dataCmd := daemondatactx.DaemonData(t.ctx)
	dataCmd.CommitPending()
	if msg := dataCmd.GetHbMessage(); len(msg) > 0 {
		daemonctx.HBSendQ(t.ctx) <- msg
	}
}
