package lsnrrawux

import (
	"context"
	"net"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/daemonlogctx"
	"opensvc.com/opensvc/daemon/listener/routeraw"
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
	name := "lsnr-raw-ux"
	t.log = daemonlogctx.Logger(t.ctx).
		With().
		Str("addr", t.addr).
		Str("sub", name).
		Logger()
	t.ctx = daemonlogctx.WithLogger(t.ctx, t.log)
	t.T = subdaemon.New(
		subdaemon.WithName(name),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
		subdaemon.WithContext(t.ctx),
	)
	return t
}

func (t *T) MainStart() error {
	t.log.Debug().Msg("mgr starting")
	started := make(chan bool)
	go func() {
		defer t.Trace(t.Name() + "-lsnr-raw")()
		if err := t.start(); err != nil {
			t.log.Error().Err(err).Msgf("starting raw listener")
		}
		started <- true
	}()
	<-started
	t.log.Debug().Msg(" started")
	return nil
}

func (t *T) MainStop() error {
	t.log.Debug().Msg("mgr stopping")
	if err := t.stop(); err != nil {
		t.log.Error().Err(err).Msg("stop")
	}
	t.log.Debug().Msg("mgr stopped")
	return nil
}
