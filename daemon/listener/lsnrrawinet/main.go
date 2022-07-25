package lsnrrawinet

import (
	"context"
	"net"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"opensvc.com/opensvc/daemon/listener/routeraw"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		routinehelper.TT
		listener     *net.Listener
		log          zerolog.Logger
		routineTrace routineTracer
		addr         string
	}

	routineTracer interface {
		Trace(string) func()
		Stats() routinehelper.Stat
	}

	rawServer interface {
		Serve(routeraw.ReadWriteCloseSetDeadliner)
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("listener funcopt.Apply")
		return nil
	}
	name := "lsnr-raw-inet"
	t.log = log.Logger.With().
		Str("addr", t.addr).
		Str("sub", name).
		Logger()
	t.T = subdaemon.New(
		subdaemon.WithName(name),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
	)
	return t
}

func (t *T) MainStart(ctx context.Context) error {
	started := make(chan bool)
	go func() {
		defer t.Trace(t.Name())()
		if err := t.start(ctx); err != nil {
			t.log.Error().Err(err).Msgf("mgr start failure")
		}
		started <- true
	}()
	<-started
	return nil
}

func (t *T) MainStop() error {
	if err := t.stop(); err != nil {
		t.log.Error().Err(err).Msg("mgr stop failure")
	}
	return nil
}
