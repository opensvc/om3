package lsnrhttpinet

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"

	"opensvc.com/opensvc/daemon/daemonctx"
	"opensvc.com/opensvc/daemon/routinehelper"
	"opensvc.com/opensvc/daemon/subdaemon"
	"opensvc.com/opensvc/util/funcopt"
)

type (
	T struct {
		*subdaemon.T
		daemonctx.TCtx
		routinehelper.TT
		listener     *http.Server
		log          zerolog.Logger
		routineTrace routineTracer
		handler      http.Handler
		addr         string
		certFile     string
		keyFile      string
	}

	routineTracer interface {
		Trace(string) func()
		Stats() routinehelper.Stat
	}
)

func New(opts ...funcopt.O) *T {
	t := &T{TCtx: daemonctx.TCtx{}}
	t.SetTracer(routinehelper.NewTracerNoop())
	if err := funcopt.Apply(t, opts...); err != nil {
		t.log.Error().Err(err).Msg("listener funcopt.Apply")
		return nil
	}
	t.T = subdaemon.New(
		subdaemon.WithName("lsnr-http-inet"),
		subdaemon.WithMainManager(t),
		subdaemon.WithRoutineTracer(&t.TT),
	)
	t.log = t.Log().With().
		Str("addr", t.addr).
		Str("sub", t.Name()).
		Logger()
	return t
}

func (t *T) MainStart() error {
	t.log.Debug().Msg("mgr starting")
	started := make(chan bool)
	go func() {
		defer t.Trace(t.Name())()
		if err := t.start(); err != nil {
			t.log.Error().Err(err).Msg("mgr start failure")
		}
		started <- true
	}()
	<-started
	t.log.Debug().Msg("started")
	return nil
}

func (t *T) MainStop() error {
	t.log.Debug().Msg("mgr stopping")
	if err := t.stop(); err != nil {
		t.log.Error().Err(err).Msg("mgr stop failure")
	}
	t.log.Debug().Msg("mgr stopped")
	return nil
}

func (t *T) stop() error {
	if err := (*t.listener).Close(); err != nil {
		t.log.Error().Err(err).Msg("listener Close failure")
		return err
	}
	t.log.Info().Msg("listener Closed")
	return nil
}

func (t *T) start() error {
	t.log.Info().Msg("listener starting")
	started := make(chan bool)
	t.listener = &http.Server{Addr: t.addr, Handler: t.handler}
	go func() {
		started <- true
		err := t.listener.ListenAndServeTLS(t.certFile, t.keyFile)
		if err != http.ErrServerClosed && !strings.Contains(err.Error(), "use of closed network connection") {
			t.log.Debug().Err(err).Msg("listener ends with unexpected error ")
		}
		t.log.Info().Msg("listener stopped")
	}()
	<-started
	t.log.Info().Msg("listener started ")
	return nil
}
